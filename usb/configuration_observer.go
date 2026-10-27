package usb

// ConfigurationObserver receives only accepted SET_CONFIGURATION values.
// It can expose readiness without guessing from endpoint resets or snapshots.
type ConfigurationObserver interface {
	USBConfigurationChanged(value uint8)
}
