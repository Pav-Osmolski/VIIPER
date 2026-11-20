package dualsense

import (
	"strings"
	"testing"

	"github.com/Alia5/VIIPER/device"
	"github.com/Alia5/VIIPER/usb"
)

func TestShellColorCreationDefaultsAndExplicitValues(t *testing.T) {
	factories := []struct {
		name    string
		create  func(*device.CreateOptions) (usb.Device, error)
		handler bool
	}{
		{"direct-ds", func(o *device.CreateOptions) (usb.Device, error) { return New(o) }, false},
		{"direct-edge", func(o *device.CreateOptions) (usb.Device, error) { return NewEdge(o) }, false},
		{"ds", (&dshandler{}).CreateDevice, true},
		{"ds-events", (&dshandler{micInterfaceEvents: true}).CreateDevice, true},
		{"ds-raw", (&dshandler{micInterfaceEvents: true, physicalInputMetadata: true}).CreateDevice, true},
		{"ds-audio", (&dshandler{audioOnly: true}).CreateDevice, true},
		{"ds-audio-events", (&dshandler{audioOnly: true, micInterfaceEvents: true}).CreateDevice, true},
		{"ds-audio-raw", (&dshandler{audioOnly: true, micInterfaceEvents: true, physicalInputMetadata: true}).CreateDevice, true},
		{"ds-gamepad", (&dshandler{gamepadOnly: true}).CreateDevice, true},
		{"ds-gamepad-raw", (&dshandler{gamepadOnly: true, physicalInputMetadata: true}).CreateDevice, true},
		{"edge", (&dsedgehandler{}).CreateDevice, true},
		{"edge-events", (&dsedgehandler{micInterfaceEvents: true}).CreateDevice, true},
		{"edge-raw", (&dsedgehandler{micInterfaceEvents: true, physicalInputMetadata: true}).CreateDevice, true},
		{"edge-gamepad", (&dsedgehandler{gamepadOnly: true}).CreateDevice, true},
		{"edge-gamepad-raw", (&dsedgehandler{gamepadOnly: true, physicalInputMetadata: true}).CreateDevice, true},
	}
	cases := []struct{ name, payload, colour string }{
		{"nil", "", DefaultShellColor},
		{"omitted", `{}`, DefaultShellColor},
		{"unrelated-options", `{"battery_status":42}`, DefaultShellColor},
		{"empty", `{"shell_color":""}`, DefaultShellColor},
		{"white", `{"shell_color":"00"}`, "00"},
		{"special-edition", `{"shell_color":"Z1"}`, "Z1"},
	}
	for _, factory := range factories {
		for _, test := range cases {
			t.Run(factory.name+"/"+test.name, func(t *testing.T) {
				var options *device.CreateOptions
				if test.payload != "" {
					options = &device.CreateOptions{DeviceSpecific: test.payload}
				}
				created, err := factory.create(options)
				if err != nil {
					t.Fatal(err)
				}
				ds := created.(*DualSense)
				if factory.handler {
					t.Cleanup(func() { releaseRearConverterTestIdentity(ds) })
				}
				if got := ds.GetDeviceSpecificArgs()["shell_color"]; got != test.colour {
					t.Fatalf("shell colour = %v, want %q", got, test.colour)
				}
				if factory.handler {
					serial := ds.metaState.SerialNumber
					if len(serial) < 6 || serial[4:6] != strings.ToUpper(test.colour) {
						t.Fatalf("serial %q did not encode shell colour %q", serial, test.colour)
					}
				}
			})
		}
	}
}

func TestShellColorPartialMetadataUpdatesPreserveIdentity(t *testing.T) {
	for _, edge := range []bool{false, true} {
		t.Run(map[bool]string{false: "ds", true: "edge"}[edge], func(t *testing.T) {
			ds, err := new(&device.CreateOptions{DeviceSpecific: `{"shell_color":"Z1","serial_number":"0011Z1223344","mac_address":"AA:BB:CC:DD:EE:01"}`}, edge)
			if err != nil {
				t.Fatal(err)
			}
			serial, mac := ds.metaState.SerialNumber, ds.metaState.MACAddress
			for _, update := range []map[string]any{{"battery_status": 42}, {"shell_color": ""}, {"shell_color": nil}, {}} {
				ds.metaState.UpdateFromMap(update)
				if ds.metaState.ShellColor != "Z1" {
					t.Fatalf("partial update %v erased shell colour", update)
				}
				if ds.metaState.SerialNumber != serial || ds.metaState.MACAddress != mac {
					t.Fatal("partial update changed identity")
				}
			}
			ds.metaState.UpdateFromMap(map[string]any{"shell_color": "00"})
			if ds.metaState.ShellColor != "00" {
				t.Fatal("explicit white colour was not applied")
			}
			if ds.metaState.SerialNumber != serial || ds.metaState.MACAddress != mac {
				t.Fatal("colour update rewrote an existing identity")
			}
			if ds.GetDeviceSpecificArgs()["shell_color"] != "00" {
				t.Fatal("updated colour was not exposed")
			}
		})
	}
}
