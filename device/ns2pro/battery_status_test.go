package ns2pro

import "testing"

func TestGenericBatteryPreservesMeasuredVoltageAndIdentity(t *testing.T) {
	d, err := New(nil)
	if err != nil {
		t.Fatal(err)
	}
	before := *d.metaState
	if err := d.UpdateBatteryStatusV1(`{"version":1,"percent":50,"charging":true,"externalPower":true,"complete":false}`); err != nil {
		t.Fatal(err)
	}
	if d.metaState.BatteryLevel != 4 || !d.metaState.Charging || !d.metaState.ExternalPower {
		t.Fatal("power status did not update")
	}
	if d.metaState.BatteryVolts != before.BatteryVolts || d.metaState.SerialNumber != before.SerialNumber {
		t.Fatal("unrelated metadata changed")
	}
	if err := d.UpdateBatteryStatusV1(`{"version":1,"percent":0,"charging":false,"externalPower":false,"complete":false}`); err != nil {
		t.Fatal(err)
	}
	if d.metaState.BatteryLevel != 0 || d.metaState.Charging || d.metaState.ExternalPower {
		t.Fatal("zero/false status did not clear")
	}
}
