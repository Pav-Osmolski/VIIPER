package ns2pro

import "github.com/Alia5/VIIPER/usb"

const (
	GameCubePID              = 0x2073
	ReportIDGameCube         = 0x0A
	ReportIDGameCubeRumble   = 0x03
	OutputFlagGameCubeRumble = 0x04
)

func makeGameCubeDescriptor() usb.Descriptor {
	d := makeDescriptor(ReportIDGameCube, ReportIDGameCubeRumble)
	d.Device.IDProduct = GameCubePID
	d.Device.BcdDevice = 0x0101
	d.Associations = []usb.InterfaceAssociationDescriptor{
		{BFirstInterface: 0, BInterfaceCount: 1, BFunctionClass: 0x03},
		{BFirstInterface: 1, BInterfaceCount: 1, BFunctionClass: 0xFF},
	}
	d.Interfaces[0].Endpoints[0].BInterval = 4
	for _, index := range []uint8{2, 4, 5, 6} {
		d.Strings[index] = "Nintendo GameCube Controller"
	}
	return d
}

func (d *NS2Pro) outputReportID() uint8 {
	if d.gameCube {
		return ReportIDGameCubeRumble
	}
	return ReportIDOutput
}

var gameCubeNativeButtons = []buttonReportBit{
	{ButtonA, 0, 0x01}, {ButtonB, 0, 0x02}, {ButtonX, 0, 0x04}, {ButtonY, 0, 0x08},
	{ButtonZR, 0, 0x10}, {ButtonR, 0, 0x20}, {ButtonPlus, 0, 0x40},
	{ButtonDown, 1, 0x01}, {ButtonRight, 1, 0x02}, {ButtonLeft, 1, 0x04}, {ButtonUp, 1, 0x08},
	{ButtonZL, 1, 0x10}, {ButtonL, 1, 0x20},
	{ButtonHome, 2, 0x01}, {ButtonCapture, 2, 0x02}, {ButtonC, 2, 0x10},
}

var gameCubeCommonButtons = []buttonReportBit{
	{ButtonX, 0, 0x01}, {ButtonY, 0, 0x02}, {ButtonA, 0, 0x04}, {ButtonB, 0, 0x08},
	{ButtonZR, 0, 0x40}, {ButtonR, 0, 0x80},
	{ButtonPlus, 1, 0x02}, {ButtonHome, 1, 0x10}, {ButtonCapture, 1, 0x20}, {ButtonC, 1, 0x40},
	{ButtonDown, 2, 0x01}, {ButtonUp, 2, 0x02}, {ButtonRight, 2, 0x04}, {ButtonLeft, 2, 0x08},
	{ButtonZL, 2, 0x40}, {ButtonL, 2, 0x80},
}

func (s *ns2InputReportSequencer) buildCommonInto(state *InputState, dst []byte,
	motionTimestamp uint32, features uint8, meta MetaState) int {
	if s.nativeReportID != ReportIDGameCube {
		return state.buildCommonReportInto(dst, s.reportCounter32, motionTimestamp, features, meta)
	}
	size := state.buildCommonReportInto(dst, s.reportCounter32, 0, features&^FeatureIMU, meta)
	clear(dst[5:9])
	encodeButtonMap(state.Buttons, gameCubeCommonButtons, dst[5:9])
	dst[61], dst[62] = state.LeftTrigger, state.RightTrigger
	return size
}

func (s InputState) buildGameCubeReportInto(dst []byte, counter, features uint8, meta MetaState) int {
	b := dst[:InputReportSize]
	clear(b)
	b[0], b[1], b[2] = ReportIDGameCube, counter, powerInfo(meta)
	encodeButtonMap(s.Buttons, gameCubeNativeButtons, b[3:6])
	packStick12(b[6:9], s.LX, s.LY)
	packStick12(b[9:12], s.RX, s.RY)
	b[12] = 0x30
	if features&FeatureRumble != 0 {
		b[12] |= 0x08
	}
	b[13], b[14] = s.LeftTrigger, s.RightTrigger
	return len(b)
}

func (d *NS2Pro) handleGameCubeRumble(report []byte) {
	if len(report) < 3 || report[0] != ReportIDGameCubeRumble || report[1]&0xF0 != 0x50 || report[2] > 2 {
		return
	}
	feedback := OutputState{Flags: OutputFlagGameCubeRumble}
	if report[2] == 1 {
		feedback.LeftRumble[0] = 255
	}
	d.emitOutput(feedback)
}
