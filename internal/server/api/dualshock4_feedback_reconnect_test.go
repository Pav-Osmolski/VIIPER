package api_test

import (
	"context"
	"encoding/binary"
	"hash/crc32"
	"io"
	"log/slog"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Alia5/VIIPER/device/dualshock4"
	"github.com/Alia5/VIIPER/internal/log"
	"github.com/Alia5/VIIPER/internal/server/api"
	"github.com/Alia5/VIIPER/internal/server/api/handler"
	srvusb "github.com/Alia5/VIIPER/internal/server/usb"
	"github.com/Alia5/VIIPER/viiperclient"
	"github.com/Alia5/VIIPER/virtualbus"
)

// Uses actual localhost API stream ownership and framed DS4 callbacks. No USB
// server listener, host driver, imported device, HID handle, or hardware is used.
func TestAPIServer_DS4OverlappingFramedReconnectPreservesFeedbackOwner(t *testing.T) {
	const (
		busID          = uint32(71006)
		cleanupTimeout = 750 * time.Millisecond
	)
	usbServer := srvusb.New(srvusb.ServerConfig{Addr: "127.0.0.1:0"},
		slog.Default(), log.NewRaw(nil))
	apiServer := api.New(usbServer, "127.0.0.1:0", api.ServerConfig{
		Addr:                        "127.0.0.1:0",
		DeviceHandlerConnectTimeout: cleanupTimeout,
	}, slog.Default())
	apiServer.Router().Register("bus/{id}/add", handler.BusDeviceAdd(usbServer, apiServer))
	apiServer.Router().RegisterStream("bus/{busId}/{deviceid}", api.DeviceStreamHandler(usbServer))
	require.NoError(t, apiServer.Start())
	defer apiServer.Close()
	defer usbServer.Close() //nolint:errcheck

	bus, err := virtualbus.NewWithBusID(busID)
	require.NoError(t, err)
	defer bus.Close() //nolint:errcheck
	require.NoError(t, usbServer.AddBus(bus))
	client := viiperclient.New(apiServer.Addr())
	created, err := client.DeviceAdd(busID, "dualshock4audioduplexv3", nil)
	require.NoError(t, err)
	require.NotNil(t, created)
	devices := bus.Devices()
	require.Len(t, devices, 1)
	ds4, ok := devices[0].(*dualshock4.DualShock4)
	require.True(t, ok)
	ds4.SetInterfaceAltSetting(dualshock4.InterfaceMicrophone, 1)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	first, err := client.OpenStream(ctx, busID, created.DevID)
	require.NoError(t, err)
	defer first.Close() //nolint:errcheck
	writeDS4OverlapMicrophone(t, first, 0, 0x11)
	require.Eventually(t, func() bool {
		return ds4.GetDeviceSpecificArgs()["queuedMicrophoneBytes"] == dualshock4.USBMicrophoneClientFrameSize
	}, 2*time.Second, 5*time.Millisecond, "The first framed handler must own callbacks before displacement.")

	admitDS4OverlapFeedback(t, ds4, 11, 22, 33)
	readDS4OverlapFeedback(t, first, 0, []byte{11, 22, 33, 0, 0, 0, 0})

	// Do not close first: the real API coordinator must close and finish its
	// handler before the replacement can register the new framed callbacks.
	second, err := client.OpenStream(ctx, busID, created.DevID)
	require.NoError(t, err)
	defer second.Close() //nolint:errcheck
	writeDS4OverlapMicrophone(t, second, 0, 0x44)
	require.Eventually(t, func() bool {
		return ds4.GetDeviceSpecificArgs()["queuedMicrophoneBytes"] == 2*dualshock4.USBMicrophoneClientFrameSize
	}, 2*time.Second, 5*time.Millisecond, "Replacement input proves its framed handler started after ownership handoff.")

	require.NoError(t, first.SetReadDeadline(time.Now().Add(2*time.Second)))
	var oldByte [1]byte
	n, err := first.Read(oldByte[:])
	require.Zero(t, n)
	require.Error(t, err, "The displaced connection must not remain a feedback consumer.")
	if networkError, ok := err.(net.Error); ok {
		require.False(t, networkError.Timeout(), "An open idle predecessor is not a completed handoff.")
	}

	// Initial replay belongs to the successor's new sequence space. Subsequent
	// commands must not enter the retired writer or be consumed by stale cleanup.
	readDS4OverlapFeedback(t, second, 0, []byte{11, 22, 33, 0, 0, 0, 0})
	admitDS4OverlapFeedback(t, ds4, 55, 66, 77)
	readDS4OverlapFeedback(t, second, 1, []byte{55, 66, 77, 0, 0, 0, 0})

	// Cross the old handler's removal deadline while the successor remains
	// live. This is bounded lifecycle validation, not a transport timing claim.
	time.Sleep(cleanupTimeout + 50*time.Millisecond)
	require.Len(t, bus.Devices(), 1)
	require.Same(t, ds4, bus.Devices()[0])
	writeDS4OverlapMicrophone(t, second, 1, 0x55)
	require.Eventually(t, func() bool {
		return ds4.GetDeviceSpecificArgs()["queuedMicrophoneBytes"] == 3*dualshock4.USBMicrophoneClientFrameSize
	}, 2*time.Second, 5*time.Millisecond)
	admitDS4OverlapFeedback(t, ds4, 0, 0, 88)
	readDS4OverlapFeedback(t, second, 2, []byte{0, 0, 88, 0, 0, 0, 0})
}

func writeDS4OverlapMicrophone(t *testing.T, stream *viiperclient.DeviceStream, sequence uint32, value byte) {
	t.Helper()
	payload := make([]byte, dualshock4.USBMicrophoneClientFrameSize)
	for index := range payload {
		payload[index] = value
	}
	frame := makeDS4StreamFrame(sequence, payload)
	require.NoError(t, stream.SetWriteDeadline(time.Now().Add(2*time.Second)))
	n, err := stream.Write(frame)
	require.NoError(t, err)
	require.Equal(t, len(frame), n)
}

func admitDS4OverlapFeedback(t *testing.T, ds4 *dualshock4.DualShock4, small, large, red byte) {
	t.Helper()
	report := []byte{dualshock4.ReportIDOutput, 7, 0, 0, small, large, red, 0, 0, 0, 0}
	handled, accepted := ds4.TryHandleOutputCommand(dualshock4.EndpointOut, [8]byte{}, report)
	require.True(t, handled, "The actual API framed handler must own admission.")
	require.True(t, accepted)
}

func readDS4OverlapFeedback(t *testing.T, stream *viiperclient.DeviceStream, sequence uint32, expected []byte) {
	t.Helper()
	require.NoError(t, stream.SetReadDeadline(time.Now().Add(2*time.Second)))
	var header [16]byte
	_, err := io.ReadFull(stream, header[:])
	require.NoError(t, err)
	require.Equal(t, []byte{'V', 'P', 'C', 'M'}, header[:4])
	require.Equal(t, byte(dualshock4.StreamFrameVersionV3), header[4])
	require.Equal(t, byte(dualshock4.StreamFrameOutputState), header[5])
	require.Equal(t, uint16(len(expected)), binary.LittleEndian.Uint16(header[6:8]))
	require.Equal(t, sequence, binary.LittleEndian.Uint32(header[8:12]))
	payload := make([]byte, len(expected))
	_, err = io.ReadFull(stream, payload)
	require.NoError(t, err)
	require.Equal(t, expected, payload)
	checksum := crc32.NewIEEE()
	_, _ = checksum.Write(header[4:12])
	_, _ = checksum.Write(payload)
	require.Equal(t, checksum.Sum32(), binary.LittleEndian.Uint32(header[12:16]))
}
