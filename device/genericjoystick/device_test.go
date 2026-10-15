package genericjoystick

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"testing"
	"time"

	"github.com/Alia5/VIIPER/device"
	"github.com/Alia5/VIIPER/internal/inputpresentation"
	"github.com/Alia5/VIIPER/usb"
	"github.com/Alia5/VIIPER/usbip"
)

func newTestJoystick(t *testing.T, buttons int) *Joystick {
	t.Helper()
	args, _ := json.Marshal(CreateOptions{Version: 1, Experimental: true, ButtonCount: buttons})
	j, err := New(&device.CreateOptions{DeviceSpecific: string(args)})
	if err != nil {
		t.Fatal(err)
	}
	return j
}

func present(t *testing.T, j *Joystick, at time.Time) []byte {
	t.Helper()
	b := make([]byte, ReportSize)
	c := j.ClaimInputPresentation(b, at)
	if !c.Valid() || !j.CanAdmitInputPresentation(c, at) || !j.ResolveInputPresentation(c, inputpresentation.OutcomeCommit, at) {
		t.Fatal("input not admitted")
	}
	return b
}

func TestWireAndHIDAreDifferentValueSpaces(t *testing.T) {
	s := InputState{X: -32767, Y: 0, Z: 32767, Rx: -123, Ry: 456, Rz: 789, Buttons: 0x801, Hat: HatNeutral}
	wire, err := s.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	// Shared with DS4Windows GenericJoystickMappingTests: one non-neutral
	// cross-language golden frame fixes axis order, signedness and endianness.
	if hex.EncodeToString(wire) != "0101800000ff7f85ffc8011503010808" {
		t.Fatalf("wire contract changed: %x", wire)
	}
	var decoded InputState
	if err := decoded.UnmarshalBinary(wire); err != nil || decoded != s {
		t.Fatalf("round trip: %v %+v", err, decoded)
	}
	hid := make([]byte, ReportSize)
	s.BuildReportInto(hid)
	for i, want := range []uint16{0, 32767, 65534, 32644, 33223, 33556} {
		if got := binary.LittleEndian.Uint16(hid[1+i*2:]); got != want {
			t.Fatalf("axis %d: %d != %d", i, got, want)
		}
	}
	if hid[0] != 1 || hid[15] != HatNeutral || binary.LittleEndian.Uint16(hid[13:]) != s.Buttons {
		t.Fatal("button/hat packing")
	}
}

func TestMalformedFramesDoNotMutateReceiver(t *testing.T) {
	good, _ := NewInputState().MarshalBinary()
	cases := [][]byte{good[:15], append(append([]byte{}, good...), 0), append([]byte{2}, good[1:]...)}
	badAxis := append([]byte{}, good...)
	binary.LittleEndian.PutUint16(badAxis[1:], 0x8000)
	cases = append(cases, badAxis)
	badHat := append([]byte{}, good...)
	badHat[15] = 9
	cases = append(cases, badHat)
	for _, data := range cases {
		s := InputState{X: 123, Hat: 2}
		before := s
		if s.UnmarshalBinary(data) == nil || s != before {
			t.Fatalf("accepted malformed frame %x", data)
		}
	}
}

func TestCreationRequiresExactExperimentalContract(t *testing.T) {
	for _, payload := range []string{"", `{}`, `{"version":1,"buttonCount":12}`, `{"version":2,"experimental":true,"buttonCount":12}`, `{"version":1,"experimental":true,"buttonCount":32}`, `{"version":1,"experimental":true,"buttonCount":12,"unknown":1}`, `{"version":1,"experimental":true,"buttonCount":12} {}`, `{"version":1,"experimental":true,"buttonCount":12,"neutralAxes":[0]}`} {
		if _, err := New(&device.CreateOptions{DeviceSpecific: payload}); err == nil {
			t.Fatalf("accepted %s", payload)
		}
	}
	if _, err := New(nil); err == nil {
		t.Fatal("accepted implicit creation")
	}
	id := uint16(0x054c)
	if _, err := New(&device.CreateOptions{IDVendor: &id, DeviceSpecific: `{"version":1,"experimental":true,"buttonCount":12}`}); err == nil {
		t.Fatal("accepted hardware impersonation")
	}
}

func TestDescriptorMatchesAxisButtonHatReport(t *testing.T) {
	for _, count := range []int{12, 16} {
		j := newTestJoystick(t, count)
		d := j.GetDescriptor()
		iface := d.Interfaces[0]
		if iface.Descriptor.BInterfaceProtocol != 0 || iface.Descriptor.BInterfaceSubClass != 0 || len(iface.Endpoints) != 1 || iface.Endpoints[0].WMaxPacketSize != ReportSize {
			t.Fatal("not a plain input-only HID joystick")
		}
		b, err := iface.HID.ReportDescriptor.Bytes()
		if err != nil {
			t.Fatal(err)
		}
		// Parse the encoded HID items independently of the report encoder. Six
		// named 16-bit absolute axes, button bits/padding and a null-state hat
		// must occupy exactly the fifteen payload bytes following report ID 1.
		page, size, n, total, reportID := uint32(0), uint32(0), uint32(0), uint32(0), uint32(0)
		usages := []uint32{}
		axes := []uint32{}
		hatNull := false
		buttonBits := uint32(0)
		for i := 0; i < len(b); {
			h := b[i]
			i++
			width := int(h & 3)
			if width == 3 {
				width = 4
			}
			if i+width > len(b) {
				t.Fatal("truncated HID item")
			}
			var value uint32
			for k := 0; k < width; k++ {
				value |= uint32(b[i+k]) << (8 * k)
			}
			i += width
			typ, tag := (h>>2)&3, h>>4
			if typ == 1 {
				switch tag {
				case 0:
					page = value
				case 7:
					size = value
				case 8:
					reportID = value
				case 9:
					n = value
				}
			}
			if typ == 2 && tag == 0 {
				usages = append(usages, value)
			}
			if typ == 0 {
				if tag == 8 {
					total += size * n
					if page == 1 && size == 16 {
						axes = append(axes, usages...)
						if n != 6 || value&7 != 2 {
							t.Fatal("axes not absolute variables")
						}
					}
					if page == 9 && value&1 == 0 {
						buttonBits += size * n
					}
					if page == 1 && size == 4 && value&1 == 0 {
						hatNull = value&0x40 != 0
					}
				}
				usages = nil
			}
		}
		if reportID != 1 || total != 120 || buttonBits != uint32(count) || !hatNull {
			t.Fatalf("descriptor shape id=%d bits=%d buttons=%d null=%v", reportID, total, buttonBits, hatNull)
		}
		for i, want := range []uint32{0x30, 0x31, 0x32, 0x33, 0x34, 0x35} {
			if len(axes) != 6 || axes[i] != want {
				t.Fatalf("axis usages %v", axes)
			}
		}
	}
}

func TestButtonAndHatEdgesRemainOrderedWhileAxesCoalesce(t *testing.T) {
	j := newTestJoystick(t, 12)
	lease, _ := j.acquireProducer()
	at := time.Now()
	states := []InputState{{X: 1, Hat: 8}, {X: 2, Hat: 8}, {X: 2, Buttons: 1, Hat: 8}, {X: 3, Buttons: 1, Hat: 2}, {X: 4, Hat: 8}}
	for i, s := range states {
		if err := j.publish(lease, s, at.Add(time.Duration(i)*time.Microsecond)); err != nil {
			t.Fatal(err)
		}
	}
	got := []InputState{}
	for range 4 {
		b := present(t, j, at.Add(time.Millisecond))
		s := InputState{X: int16(int(binary.LittleEndian.Uint16(b[1:])) - 32767), Buttons: binary.LittleEndian.Uint16(b[13:]), Hat: b[15]}
		got = append(got, s)
	}
	if got[0].X != 2 || got[1].Buttons != 1 || got[2].Hat != 2 || got[3].Buttons != 0 || got[3].Hat != 8 {
		t.Fatalf("ordered reports %+v", got)
	}
}

func TestRetirementRevokesClaimAndReturnsNeutral(t *testing.T) {
	j := newTestJoystick(t, 12)
	lease, _ := j.acquireProducer()
	at := time.Now()
	j.publish(lease, InputState{X: 100, Buttons: 1, Hat: 8}, at)
	present(t, j, at.Add(time.Millisecond))
	b := make([]byte, ReportSize)
	_, version := j.SnapshotInputReportInto(b)
	c := j.ClaimInputPresentation(b, at.Add(2*time.Millisecond))
	j.releaseProducer(lease)
	if j.CanAdmitInputPresentation(c, time.Now()) || j.InputReportSnapshotCurrent(version) {
		t.Fatal("retired input still admissible")
	}
	if err := j.publish(lease, InputState{Buttons: 1, Hat: 8}, time.Now()); err == nil {
		t.Fatal("retired producer accepted")
	}
	neutral := present(t, j, time.Now())
	want := make([]byte, ReportSize)
	NewInputState().BuildReportInto(want)
	if !bytes.Equal(neutral, want) {
		t.Fatalf("not neutral: %x", neutral)
	}
	newLease, ok := j.acquireProducer()
	if !ok {
		t.Fatal("successor refused")
	}
	if err := j.publish(newLease, InputState{X: 200, Hat: 8}, time.Now()); err != nil {
		t.Fatal(err)
	}
	if got := present(t, j, time.Now()); binary.LittleEndian.Uint16(got[1:]) != 32967 {
		t.Fatal("successor did not resynchronize")
	}
}

func TestEndpointRetirementCannotReplayOldState(t *testing.T) {
	j := newTestJoystick(t, 12)
	lease, _ := j.acquireProducer()
	j.publish(lease, InputState{Buttons: 1, Hat: 8}, time.Now())
	present(t, j, time.Now())
	gen := j.InputPresentationGeneration()
	if !j.RetireInputPresentationGeneration(gen, time.Now()) || j.RetireInputPresentationGeneration(gen, time.Now()) {
		t.Fatal("generation fencing")
	}
	b := present(t, j, time.Now())
	if binary.LittleEndian.Uint16(b[13:]) != 0 {
		t.Fatal("replayed old press")
	}
}

func TestConfigurationFenceRequiresPostEnumerationProducer(t *testing.T) {
	j := newTestJoystick(t, 12)
	early, _ := j.acquireProducer()
	if !j.RetireInputPresentationGenerationForImport(j.InputPresentationGeneration(), time.Now(), true) {
		t.Fatal("configuration fence refused")
	}
	if err := j.publish(early, InputState{X: 32767, Hat: 8}, time.Now()); err == nil {
		t.Fatal("pre-configuration producer survived the fence")
	}
	present(t, j, time.Now()) // Windows reads neutral before lab input connects.
	late, ok := j.acquireProducer()
	if !ok {
		t.Fatal("post-enumeration producer refused")
	}
	j.releaseProducer(early) // Closing the old stream must not revoke its successor.
	if err := j.publish(late, InputState{X: 32767, Hat: 8}, time.Now()); err != nil {
		t.Fatal(err)
	}
	if got := present(t, j, time.Now()); binary.LittleEndian.Uint16(got[1:]) != 65534 {
		t.Fatal("post-enumeration producer did not deliver movement")
	}
}

func TestNativeReadinessFollowsExactImportLifecycle(t *testing.T) {
	j := newTestJoystick(t, 12)
	if j.GetDeviceSpecificArgs()["inputConfigured"] != false {
		t.Fatal("ready before initial configuration")
	}
	old := j.InputPresentationGeneration()
	if !j.RetireInputPresentationGenerationForImport(old, time.Now(), true) {
		t.Fatal("configuration refused")
	}
	if j.GetDeviceSpecificArgs()["inputConfigured"] != false {
		t.Fatal("endpoint reset guessed configuration readiness")
	}
	j.USBConfigurationChanged(1)
	if j.GetDeviceSpecificArgs()["inputConfigured"] != true {
		t.Fatal("not ready after configuration")
	}
	current := j.InputPresentationGeneration()
	if j.RetireInputPresentationGenerationForImport(old, time.Now(), false) {
		t.Fatal("stale import closed successor")
	}
	if j.GetDeviceSpecificArgs()["inputConfigured"] != true {
		t.Fatal("stale close changed readiness")
	}
	j.USBConfigurationChanged(0)
	if j.GetDeviceSpecificArgs()["inputConfigured"] != false {
		t.Fatal("ready after unconfiguration")
	}
	j.USBConfigurationChanged(1)
	if !j.RetireInputPresentationGenerationForImport(current, time.Now(), false) {
		t.Fatal("import close refused")
	}
	if j.GetDeviceSpecificArgs()["inputConfigured"] != false {
		t.Fatal("ready after native import closed")
	}
}

func TestStaleHistoryFencesControlSnapshot(t *testing.T) {
	j := newTestJoystick(t, 12)
	lease, _ := j.acquireProducer()
	at := time.Now()
	j.publish(lease, InputState{Buttons: 1, Hat: 8}, at)
	present(t, j, at.Add(time.Millisecond))
	j.publish(lease, InputState{Hat: 8}, at.Add(2*time.Millisecond))
	old := make([]byte, ReportSize)
	_, v := j.SnapshotInputReportInto(old)
	b := present(t, j, at.Add(time.Second))
	if binary.LittleEndian.Uint16(b[13:]) != 0 || j.InputReportSnapshotCurrent(v) {
		t.Fatal("stale history/snapshot not fenced")
	}
}

func TestWrongEndpointAndCancelledPollCannotConsumeJournal(t *testing.T) {
	j := newTestJoystick(t, 12)
	lease, _ := j.acquireProducer()
	j.publish(lease, InputState{Buttons: 1, Hat: 8}, time.Now())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if j.HandleTransfer(ctx, 1, usbip.DirIn, nil) != nil || j.HandleTransfer(context.Background(), 2, usbip.DirIn, nil) != nil || j.HandleTransfer(context.Background(), 1, usbip.DirOut, nil) != nil {
		t.Fatal("wrong poll accepted")
	}
	if b := present(t, j, time.Now()); binary.LittleEndian.Uint16(b[13:]) != 1 {
		t.Fatal("edge consumed by unrelated endpoint")
	}
}

func TestUnchangedIdlePollDoesNotStarveControlSnapshot(t *testing.T) {
	j := newTestJoystick(t, 12)
	b := make([]byte, ReportSize)
	_, version := j.SnapshotInputReportInto(b)
	for range 16 {
		present(t, j, time.Now())
	}
	if !j.InputReportSnapshotCurrent(version) {
		t.Fatal("identical idle reports invalidated an unchanged control snapshot")
	}
}

func TestStreamFragmentationAndDisconnectReset(t *testing.T) {
	j := newTestJoystick(t, 12)
	var dev usb.Device = j
	server, client := net.Pipe()
	defer client.Close()
	done := make(chan error, 1)
	go func() {
		defer server.Close()
		done <- (&handler{}).StreamHandler()(server, &dev, slog.New(slog.NewTextHandler(io.Discard, nil)))
	}()
	wire, _ := (InputState{Buttons: 1, Hat: 8}).MarshalBinary()
	client.SetWriteDeadline(time.Now().Add(time.Second))
	if _, err := client.Write(wire[:3]); err != nil {
		t.Fatal(err)
	}
	if b := present(t, j, time.Now()); binary.LittleEndian.Uint16(b[13:]) != 0 {
		t.Fatal("partial frame published")
	}
	if _, err := client.Write(wire[3:]); err != nil {
		t.Fatal(err)
	}
	client.Close()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("disconnect blocked")
	}
	if b := present(t, j, time.Now()); binary.LittleEndian.Uint16(b[13:]) != 0 {
		t.Fatal("disconnect left button held")
	}
}
