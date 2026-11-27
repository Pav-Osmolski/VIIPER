package ns2pro

import (
	"context"
	"encoding/binary"
	"io"
	"math"
	"testing"
	"time"

	viiperTesting "github.com/Alia5/VIIPER/_testing"
	"github.com/Alia5/VIIPER/internal/server/api"
	apihandler "github.com/Alia5/VIIPER/internal/server/api/handler"
	"github.com/Alia5/VIIPER/usbip"
	"github.com/Alia5/VIIPER/viiperclient"
	"github.com/Alia5/VIIPER/virtualbus"
	"github.com/stretchr/testify/require"
)

func TestGameCubeDescriptorAndReports(t *testing.T) {
	registration := api.GetRegistration("ns2gamecube")
	require.NotNil(t, registration)
	d, err := newDevice(nil, true)
	require.NoError(t, err)
	desc := d.GetDescriptor()
	require.Equal(t, uint16(0x057e), desc.Device.IDVendor)
	require.Equal(t, uint16(0x2073), desc.Device.IDProduct)
	require.Equal(t, uint16(0x0100), desc.Device.BcdDevice)
	require.Equal(t, "Nintendo GameCube Controller", desc.Strings[2])
	require.Len(t, desc.Associations, 2)
	require.Equal(t, uint8(4), desc.Interfaces[0].Endpoints[0].BInterval)
	require.Equal(t, "WINUSB", desc.MicrosoftOS10.CompatibleID)
	report, err := desc.Interfaces[0].HID.ReportDescriptor.Bytes()
	require.NoError(t, err)
	require.Equal(t, mustHexBytes(t, "05010905a101850505ff0901150026ff00953f75088102850a090195028102050919012915250195157501810295017503810305010901a100093009310933093526ff0f9504750c8102c005ff090226ff0095347508810285030901953f9102c0"), []byte(report))

	ctx := context.Background()
	state := *NewInputState()
	state.Buttons = ButtonA | ButtonX | ButtonR | ButtonC | ButtonZL
	state.LX, state.LY = 0x123, 0xabc
	state.LeftTrigger, state.RightTrigger = 223, 61
	require.True(t, d.UpdateInputState(state))
	d.HandleTransfer(ctx, 2, usbip.DirOut, enableReportsCommand())
	input := d.HandleTransfer(ctx, 1, usbip.DirIn, nil)
	require.Len(t, input, 64)
	require.Equal(t, byte(0x0a), input[0])
	require.Equal(t, []byte{0x25, 0x10, 0x10, 0x23, 0xc1, 0xab}, input[3:9])
	require.Equal(t, []byte{223, 61}, input[13:15])

	input, ok := d.HandleControl(hidClassRequestIn, hidGetReport, 0x0105, 0, 64, nil)
	require.True(t, ok)
	require.Equal(t, []byte{0x85, 0x40, 0x40}, input[5:8])
	require.Equal(t, []byte{223, 61}, input[61:63])
	require.False(t, d.SupportsInputReportSnapshot(ReportIDPro))
	d.HandleTransfer(ctx, 2, usbip.DirOut, selectReportCommand(ReportIDCommon))
	require.True(t, d.UpdateInputState(state))
	input = d.HandleTransfer(ctx, 1, usbip.DirIn, nil)
	require.NotEmpty(t, input)
	require.Equal(t, byte(0x05), input[0])
}

func TestGameCubeMotionFeaturesAndReportSnapshots(t *testing.T) {
	s := newNS2InputReportSequencer(*defaultMetaState(), time.Now().Add(-20*time.Millisecond))
	s.nativeReportID = ReportIDGameCube
	require.True(t, s.setActiveReportID(ReportIDGameCube))
	state := NewInputState()
	state.AccelZ, state.GyroX, state.GyroY = 4096, -1640, 820
	state.LeftTrigger, state.RightTrigger = 42, 223
	packet := make([]byte, InputReportSize)

	s.buildCurrentInto(state, packet)
	require.Zero(t, packet[15])
	s.enableFeatures(FeatureIMU)
	s.buildCurrentInto(state, packet)
	require.Equal(t, byte(40), packet[15])
	require.Equal(t, []byte{42, 223}, packet[13:15])
	require.Equal(t, uint32(0x0e), binary.LittleEndian.Uint32(packet[16:20])>>24)
	body := packet[20:56]
	require.Equal(t, uint32(3), readGameCubeBits(body, 0, 2))
	require.Equal(t, uint32(0x100000), readGameCubeBits(body, 5, 21))
	require.Equal(t, uint32(0x100000), readGameCubeBits(body, 26, 21))
	require.Equal(t, uint32(0x100000), readGameCubeBits(body, 47, 21))
	require.Equal(t, uint32(16384-1640), readGameCubeBits(body, 188, 14))
	require.Equal(t, uint32(820), readGameCubeBits(body, 202, 14))
	require.Equal(t, uint32(4096), readGameCubeBits(body, 258, 14))

	snapshot := make([]byte, InputReportSize)
	s.buildControlInto(state, ReportIDGameCube, snapshot)
	require.Equal(t, packet, snapshot)
	require.True(t, s.setActiveReportID(ReportIDCommon))
	s.buildCurrentInto(state, packet)
	require.NotZero(t, binary.LittleEndian.Uint32(packet[43:47]))
	require.Equal(t, int16(4096), int16(binary.LittleEndian.Uint16(packet[53:55])))
	require.Equal(t, int16(-1640), int16(binary.LittleEndian.Uint16(packet[55:57])))
	require.Equal(t, int16(820), int16(binary.LittleEndian.Uint16(packet[57:59])))
	require.Equal(t, []byte{42, 223}, packet[61:63])
	s.disableFeatures(FeatureIMU)
	s.buildCurrentInto(state, packet)
	require.Equal(t, make([]byte, 18), packet[43:61])
}

func TestGameCubeNativeMotionUsesSensorTimeAndResetsOnNeutral(t *testing.T) {
	var encoder gameCubeMotionEncoder
	state := InputState{AccelX: -16000, AccelZ: 4096, GyroZ: 1640}
	initial := *encoder.present(state, gameCubeTick*4090)
	second := *encoder.present(state, gameCubeTick*4102)
	third := *encoder.present(state, gameCubeTick*4114)
	require.Equal(t, uint32(6|12<<12|0x0e000000), binary.LittleEndian.Uint32(second[:4]))
	require.Equal(t, uint32(2), readGameCubeBits(third[4:], 0, 2))
	require.Equal(t, uint32(16384-4000), readGameCubeBits(third[4:], 230, 14))
	require.Equal(t, uint32(820), readGameCubeBits(third[4:], 136, 13))
	require.Equal(t, initial[4:12], second[4:12])
	// 100 degrees/s for 15 ms produces a 1.5 degree turn about Z.
	projectedZ := float64(readGameCubeBits(third[4:], 47, 21))/float64(1<<20) - 1
	require.InDelta(t, math.Tan(0.75*math.Pi/180), projectedZ, 1.0/(1<<20))
	require.Nil(t, encoder.present(InputState{}, gameCubeTick*4115))
	reconnected := encoder.present(state, gameCubeTick*4116)
	require.Equal(t, uint32(0x100000), readGameCubeBits(reconnected[4:], 47, 21))
}

func readGameCubeBits(body []byte, offset, width int) uint32 {
	var value uint32
	for i := 0; i < width; i++ {
		bit := offset + i
		value |= uint32(body[bit/8]>>(bit%8)&1) << i
	}
	return value
}

func TestGameCubeRumbleDutyAndStop(t *testing.T) {
	d, err := newDevice(nil, true)
	require.NoError(t, err)
	var feedback []OutputState
	defer d.SetOutputCallback(func(out OutputState) { feedback = append(feedback, out) })()
	for _, mode := range []byte{1, 0, 2, 3} {
		d.HandleTransfer(context.Background(), 1, usbip.DirOut, []byte{3, 0x5c, mode})
	}
	_, ok := d.HandleControl(hidClassRequestOut, hidSetReport, 0x0203, 0, 64, []byte{3, 0x50, 1})
	require.True(t, ok)
	require.Len(t, feedback, 4)
	for i, wanted := range []byte{255, 0, 0, 255} {
		require.Equal(t, byte(OutputFlagGameCubeRumble), feedback[i].Flags)
		require.Equal(t, wanted, feedback[i].LeftRumble[0])
	}
}

func TestGameCubeClientStreamAndUSBIPRoundTrip(t *testing.T) {
	server := viiperTesting.NewTestServer(t)
	defer server.UsbServer.Close()
	defer server.ApiServer.Close()
	routes := server.ApiServer.Router()
	routes.Register("bus/{id}/add", apihandler.BusDeviceAdd(server.UsbServer, server.ApiServer))
	routes.RegisterStream("bus/{busId}/{deviceid}", api.DeviceStreamHandler(server.UsbServer))
	require.NoError(t, server.ApiServer.Start())
	bus, err := virtualbus.NewWithBusID(1)
	require.NoError(t, err)
	defer bus.Close()
	require.NoError(t, server.UsbServer.AddBus(bus))

	client := viiperclient.New(server.ApiServer.Addr())
	stream, _, err := client.AddDeviceAndConnect(context.Background(), bus.BusID(), "ns2gamecube", nil)
	require.NoError(t, err)
	defer stream.Close()
	usbClient := viiperTesting.NewUsbIpClient(t, server.UsbServer.Addr())
	devices, err := usbClient.ListDevices()
	require.NoError(t, err)
	require.Len(t, devices, 1)
	imported, err := usbClient.AttachDevice(devices[0].BusID)
	require.NoError(t, err)
	defer imported.Conn.Close()

	config, err := controlIn(imported.Conn, controlSetup(0x0200, 0, 512))
	require.NoError(t, err)
	require.Len(t, config, 80)
	require.Equal(t, []byte{8, 0x0b, 0, 1, 3, 0, 0, 0}, config[9:17])
	require.Equal(t, []byte{8, 0x0b, 1, 1, 0xff, 0, 0, 0}, config[49:57])

	require.NoError(t, usbClient.Submit(imported.Conn, usbip.DirOut, 2, selectReportCommand(ReportIDGameCube), nil))
	require.NoError(t, usbClient.Submit(imported.Conn, usbip.DirOut, 2, enableReportsCommand(), nil))
	state := *NewInputState()
	state.Buttons = ButtonA | ButtonZR
	state.LeftTrigger, state.RightTrigger = 97, 232
	wire, err := state.MarshalBinary()
	require.NoError(t, err)
	wire = append(wire, state.LeftTrigger, state.RightTrigger)
	_, err = stream.Write(wire)
	require.NoError(t, err)
	expected := make([]byte, InputReportSize)
	state.buildGameCubeReportInto(expected, 0, FeatureButtons|FeatureSticks, *defaultMetaState(), nil)
	actual := pollInputIgnoringCounter(t, usbClient, imported.Conn, expected, viiperTesting.IntegrationTimeout)
	require.Len(t, actual, InputReportSize)
	require.Equal(t, expected[2:], actual[2:])

	require.NoError(t, usbClient.Submit(imported.Conn, usbip.DirOut, 2,
		featureCommand(subFeatureEnable, FeatureButtons|FeatureSticks|FeatureIMU), nil))
	state.AccelZ, state.GyroX = 4096, -1640
	wire, err = state.MarshalBinary()
	require.NoError(t, err)
	_, err = stream.Write(append(wire, state.LeftTrigger, state.RightTrigger))
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		report, readErr := usbClient.ReadInputReport(imported.Conn)
		return readErr == nil && len(report) == InputReportSize && report[15] == 40 &&
			readGameCubeBits(report[20:56], 188, 14) == 16384-1640 &&
			readGameCubeBits(report[20:56], 258, 14) == 4096 &&
			report[13] == 97 && report[14] == 232
	}, viiperTesting.IntegrationTimeout, time.Millisecond)

	require.NoError(t, usbClient.Submit(imported.Conn, usbip.DirOut, 1,
		[]byte{ReportIDGameCubeRumble, 0x51, 1}, nil))
	var feedback [OutputWireSize]byte
	require.NoError(t, stream.SetReadDeadline(time.Now().Add(viiperTesting.IntegrationTimeout)))
	_, err = io.ReadFull(stream, feedback[:])
	require.NoError(t, err)
	require.Equal(t, byte(OutputFlagGameCubeRumble), feedback[32])
	require.Equal(t, byte(255), feedback[0])
}
