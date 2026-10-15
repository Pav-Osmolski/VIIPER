package dualshock4

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"log/slog"
	"net"
	"testing"
	"time"

	srvusb "github.com/Alia5/VIIPER/internal/server/usb"
	"github.com/Alia5/VIIPER/usb"
	"github.com/Alia5/VIIPER/usbip"
	"github.com/Alia5/VIIPER/virtualbus"
)

var _ usb.OutputCommandAdmissionDevice = (*DualShock4)(nil)

func TestFramedDS4PendingRepeatsAllocateNothingAndPreserveABAStop(t *testing.T) {
	writer := newDualShock4OutputWriter(nil, StreamFrameVersionV3)
	a := []byte{1, 2, 3, 4, 5, 0, 0}
	b := []byte{6, 7, 8, 9, 10, 0, 0}
	zero := []byte{0, 0, 11, 12, 13, 0, 0}
	accepted := true
	allocations := testing.AllocsPerRun(10_000, func() {
		accepted = writer.EnqueueControl(StreamFrameOutputState, a) && accepted
	})
	if !accepted || allocations != 0 || writer.controlCount != 1 {
		t.Fatalf("repeat admission: accepted=%t allocations=%v pending=%d", accepted, allocations, writer.controlCount)
	}
	for _, payload := range [][]byte{b, a, zero} {
		if !writer.EnqueueControl(StreamFrameOutputState, payload) {
			t.Fatal("distinct A/B/A/zero rejected before capacity")
		}
	}
	for _, payload := range [][]byte{a, b, a, zero} {
		requireAuditControl(t, writer, payload)
	}
	requireAuditEmpty(t, writer)
}

// Safety regression replacing the prior characterization of a silently lost
// final zero. Distinct overload is explicit, and accepted history is intact.
func TestFramedDS4FullDistinctQueueReservesExactZeroAndLEDWithoutEviction(t *testing.T) {
	writer := newDualShock4OutputWriter(nil, StreamFrameVersionV3)
	for index := 1; index < dualShock4ControlCapacity; index++ {
		if !writer.EnqueueControl(StreamFrameOutputState, []byte{byte(index), 80, 32, 0, 0, 0, 0}) {
			t.Fatalf("ordinary state %d rejected early", index)
		}
	}
	if writer.EnqueueControl(StreamFrameOutputState, []byte{99, 80, 32, 0, 0, 0, 0}) {
		t.Fatal("ordinary overload falsely accepted")
	}
	zero := []byte{0, 0, 42, 43, 44, 0, 0}
	if !writer.EnqueueControl(StreamFrameOutputState, zero) || writer.controlCount != 32 {
		t.Fatal("reserved neutral capacity did not preserve exact zero and LED changes")
	}
	if !writer.EnqueueControl(StreamFrameOutputState, zero) || writer.controlCount != 32 {
		t.Fatal("exact pending neutral repeat rejected or used another slot at capacity")
	}
	newZero := []byte{0, 0, 45, 46, 47, 0, 0}
	if writer.EnqueueControl(StreamFrameOutputState, newZero) {
		t.Fatal("full distinct neutral/LED change falsely accepted")
	}
	for index := 1; index < dualShock4ControlCapacity; index++ {
		requireAuditControl(t, writer, []byte{byte(index), 80, 32, 0, 0, 0, 0})
	}
	requireAuditControl(t, writer, zero)
	requireAuditEmpty(t, writer)
	if !writer.EnqueueControl(StreamFrameOutputState, newZero) {
		t.Fatal("retained distinct zero could not retry after drain")
	}
	requireAuditControl(t, writer, newZero)
}

func TestFramedDS4TakenStateIsNotFoldedAndOwnsItsPayload(t *testing.T) {
	writer := newDualShock4OutputWriter(nil, StreamFrameVersionV3)
	payload := []byte{1, 2, 3, 4, 5, 0, 0}
	if !writer.EnqueueControl(StreamFrameOutputState, payload) {
		t.Fatal("initial admission")
	}
	var inFlight [7]byte
	if _, ok := writer.takeControl(inFlight[:]); !ok {
		t.Fatal("initial claim")
	}
	if !writer.EnqueueControl(StreamFrameOutputState, payload) || writer.controlCount != 1 {
		t.Fatal("in-flight repeat incorrectly folded")
	}
	payload[0] = 99
	if inFlight[0] != 1 {
		t.Fatal("producer mutation changed in-flight state")
	}
	requireAuditControl(t, writer, []byte{1, 2, 3, 4, 5, 0, 0})
}

func TestFramedDS4BlinkMediaAndStopBoundariesDoNotReusePendingTail(t *testing.T) {
	for _, boundary := range []string{"blink", "audio", "reset", "stop"} {
		t.Run(boundary, func(t *testing.T) {
			writer := newDualShock4OutputWriter(nil, StreamFrameVersionV3)
			state := []byte{1, 2, 3, 4, 5, 0, 0}
			if boundary == "blink" {
				state[5], state[6] = 4, 5
			}
			if !writer.EnqueueControl(StreamFrameOutputState, state) {
				t.Fatal("initial admission")
			}
			switch boundary {
			case "audio":
				writer.EnqueueAudioOwned(StreamFrameSpeakerPCM, []byte{0, 0, 0, 0})
			case "reset":
				writer.ResetSpeaker()
			case "stop":
				writer.requestStop()
				if writer.EnqueueControl(StreamFrameOutputState, state) {
					t.Fatal("retired transport accepted output")
				}
				requireAuditEmpty(t, writer)
				return
			}
			if !writer.EnqueueControl(StreamFrameOutputState, state) || writer.controlCount != 2 {
				t.Fatal("blink or media-separated repeat was folded")
			}
			requireAuditControl(t, writer, state)
			requireAuditControl(t, writer, state)
		})
	}
}

func TestFramedDS4USBAdmissionRejectsWithoutCommitAndReservesZero(t *testing.T) {
	for _, endpoint := range []uint8{EndpointOut, 0} {
		t.Run(map[uint8]string{EndpointOut: "interrupt", 0: "set-report"}[endpoint], func(t *testing.T) {
			dev, err := New(nil)
			if err != nil {
				t.Fatal(err)
			}
			writer := newDualShock4OutputWriter(nil, StreamFrameVersionV3)
			generation := bindAuditWriter(t, dev, writer)
			for index := 1; index < dualShock4ControlCapacity; index++ {
				report := auditOutputReport(byte(index), 80, 32)
				if handled, accepted := dev.TryHandleOutputCommand(endpoint, auditOutputSetup(report), report); !handled || !accepted {
					t.Fatalf("valid state %d was not admitted", index)
				}
			}
			before := dev.outputState
			rejected := auditOutputReport(99, 80, 33)
			if handled, accepted := dev.TryHandleOutputCommand(endpoint, auditOutputSetup(rejected), rejected); !handled || accepted {
				t.Fatal("ordinary overload did not return handled rejection")
			}
			if dev.outputState != before {
				t.Fatal("rejected state committed before admission")
			}
			zero := auditOutputReport(0, 0, 42)
			if handled, accepted := dev.TryHandleOutputCommand(endpoint, auditOutputSetup(zero), zero); !handled || !accepted {
				t.Fatal("reserved zero was not admitted")
			}
			newZero := auditOutputReport(0, 0, 43)
			if handled, accepted := dev.TryHandleOutputCommand(endpoint, auditOutputSetup(newZero), newZero); !handled || accepted {
				t.Fatal("full distinct zero/LED overload falsely succeeded")
			}
			if dev.outputState.RumbleSmall != 0 || dev.outputState.RumbleLarge != 0 || dev.outputState.LedRed != 42 {
				t.Fatal("failed admission leaked into cached replay state")
			}
			if !dev.clearFramedOutputCallbacks(generation) {
				t.Fatal("current transport did not retire")
			}
			next := newDualShock4OutputWriter(nil, StreamFrameVersionV3)
			bindAuditWriter(t, dev, next)
			requireAuditControl(t, next, []byte{0, 0, 42, 0, 0, 0, 0})
		})
	}
}

func TestFramedDS4AdmissionLeavesUnrelatedRequestsAndLegacyCallbackUnchanged(t *testing.T) {
	dev, err := New(nil)
	if err != nil {
		t.Fatal(err)
	}
	legacyCalls := 0
	dev.SetOutputCallback(func(OutputState) { legacyCalls++ })
	report := auditOutputReport(1, 2, 3)
	if handled, _ := dev.TryHandleOutputCommand(EndpointOut, [8]byte{}, report); handled {
		t.Fatal("legacy output intercepted")
	}
	dev.HandleTransfer(context.Background(), EndpointOut, usbip.DirOut, report)
	if legacyCalls != 1 {
		t.Fatal("legacy callback changed")
	}
	writer := newDualShock4OutputWriter(nil, StreamFrameVersionV3)
	bindAuditWriter(t, dev, writer)
	before := dev.outputState
	setup := auditOutputSetup(report)
	for index := range setup {
		wrong := setup
		wrong[index] ^= 0x80
		if handled, _ := dev.TryHandleOutputCommand(0, wrong, report); handled {
			t.Fatalf("unrelated setup byte %d intercepted", index)
		}
	}
	if handled, _ := dev.TryHandleOutputCommand(9, setup, report); handled {
		t.Fatal("unrelated endpoint intercepted")
	}
	for _, bad := range [][]byte{report[:10], {0xFF, 0, 0, 0, 1, 2, 3, 0, 0, 0, 0}} {
		if handled, accepted := dev.TryHandleOutputCommand(EndpointOut, [8]byte{}, bad); !handled || accepted {
			t.Fatal("malformed framed report accepted")
		}
	}
	if dev.outputState != before || legacyCalls != 1 {
		t.Fatal("unrelated/rejected traffic changed state or called legacy sink")
	}
}

func TestFramedDS4RegistrationFailureAndStaleCleanupCannotTouchSuccessor(t *testing.T) {
	dev, err := New(nil)
	if err != nil {
		t.Fatal(err)
	}
	dev.HandleTransfer(context.Background(), EndpointOut, usbip.DirOut, auditOutputReport(1, 2, 3))
	if generation, accepted := dev.setFramedOutputCallbacks(func(OutputState) bool { return false }, nil, nil); accepted || generation != 0 || dev.transportOutputFunc != nil {
		t.Fatal("rejected initial replay installed a transport")
	}
	first := newDualShock4OutputWriter(nil, StreamFrameVersionV3)
	firstGeneration := bindAuditWriter(t, dev, first)
	if _, accepted := dev.setFramedOutputCallbacks(func(OutputState) bool { return true }, nil, nil); accepted {
		t.Fatal("second consumer displaced current owner")
	}
	if !dev.clearFramedOutputCallbacks(firstGeneration) {
		t.Fatal("current owner could not retire")
	}
	second := newDualShock4OutputWriter(nil, StreamFrameVersionV3)
	secondGeneration := bindAuditWriter(t, dev, second)
	if secondGeneration == firstGeneration || dev.clearFramedOutputCallbacks(firstGeneration) {
		t.Fatal("old cleanup reached successor")
	}
	requireAuditControl(t, second, []byte{1, 2, 3, 0, 0, 0, 0})
	report := auditOutputReport(0, 0, 4)
	if handled, accepted := dev.TryHandleOutputCommand(EndpointOut, [8]byte{}, report); !handled || !accepted {
		t.Fatal("successor lost callback")
	}
	requireAuditControl(t, second, []byte{0, 0, 4, 0, 0, 0, 0})
	requireAuditControl(t, first, []byte{1, 2, 3, 0, 0, 0, 0})
	requireAuditEmpty(t, first)
}

func TestFramedDS4RejectedAndStaleStreamCannotReleaseSerialReservation(t *testing.T) {
	dev, err := New(nil)
	if err != nil {
		t.Fatal(err)
	}
	dev.SetMetaState(MetaState{SerialNumber: t.Name()})
	serial, firstReservation, accepted := claimDualShock4StreamSerial(dev)
	if !accepted {
		t.Fatal("initial serial claim rejected")
	}
	t.Cleanup(func() { releaseDualShock4Serial(serial, firstReservation) })
	writer := newDualShock4OutputWriter(nil, StreamFrameVersionV3)
	generation := bindAuditWriter(t, dev, writer)
	var device usb.Device = dev
	handler := &handler{speakerOutput: true, streamFrameVersion: StreamFrameVersionV3}
	if err := handler.StreamHandler()(nil, &device, slog.New(slog.NewTextHandler(io.Discard, nil))); err == nil {
		t.Fatal("second framed handler did not reject the active owner")
	}
	serialsMu.Lock()
	current := serials[serial]
	serialsMu.Unlock()
	if current != firstReservation {
		t.Fatal("rejected second handler released the active owner's serial")
	}
	if !dev.clearFramedOutputCallbacks(generation) {
		t.Fatal("first callback generation did not retire")
	}
	_, successor, accepted := claimDualShock4StreamSerial(dev)
	if !accepted || successor == firstReservation {
		t.Fatal("successor did not obtain a fresh serial ownership token")
	}
	t.Cleanup(func() { releaseDualShock4Serial(serial, successor) })
	if releaseDualShock4Serial(serial, firstReservation) {
		t.Fatal("stale handler released its successor's serial")
	}
	other, err := New(nil)
	if err != nil {
		t.Fatal(err)
	}
	other.SetMetaState(MetaState{SerialNumber: serial})
	if _, token, accepted := claimDualShock4StreamSerial(other); accepted || token != nil {
		t.Fatal("another device stole a live serial reservation")
	}
	if !releaseDualShock4Serial(serial, successor) {
		t.Fatal("current serial owner could not release its reservation")
	}
}

// Exercise the real USB/IP server and DS4 admission interface together, with
// the consumer deliberately paused. No mocked acceptance decision or hardware.
func TestFramedDS4WireOverflowReturnsENOSPCAndKeepsStreamUsable(t *testing.T) {
	for _, endpoint := range []uint32{EndpointOut, 0} {
		t.Run(map[uint32]string{EndpointOut: "interrupt", 0: "set-report"}[endpoint], func(t *testing.T) {
			dev, err := New(nil)
			if err != nil {
				t.Fatal(err)
			}
			writer := newDualShock4OutputWriter(nil, StreamFrameVersionV3)
			generation := bindAuditWriter(t, dev, writer)
			t.Cleanup(func() { dev.clearFramedOutputCallbacks(generation); writer.requestStop() })
			bus, err := virtualbus.NewWithBusID(71204)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = bus.Close() })
			registration, err := bus.AddRegistration(dev)
			if err != nil {
				t.Fatal(err)
			}
			server := srvusb.New(srvusb.ServerConfig{
				Addr: "127.0.0.1:0", ConnectionTimeout: 2 * time.Second,
			}, slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
			if err := server.AddBus(bus); err != nil {
				t.Fatal(err)
			}
			serverDone := make(chan error, 1)
			go func() { serverDone <- server.ListenAndServe() }()
			t.Cleanup(func() {
				_ = server.Close()
				select {
				case <-serverDone:
				case <-time.After(2 * time.Second):
					t.Error("USB/IP listener did not retire")
				}
			})
			select {
			case <-server.Ready():
			case err := <-serverDone:
				t.Fatalf("USB/IP startup: %v", err)
			case <-time.After(2 * time.Second):
				t.Fatal("USB/IP startup timed out")
			}
			conn, err := net.DialTimeout("tcp", server.Addr(), 2*time.Second)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = conn.Close() })
			if err := conn.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
				t.Fatal(err)
			}
			if err := (&usbip.MgmtHeader{Version: usbip.Version, Command: usbip.OpReqImport}).Write(conn); err != nil {
				t.Fatal(err)
			}
			busID := registration.Meta.USBBusID
			if _, err := conn.Write(busID[:]); err != nil {
				t.Fatal(err)
			}
			var importHeader [8]byte
			if err := usbip.ReadExactly(conn, importHeader[:]); err != nil {
				t.Fatal(err)
			}
			if binary.BigEndian.Uint16(importHeader[:2]) != usbip.Version ||
				binary.BigEndian.Uint16(importHeader[2:4]) != usbip.OpRepImport ||
				binary.BigEndian.Uint32(importHeader[4:8]) != 0 {
				t.Fatalf("USB/IP import did not succeed: %x", importHeader)
			}
			var imported [312]byte
			if err := usbip.ReadExactly(conn, imported[:]); err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(imported[256:288], busID[:]) {
				t.Fatal("USB/IP imported a different device identity")
			}
			var sequence uint32
			submit := func(report []byte) (int32, uint32) {
				t.Helper()
				sequence++
				command := usbip.CmdSubmit{Basic: usbip.HeaderBasic{Command: usbip.CmdSubmitCode, Seqnum: sequence, Dir: usbip.DirOut, Ep: endpoint}, TransferBufferLen: uint32(len(report)), NumberOfPackets: -1, Setup: auditOutputSetup(report)}
				if err := command.Write(conn); err != nil {
					t.Fatal(err)
				}
				if _, err := conn.Write(report); err != nil {
					t.Fatal(err)
				}
				var response [48]byte
				if err := usbip.ReadExactly(conn, response[:]); err != nil {
					t.Fatal(err)
				}
				if binary.BigEndian.Uint32(response[:4]) != usbip.RetSubmitCode || binary.BigEndian.Uint32(response[4:8]) != sequence {
					t.Fatal("wrong USB/IP completion identity")
				}
				return int32(binary.BigEndian.Uint32(response[20:24])), binary.BigEndian.Uint32(response[24:28])
			}
			for index := 1; index < dualShock4ControlCapacity; index++ {
				status, actual := submit(auditOutputReport(byte(index), 80, 32))
				if status != 0 || actual != 11 {
					t.Fatalf("admission %d: status=%d actual=%d", index, status, actual)
				}
			}
			status, actual := submit(auditOutputReport(99, 80, 33))
			if status != -28 || actual != 0 {
				t.Fatalf("ordinary overload falsely acknowledged: status=%d actual=%d", status, actual)
			}
			status, actual = submit(auditOutputReport(0, 0, 42))
			if status != 0 || actual != 11 {
				t.Fatal("reserved zero rejected on real wire")
			}
			status, actual = submit(auditOutputReport(0, 0, 43))
			if status != -28 || actual != 0 {
				t.Fatal("distinct zero/LED overload falsely acknowledged")
			}
			for index := 1; index < dualShock4ControlCapacity; index++ {
				requireAuditControl(t, writer, []byte{byte(index), 80, 32, 0, 0, 0, 0})
			}
			requireAuditControl(t, writer, []byte{0, 0, 42, 0, 0, 0, 0})
			status, actual = submit(auditOutputReport(0, 0, 43))
			if status != 0 || actual != 11 {
				t.Fatal("same USB/IP stream did not accept retry after drain")
			}
			requireAuditControl(t, writer, []byte{0, 0, 43, 0, 0, 0, 0})
		})
	}
}

func bindAuditWriter(t *testing.T, dev *DualShock4, writer *dualShock4OutputWriter) uint64 {
	t.Helper()
	generation, accepted := dev.setFramedOutputCallbacks(func(state OutputState) bool {
		payload, err := state.MarshalBinary()
		return err == nil && writer.EnqueueControl(StreamFrameOutputState, payload)
	}, nil, nil)
	if !accepted {
		t.Fatal("framed writer registration rejected")
	}
	return generation
}

func auditOutputReport(small, large, red byte) []byte {
	return []byte{ReportIDOutput, 7, 0, 0, small, large, red, 0, 0, 0, 0}
}

func auditOutputSetup(report []byte) [8]byte {
	setup := [8]byte{hidClassOUT, hidSetReport, ReportIDOutput, reportTypeOutput, InterfaceHID, 0, 0, 0}
	binary.LittleEndian.PutUint16(setup[6:8], uint16(len(report)))
	return setup
}

func requireAuditControl(t *testing.T, writer *dualShock4OutputWriter, expected []byte) {
	t.Helper()
	var payload [7]byte
	frameType, present := writer.takeControl(payload[:])
	if !present || frameType != StreamFrameOutputState || !bytes.Equal(expected, payload[:]) {
		t.Fatalf("control mismatch: present=%t type=%x got=%v want=%v", present, frameType, payload, expected)
	}
}

func requireAuditEmpty(t *testing.T, writer *dualShock4OutputWriter) {
	t.Helper()
	var payload [7]byte
	if _, present := writer.takeControl(payload[:]); present {
		t.Fatalf("unexpected historical control: %v", payload)
	}
}
