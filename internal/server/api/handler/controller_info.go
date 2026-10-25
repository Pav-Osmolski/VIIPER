package handler

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"

	"github.com/Alia5/VIIPER/device/dualsense"
	"github.com/Alia5/VIIPER/device/dualshock4"
	"github.com/Alia5/VIIPER/internal/server/api"
	apierror "github.com/Alia5/VIIPER/internal/server/api/error"
	"github.com/Alia5/VIIPER/internal/server/usb"
)

// BusDeviceControllerInfoV1 is a read-only snapshot of the selected virtual
// device. No caller-supplied USB request, write, stream or device reopen is allowed.
func BusDeviceControllerInfoV1(s *usb.Server) api.HandlerFunc {
	return func(req *api.Request, res *api.Response, _ *slog.Logger) error {
		if req.Payload != "" {
			return apierror.ErrBadRequest("controller info has no payload")
		}
		busID, err := strconv.ParseUint(req.Params["busId"], 10, 32)
		if err != nil {
			return apierror.ErrBadRequest("invalid busId")
		}
		devID, err := strconv.ParseUint(req.Params["devId"], 10, 32)
		if err != nil {
			return apierror.ErrBadRequest("invalid devId")
		}
		bus := s.GetBus(uint32(busID))
		if bus == nil {
			return apierror.ErrNotFound("controller bus not found")
		}
		for _, entry := range bus.GetAllDeviceMetas() {
			if entry.Meta.DevID != uint32(devID) {
				continue
			}
			desc, err := s.SnapshotDeviceDescriptor(entry)
			if err != nil {
				return apierror.ErrConflict("controller changed during snapshot")
			}
			// Expose only diagnostic metadata, never registration/activation
			// authority, application paths or arbitrary device-specific options.
			metadata := map[string]any{}
			args := entry.Dev.GetDeviceSpecificArgs()
			for _, key := range []string{"serial_number", "mac_address", "board", "build_time", "battery_status", "battery_voltage", "temperature_celsius", "shell_color", "subType", "battery_level", "charging", "external_power", "battery_volts"} {
				if value, ok := args[key]; ok {
					metadata[key] = value
				}
			}
			details := map[string]string{}
			if provider, ok := entry.Dev.(interface{ ControllerInfoFieldsV1() map[string]string }); ok {
				fields := provider.ControllerInfoFieldsV1()
				for _, key := range []string{"firmwareVersion", "hardwareVersion", "deviceInfo"} {
					if value := fields[key]; value != "" {
						details[key] = value
					}
				}
			}
			var factory []byte
			switch d := entry.Dev.(type) {
			case *dualshock4.DualShock4:
				factory, _ = d.HandleControl(0xa1, 1, 0x03a3, 3, 49, nil)
			case *dualsense.DualSense:
				factory, _ = d.HandleControl(0xa1, 1, 0x0320, 3, 64, nil)
			}
			response := struct {
				Version       int               `json:"version"`
				BusID         uint32            `json:"busId"`
				DevID         string            `json:"devId"`
				Type          string            `json:"type"`
				VendorID      uint16            `json:"vendorId"`
				ProductID     uint16            `json:"productId"`
				USBRevision   uint16            `json:"usbRevision"`
				Manufacturer  string            `json:"manufacturer"`
				Product       string            `json:"product"`
				Serial        string            `json:"serial"`
				Metadata      map[string]any    `json:"metadata"`
				Details       map[string]string `json:"details,omitempty"`
				FactoryReport []byte            `json:"factoryReport,omitempty"`
			}{1, uint32(busID), strconv.FormatUint(devID, 10), inferDeviceType(entry.Dev),
				desc.Device.IDVendor, desc.Device.IDProduct, desc.Device.BcdDevice,
				desc.Strings[desc.Device.IManufacturer], desc.Strings[desc.Device.IProduct], desc.Strings[desc.Device.ISerialNumber], metadata, details, factory}
			payload, err := json.Marshal(response)
			if err != nil {
				return apierror.ErrInternal("cannot encode controller info")
			}
			res.JSON = string(payload)
			return nil
		}
		return apierror.ErrNotFound(fmt.Sprintf("controller %d not found", devID))
	}
}
