package ns2pro

import "github.com/Alia5/VIIPER/internal/batterystatus"

func (d *NS2Pro) UpdateBatteryStatusV1(payload string) error {
	status, err := batterystatus.Decode(payload)
	if err != nil {
		return err
	}
	d.metaUpdateMu.Lock()
	defer d.metaUpdateMu.Unlock()
	d.stateMu.Lock()
	meta := *d.metaState
	meta.BatteryLevel = uint8(uint16(status.Percent) * 9 / 100)
	meta.Charging = status.Charging
	meta.ExternalPower = status.ExternalPower
	// Voltage remains the device's independent metadata: never invent a reading.
	d.metaState = &meta
	d.stateMu.Unlock()
	d.inputReports.setMeta(meta)
	return nil
}
