package ns2pro

import (
	"context"
	"testing"

	"github.com/Alia5/VIIPER/internal/server/api"
	"github.com/Alia5/VIIPER/usbip"
	"github.com/stretchr/testify/require"
)

func TestGameCubeDescriptorAndReports(t *testing.T) {
	registration := api.GetRegistration("ns2gamecube")
	require.NotNil(t, registration)
	device, err := registration.CreateDevice(nil)
	require.NoError(t, err)
	d := device.(*NS2Pro)
	desc := d.GetDescriptor()
	require.Equal(t, uint16(0x057e), desc.Device.IDVendor)
	require.Equal(t, uint16(0x2073), desc.Device.IDProduct)
	require.Equal(t, uint16(0x0101), desc.Device.BcdDevice)
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
