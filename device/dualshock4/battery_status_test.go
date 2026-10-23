package dualshock4

import "testing"

func TestBatteryStatusUpdatesReportWithoutChangingIdentity(t *testing.T) {
	d, err := New(nil)
	if err != nil {
		t.Fatal(err)
	}
	before := *d.metaState
	for _, tc := range []struct {
		payload string
		want    byte
	}{
		{`{"version":1,"percent":40,"charging":false,"externalPower":false,"complete":false}`, 4},
		{`{"version":1,"percent":0,"charging":false,"externalPower":false,"complete":false}`, 0},
		{`{"version":1,"percent":100,"charging":false,"externalPower":true,"complete":true}`, 0x1b},
	} {
		if err := d.UpdateBatteryStatusV1(tc.payload); err != nil {
			t.Fatal(err)
		}
		if report := d.buildUSBInputReport(NewInputState(), d.metaState); report[30] != tc.want {
			t.Fatalf("HID battery=%x", report[30])
		}
		if d.metaState.BatteryStatus != tc.want {
			t.Fatalf("battery %x", d.metaState.BatteryStatus)
		}
		if d.metaState.SerialNumber != before.SerialNumber || !d.metaState.BuildTime.Equal(before.BuildTime) {
			t.Fatal("identity mutated")
		}
	}
	if err := d.UpdateBatteryStatusV1(`{}`); err == nil {
		t.Fatal("invalid status accepted")
	}
	if d.metaState.BatteryStatus != 0x1b {
		t.Fatal("invalid snapshot mutated metadata")
	}
}
