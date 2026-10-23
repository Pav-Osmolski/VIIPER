package dualsense

import "testing"

func TestBatteryStatusPreservesFactoryIdentity(t *testing.T) {
	d, err := New(nil)
	if err != nil {
		t.Fatal(err)
	}
	before := *d.metaState
	for _, tc := range []struct {
		payload string
		want    byte
	}{
		{`{"version":1,"percent":0,"charging":false,"externalPower":false,"complete":false}`, 0},
		{`{"version":1,"percent":50,"charging":true,"externalPower":true,"complete":false}`, 0x15},
		{`{"version":1,"percent":100,"charging":false,"externalPower":true,"complete":true}`, 0x2a},
	} {
		if err := d.UpdateBatteryStatusV1(tc.payload); err != nil {
			t.Fatal(err)
		}
		if byte(d.inputBattery.Load()) != tc.want {
			t.Fatal("stream battery did not update")
		}
		if d.metaState.SerialNumber != before.SerialNumber || !d.metaState.BuildTime.Equal(before.BuildTime) {
			t.Fatal("factory identity changed")
		}
		report := d.buildUSBInputReport(NewInputState(), d.metaState)
		if report[53] != tc.want {
			t.Fatalf("HID battery=%x, want %x", report[53], tc.want)
		}
	}
}
