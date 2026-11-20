package dualshock4

import "github.com/Alia5/VIIPER/internal/batterystatus"

func (d *DualShock4) UpdateBatteryStatusV1(payload string) error {
	status, err := batterystatus.Decode(payload)
	if err != nil {
		return err
	}
	d.mtx.Lock()
	defer d.mtx.Unlock()
	meta := *d.metaState
	meta.BatteryStatus = status.DualShock4()
	d.metaState = &meta
	return nil
}
