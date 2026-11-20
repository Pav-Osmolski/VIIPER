package device_test

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"testing"
	"time"

	"github.com/Alia5/VIIPER/device"
	"github.com/Alia5/VIIPER/device/dualsense"
	"github.com/Alia5/VIIPER/device/dualshock4"
)

type factoryReportDevice interface {
	HandleControl(uint8, uint8, uint16, uint16, uint16, []byte) ([]byte, bool)
}

// Read the actual USB GET_REPORT response using host-side field boundaries,
// independent of the report builders. DS4 has two 16-byte NUL-padded strings;
// DualSense/Edge instead have contiguous 11-byte date and 8-byte time fields.
func TestSonyFactoryReportHostDecoding(t *testing.T) {
	cases := []struct {
		name                                string
		create                              func(*device.CreateOptions) (factoryReportDevice, error)
		reportID                            byte
		length, dateEnd, timeStart, timeEnd int
		defaultTime                         time.Time
	}{
		{"DualShock4", func(o *device.CreateOptions) (factoryReportDevice, error) { return dualshock4.New(o) }, 0xa3, 49, 17, 17, 33, time.Date(2021, 9, 17, 11, 34, 0, 0, time.UTC)},
		{"DualSense", func(o *device.CreateOptions) (factoryReportDevice, error) { return dualsense.New(o) }, 0x20, 64, 12, 12, 20, time.Date(2025, 7, 4, 10, 10, 32, 0, time.UTC)},
		{"DualSenseEdge", func(o *device.CreateOptions) (factoryReportDevice, error) { return dualsense.NewEdge(o) }, 0x20, 64, 12, 12, 20, time.Date(2025, 7, 4, 10, 10, 32, 0, time.UTC)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, custom := range []string{"", "2000-02-29T00:01:02Z", "2031-12-31T11:34:00Z", "2099-01-01T23:59:58Z"} {
				t.Run("build="+custom, func(t *testing.T) {
					want := tc.defaultTime
					var options *device.CreateOptions
					if custom != "" {
						var err error
						want, err = time.Parse(time.RFC3339, custom)
						if err != nil {
							t.Fatal(err)
						}
						options = &device.CreateOptions{DeviceSpecific: fmt.Sprintf(`{"build_time":%q}`, custom)}
					}
					dev, err := tc.create(options)
					if err != nil {
						t.Fatal(err)
					}
					// Request ID is included in the response, as WebHID returns it.
					report, ok := dev.HandleControl(0xa1, 0x01, 0x0300|uint16(tc.reportID), 3, uint16(tc.length), nil)
					if !ok || len(report) != tc.length || report[0] != tc.reportID {
						t.Fatalf("unexpected report: handled=%t bytes=% x", ok, report)
					}
					dateField := report[1:tc.dateEnd]
					timeField := report[tc.timeStart:tc.timeEnd]
					if got := string(bytes.ReplaceAll(dateField, []byte{0}, nil)); got != want.Format("Jan 02 2006") {
						t.Errorf("host date = %q, want %q", got, want.Format("Jan 02 2006"))
					}
					if got := string(bytes.ReplaceAll(timeField, []byte{0}, nil)); got != want.Format("15:04:05") {
						t.Errorf("host time = %q, want %q", got, want.Format("15:04:05"))
					}
					if tc.reportID == 0xa3 {
						if !bytes.Equal(report[12:17], make([]byte, 5)) || !bytes.Equal(report[25:33], make([]byte, 8)) {
							t.Errorf("DS4 date/time padding is not zero: % x", report[1:33])
						}
						if binary.LittleEndian.Uint16(report[33:35]) != dualshock4.HardwareVersionMajor ||
							binary.LittleEndian.Uint16(report[35:37]) != dualshock4.HardwareVersionMinor {
							t.Error("date/time overlaps DS4 hardware version fields")
						}
					} else if binary.LittleEndian.Uint16(report[44:46]) != dualsense.FirmwareVersion {
						t.Error("date/time overlaps DualSense firmware version")
					}
					short, ok := dev.HandleControl(0xa1, 0x01, 0x0300|uint16(tc.reportID), 3, 20, nil)
					if !ok || !bytes.Equal(short, report[:20]) {
						t.Error("short host read changed the report prefix")
					}
				})
			}
		})
	}
}
