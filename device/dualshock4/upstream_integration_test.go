package dualshock4

import (
	"context"
	"encoding/binary"
	"math"
	"testing"

	"github.com/Alia5/VIIPER/usbip"
)

func TestAccelerationMatchesAdvertisedCalibration(t *testing.T) {
	d, err := New(nil)
	if err != nil {
		t.Fatal(err)
	}
	calibration := d.featureReportCalibration()
	// Report ID, three gyro biases, six gyro extrema, two speed values, accel.
	for _, offset := range []int{23, 27, 31} {
		if int16(binary.LittleEndian.Uint16(calibration[offset:])) != 8192 ||
			int16(binary.LittleEndian.Uint16(calibration[offset+2:])) != -8192 {
			t.Fatalf("unexpected acceleration calibration at %d", offset)
		}
	}
	if AccelMS2ToRaw(StandardGravityMS2) != 8192 ||
		math.Abs(AccelRawToMS2(-8192)+StandardGravityMS2) > 1e-9 ||
		NewInputState().AccelZ != -8192 {
		t.Fatal("acceleration helpers/default disagree with one-g calibration")
	}
}

func TestPartialOutputCommandsPreserveOtherFieldsAndLegacyWire(t *testing.T) {
	for _, route := range []string{"interrupt", "control", "framed"} {
		t.Run(route, func(t *testing.T) {
			d, err := New(nil)
			if err != nil {
				t.Fatal(err)
			}
			var received OutputState
			if route == "framed" {
				_, ok := d.setFramedOutputCallbacks(func(s OutputState) bool { received = s; return true }, nil, nil)
				if !ok {
					t.Fatal("callback registration failed")
				}
			} else {
				d.SetOutputCallback(func(s OutputState) { received = s })
			}
			send := func(flags byte, fields ...byte) {
				report := append([]byte{ReportIDOutput, flags, 0, 0}, fields...)
				switch route {
				case "control":
					if _, ok := d.HandleControl(hidClassOUT, hidSetReport, uint16(reportTypeOutput)<<8|uint16(ReportIDOutput), InterfaceHID, uint16(len(report)), report); !ok {
						t.Fatal("control rejected")
					}
				case "framed":
					if handled, accepted := d.tryAdmitFramedOutput(report); !handled || !accepted {
						t.Fatal("framed rejected")
					}
				default:
					d.HandleTransfer(context.Background(), uint32(EndpointOut), usbip.DirOut, report)
				}
			}
			send(7, 10, 20, 30, 40, 50, 60, 70)
			send(1, 0, 0, 99, 99, 99, 99, 99)
			if received != (OutputState{0, 0, 30, 40, 50, 60, 70}) {
				t.Fatalf("rumble-only changed lights: %+v", received)
			}
			send(2, 99, 99, 1, 2, 3, 99, 99)
			send(4, 99, 99, 99, 99, 99, 4, 5)
			send(0, 99, 99, 99, 99, 99, 99, 99)
			want := OutputState{0, 0, 1, 2, 3, 4, 5}
			if received != want {
				t.Fatalf("invalid field update: %+v", received)
			}
			wire, err := received.MarshalBinary()
			if err != nil || len(wire) != 7 || wire[2] != 1 || wire[6] != 5 {
				t.Fatalf("legacy layout changed: %v %v", wire, err)
			}
			d.SetOutputCallback(func(s OutputState) { received = s })
			if received != want {
				t.Fatal("reconnect replay lost resolved state")
			}
		})
	}
}

func TestRejectedPartialOutputDoesNotCommit(t *testing.T) {
	d, _ := New(nil)
	accept := true
	d.setFramedOutputCallbacks(func(OutputState) bool { return accept }, nil, nil)
	initial := auditOutputReport(10, 20, 30)
	d.tryAdmitFramedOutput(initial)
	accept = false
	partial := auditOutputReport(0, 0, 99)
	partial[1] = 1
	if _, accepted := d.tryAdmitFramedOutput(partial); accepted {
		t.Fatal("rejected command accepted")
	}
	if d.outputState != (OutputState{10, 20, 30, 0, 0, 0, 0}) {
		t.Fatal("rejection changed cached state")
	}
	accept = true
	partial[1] = 2
	d.tryAdmitFramedOutput(partial)
	if d.outputState != (OutputState{10, 20, 99, 0, 0, 0, 0}) {
		t.Fatal("rejected rumble leaked into later partial update")
	}
}
