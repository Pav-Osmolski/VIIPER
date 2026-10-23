package dualsense

import "github.com/Alia5/VIIPER/device/batterystatus"

func (d *DualSense) UpdateBatteryStatusV1(payload string) error {
	status, err := batterystatus.Decode(payload)
	if err != nil {
		return err
	}
	d.metaMu.Lock()
	meta := *d.metaState
	meta.BatteryStatus = status.DualSense()
	d.metaState = &meta
	d.inputBattery.Store(uint32(meta.BatteryStatus))
	d.metaMu.Unlock()
	return nil
}
