package usb

import (
	"context"
	"encoding/binary"
	"io"
	"log/slog"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/Alia5/VIIPER/usbip"
	"github.com/Alia5/VIIPER/virtualbus"
	"github.com/stretchr/testify/require"
)

type pacedNativeOutputTestDevice struct {
	*outputCommandAdmissionTestDevice
}

func (d *pacedNativeOutputTestDevice) OutputCommandServiceEndpoint(endpoint uint8, setup [8]byte, data []byte) (uint8, bool) {
	return 1, len(data) != 0 && data[0] == 2 && (endpoint == 0 || endpoint == 1)
}

func TestNativeOutputBurstUsesAdvertisedServiceCadenceAcrossInterruptAndControl(t *testing.T) {

	for _, test := range []struct {
		name      string
		speed     uint32
		bInterval uint8
		interval  time.Duration
	}{
		{"high-speed-1000Hz", 3, 4, time.Millisecond},
		{"high-speed-500Hz", 3, 5, 2 * time.Millisecond},
		{"high-speed-250Hz", 3, 6, 4 * time.Millisecond},
		{"full-speed-125Hz", 2, 8, 8 * time.Millisecond},
	} {
		t.Run(test.name, func(t *testing.T) {
			device := &pacedNativeOutputTestDevice{&outputCommandAdmissionTestDevice{
				transactionalInterruptOutTestDevice: newTransactionalInterruptOutTestDevice(),
			}}
			device.accept.Store(true)
			device.desc.Device.Speed = test.speed
			device.desc.Interfaces[0].Endpoints[0].BInterval = test.bInterval
			bus := virtualbus.New(249)
			_, err := bus.Add(device)
			require.NoError(t, err)
			server := New(ServerConfig{}, slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
			require.NoError(t, server.AddBus(bus))
			serverConn, clientConn := net.Pipe()
			require.NoError(t, clientConn.SetDeadline(time.Now().Add(5*time.Second)))
			done := make(chan error, 1)
			go func() { done <- server.handleUrbStream(serverConn, device) }()
			defer func() { _ = clientConn.Close(); _ = serverConn.Close(); _ = bus.Close(); <-done }()
			const count = 32
			started := time.Now()
			for seq := uint32(1); seq <= count; seq++ {
				endpoint := uint32(1)
				if seq%2 == 0 {
					endpoint = 0
				}
				payload := []byte{2, 3, 0, byte(seq), byte(seq)}
				command := usbip.CmdSubmit{
					Basic:             usbip.HeaderBasic{Command: usbip.CmdSubmitCode, Seqnum: seq, Dir: usbip.DirOut, Ep: endpoint},
					TransferBufferLen: uint32(len(payload)), NumberOfPackets: -1,
					Setup: [8]byte{0x21, 0x09, 0x02, 0x02, 0, 0, byte(len(payload)), 0},
				}
				require.NoError(t, command.Write(clientConn))
				_, err := clientConn.Write(payload)
				require.NoError(t, err)
				var response [retSubmitHeaderSize]byte
				require.NoError(t, usbip.ReadExactly(clientConn, response[:]))
				require.Equal(t, seq, binary.BigEndian.Uint32(response[4:8]))
				require.Zero(t, int32(binary.BigEndian.Uint32(response[20:24])))
				require.Equal(t, uint32(len(payload)), binary.BigEndian.Uint32(response[24:28]))
			}
			elapsed := time.Since(started)
			t.Logf("%d native writes through USB/IP in %s; advertised minimum span %s", count, elapsed, time.Duration(count-1)*test.interval)
			require.GreaterOrEqual(t, elapsed, time.Duration(count-1)*test.interval,
				"Native HID output bypassed its service clock and acknowledged a burst faster than the emulated endpoint")
			require.Equal(t, int32(count), device.attempts.Load())
		})
	}
}

// Record the exact immutable command admitted at an endpoint opportunity.
type recordedNativeCommand struct {
	endpoint uint8
	payload  []byte
}
type recordingNativeCadenceDevice struct {
	*pacedNativeOutputTestDevice
	commandsMu sync.Mutex
	commands   []recordedNativeCommand
}

func newRecordingNativeCadenceDevice() *recordingNativeCadenceDevice {
	d := &recordingNativeCadenceDevice{pacedNativeOutputTestDevice: &pacedNativeOutputTestDevice{&outputCommandAdmissionTestDevice{transactionalInterruptOutTestDevice: newTransactionalInterruptOutTestDevice()}}}
	d.accept.Store(true)
	return d
}
func (d *recordingNativeCadenceDevice) TryHandleOutputCommand(ep uint8, setup [8]byte, payload []byte) (bool, bool) {
	handled, accepted := d.pacedNativeOutputTestDevice.TryHandleOutputCommand(ep, setup, payload)
	if handled && accepted {
		d.commandsMu.Lock()
		d.commands = append(d.commands, recordedNativeCommand{ep, append([]byte(nil), payload...)})
		d.commandsMu.Unlock()
	}
	return handled, accepted
}
func (d *recordingNativeCadenceDevice) snapshot() []recordedNativeCommand {
	d.commandsMu.Lock()
	defer d.commandsMu.Unlock()
	return append([]recordedNativeCommand(nil), d.commands...)
}

func TestNativeOutputFakeClockPreservesCopiesOrderStopsAndIndependentInput(t *testing.T) {
	base := time.Unix(1000, 0)
	clock := newFakeEndpointClock(base)
	device := newRecordingNativeCadenceDevice()
	ctx, cancel := context.WithCancel(context.Background())
	recorder := newRecordingWriter()
	worker := newEndpointWorkerWithClock(ctx, device, 1, usbip.DirOut, nativeOutputWorker, 4*time.Millisecond, 64, newResponseWriter(recorder, nil), func(err error) { t.Error(err) }, clock)
	// A separate input endpoint must stay usable while output is reserved later.
	inputRecorder := newRecordingWriter()
	inputClock := newFakeEndpointClock(base)
	inputWorker := newEndpointWorkerWithClock(ctx, &schedulerTestDevice{desc: testCompositeDescriptor()}, 4, usbip.DirIn, interruptInWorker, time.Millisecond, 64, newResponseWriter(inputRecorder, nil), func(err error) { t.Error(err) }, inputClock)
	defer func() { cancel(); worker.signal(); inputWorker.signal(); <-worker.done; <-inputWorker.done }()
	first := base.Add(4 * time.Millisecond)
	payloads := [][]byte{{2, 3, 0, 90, 120}, {2, 3, 0, 0, 0}, {2, 3, 0, 35, 60}}
	for index, payload := range payloads {
		ep := uint8(1)
		if index == 1 {
			ep = 0
		}
		require.True(t, worker.enqueueWithOutputCommand(uint32(index+1), uint32(len(payload)), payload, nil, first, 0, ep, [8]byte{0x21, 9, 2, 2, 0, 0, 5, 0}))
	}
	// The USB/IP receive scratch may be reused immediately after enqueue.
	payloads[0][3] = 255
	require.True(t, inputWorker.enqueue(99, 64, nil, nil, base.Add(time.Millisecond)))
	inputClock.waitForDeadline(t, base.Add(time.Millisecond))
	clock.advance(time.Millisecond)
	inputClock.advance(time.Millisecond)
	inputRecorder.waitForWrites(t, 1)
	require.Empty(t, device.snapshot())
	clock.waitForDeadline(t, first)
	clock.advance(3 * time.Millisecond)
	recorder.waitForWrites(t, 1)
	require.Equal(t, byte(90), device.snapshot()[0].payload[3])
	clock.waitForDeadline(t, first.Add(4*time.Millisecond))
	clock.advance(4 * time.Millisecond)
	recorder.waitForWrites(t, 2)
	require.Equal(t, uint8(0), device.snapshot()[1].endpoint)
	require.Equal(t, []byte{2, 3, 0, 0, 0}, device.snapshot()[1].payload)
	clock.waitForDeadline(t, first.Add(8*time.Millisecond))
	clock.advance(4 * time.Millisecond)
	recorder.waitForWrites(t, 3)
	require.Equal(t, byte(35), device.snapshot()[2].payload[3])
}

func TestNativeOutputPendingUnlinkAndResetHaveNoDeviceSideEffect(t *testing.T) {
	for _, operation := range []string{"unlink", "reset"} {
		t.Run(operation, func(t *testing.T) {
			clock := newFakeEndpointClock(time.Unix(1100, 0))
			device := newRecordingNativeCadenceDevice()
			recorder := newRecordingWriter()
			ctx, cancel := context.WithCancel(context.Background())
			worker := newEndpointWorkerWithClock(ctx, device, 1, usbip.DirOut, nativeOutputWorker, 4*time.Millisecond, 64, newResponseWriter(recorder, nil), func(err error) { t.Error(err) }, clock)
			defer func() { cancel(); worker.signal(); <-worker.done }()
			service := clock.Now().Add(4 * time.Millisecond)
			require.True(t, worker.enqueueWithOutputCommand(1, 5, []byte{2, 3, 0, 90, 120}, nil, service, 0, 1, [8]byte{}))
			clock.waitForDeadline(t, service)
			if operation == "unlink" {
				require.True(t, worker.unlinkAt(1, clock.Now()))
			} else {
				worker.reset()
			}
			require.Eventually(t, func() bool {
				worker.mu.Lock()
				defer worker.mu.Unlock()
				return worker.freeCount == endpointQueueCapacity
			}, time.Second, time.Millisecond)
			require.Empty(t, device.snapshot())
			next := clock.Now().Add(4 * time.Millisecond)
			require.True(t, worker.enqueueWithOutputCommand(2, 5, []byte{2, 3, 0, 0, 0}, nil, next, 0, 1, [8]byte{}))
			clock.waitForDeadline(t, next)
			clock.advance(4 * time.Millisecond)
			writes := recorder.waitForWrites(t, 1)
			require.Equal(t, uint32(2), binary.BigEndian.Uint32(writes[0].packet[4:8]))
			require.Len(t, device.snapshot(), 1)
		})
	}
}

func TestNativeOutputAdmissionCannotBeUnlinkedAfterAcceptedSideEffect(t *testing.T) {
	clock := newFakeEndpointClock(time.Unix(1200, 0))
	device := newRecordingNativeCadenceDevice()
	recorder := newRecordingWriter()
	responses := newResponseWriter(recorder, nil)
	ctx, cancel := context.WithCancel(context.Background())
	worker := newEndpointWorkerWithClock(ctx, device, 1, usbip.DirOut, nativeOutputWorker, 4*time.Millisecond, 64, responses, func(err error) { t.Error(err) }, clock)
	defer func() { cancel(); worker.signal(); <-worker.done }()
	responses.mu.Lock()
	service := clock.Now().Add(4 * time.Millisecond)
	require.True(t, worker.enqueueWithOutputCommand(1, 5, []byte{2, 3, 0, 90, 120}, nil, service, 0, 1, [8]byte{}))
	clock.waitForDeadline(t, service)
	clock.advance(4 * time.Millisecond)
	accepted := false
	for deadline := time.Now().Add(time.Second); time.Now().Before(deadline); {
		if len(device.snapshot()) == 1 {
			accepted = true
			break
		}
		time.Sleep(time.Millisecond)
	}
	// Release even on failure so the worker cannot deadlock test cleanup.
	cancelled := worker.unlink(1)
	responses.mu.Unlock()
	require.True(t, accepted)
	require.False(t, cancelled, "an admitted effect cannot be reported as cancelled")
	recorder.waitForWrites(t, 1)
}

func TestNativeOutputQueueHasBoundedCapacityAndReportsRealAdmissionFailure(t *testing.T) {
	clock := newFakeEndpointClock(time.Unix(1300, 0))
	device := newRecordingNativeCadenceDevice()
	recorder := newRecordingWriter()
	ctx, cancel := context.WithCancel(context.Background())
	worker := newEndpointWorkerWithClock(ctx, device, 1, usbip.DirOut, nativeOutputWorker, 4*time.Millisecond, 64, newResponseWriter(recorder, nil), func(err error) { t.Error(err) }, clock)
	defer func() { cancel(); worker.signal(); <-worker.done }()
	service := clock.Now().Add(4 * time.Millisecond)
	require.False(t, worker.enqueueWithOutputCommand(999, 65, make([]byte, 65), nil, service, 0, 1, [8]byte{}))
	require.False(t, worker.enqueueWithOutputCommand(998, 64, []byte{2, 3, 0, 0, 0}, nil, service, 0, 1, [8]byte{}))
	for seq := uint32(1); seq <= endpointQueueCapacity; seq++ {
		require.True(t, worker.enqueueWithOutputCommand(seq, 5, []byte{2, 3, 0, byte(seq), 1}, nil, service, 0, 1, [8]byte{}))
	}
	require.False(t, worker.enqueueWithOutputCommand(1000, 5, []byte{2, 3, 0, 0, 0}, nil, service, 0, 1, [8]byte{}))
	require.Empty(t, device.snapshot())
	device.accept.Store(false)
	clock.waitForDeadline(t, service)
	clock.advance(4 * time.Millisecond)
	writes := recorder.waitForWrites(t, 1)
	require.Equal(t, int32(errNoSpace), int32(binary.BigEndian.Uint32(writes[0].packet[20:24])))
	require.Zero(t, binary.BigEndian.Uint32(writes[0].packet[24:28]))
	require.Empty(t, device.snapshot())
}
