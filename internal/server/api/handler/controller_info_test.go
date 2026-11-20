package handler

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	"github.com/Alia5/VIIPER/device"
	"github.com/Alia5/VIIPER/device/dualsense"
	"github.com/Alia5/VIIPER/device/dualshock4"
	"github.com/Alia5/VIIPER/device/ns2pro"
	"github.com/Alia5/VIIPER/device/xbox360"
	handlerTest "github.com/Alia5/VIIPER/internal/_testing"
	"github.com/Alia5/VIIPER/internal/server/api"
	serverusb "github.com/Alia5/VIIPER/internal/server/usb"
	"github.com/Alia5/VIIPER/usb"
	"github.com/Alia5/VIIPER/viiperclient"
	"github.com/Alia5/VIIPER/virtualbus"
	"github.com/stretchr/testify/require"
)

func TestControllerInfoIsReadOnlyAndMatchesEachRealDevice(t *testing.T) {
	cases := []struct {
		name    string
		create  func() (usb.Device, error)
		factory int
	}{
		{"ds4", func() (usb.Device, error) { return dualshock4.New(nil) }, 49},
		{"dualsense", func() (usb.Device, error) { return dualsense.New(nil) }, 64},
		{"edge", func() (usb.Device, error) { return dualsense.NewEdge(nil) }, 64},
		{"xbox360", func() (usb.Device, error) { return xbox360.New(nil) }, 0},
		{"ns2pro", func() (usb.Device, error) { return ns2pro.New(nil) }, 0},
	}
	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			const path = "bus/{busId}/{devId}/controller-info-v1"
			busID := uint32(60400 + i)
			dev, err := tc.create()
			require.NoError(t, err)
			before, err := json.Marshal(dev.GetDeviceSpecificArgs())
			require.NoError(t, err)
			var bus *virtualbus.VirtualBus
			addr, _, done := handlerTest.StartAPIServer(t, func(r *api.Router, s *serverusb.Server, _ *api.Server) {
				bus, err = virtualbus.NewWithBusID(busID)
				require.NoError(t, err)
				require.NoError(t, s.AddBus(bus))
				_, err = bus.Add(dev)
				require.NoError(t, err)
				r.Register(path, BusDeviceControllerInfoV1(s))
			})
			defer done()
			client := viiperclient.NewTransport(addr)
			params := map[string]string{"busId": strconv.FormatUint(uint64(busID), 10), "devId": "1"}
			for attempt := 0; attempt < 4; attempt++ {
				text, err := client.Do(path, "", params)
				require.NoError(t, err)
				var response struct {
					Version                       int
					BusID                         uint32
					DevID                         string
					VendorID, ProductID           uint16
					FactoryReport                 []byte
					Metadata                      map[string]any
					Manufacturer, Product, Serial string
				}
				require.NoError(t, json.Unmarshal([]byte(text), &response))
				require.Equal(t, 1, response.Version)
				require.Equal(t, busID, response.BusID)
				require.Equal(t, "1", response.DevID)
				descriptor := dev.GetDescriptor()
				require.Equal(t, descriptor.Device.IDVendor, response.VendorID)
				require.Equal(t, descriptor.Device.IDProduct, response.ProductID)
				for _, field := range []struct {
					index  byte
					actual string
				}{
					{descriptor.Device.IManufacturer, response.Manufacturer},
					{descriptor.Device.IProduct, response.Product},
					{descriptor.Device.ISerialNumber, response.Serial},
				} {
					if field.index == 0 {
						require.Empty(t, field.actual, "USB language IDs must never appear as identity strings")
					} else {
						require.Equal(t, descriptor.Strings[field.index], field.actual)
					}
				}
				require.Len(t, response.FactoryReport, tc.factory)
				if tc.factory != 0 {
					id := uint16(0x20)
					if tc.factory == 49 {
						id = 0xa3
					}
					raw, handled := dev.(usb.ControlDevice).HandleControl(0xa1, 1, 0x0300|id, 3, uint16(tc.factory), nil)
					require.True(t, handled)
					require.Equal(t, raw, response.FactoryReport)
				}
				require.NotContains(t, response.Metadata, "activationToken")
				require.NotContains(t, response.Metadata, "removalToken")
			}
			after, err := json.Marshal(dev.GetDeviceSpecificArgs())
			require.NoError(t, err)
			require.JSONEq(t, string(before), string(after))
			require.Len(t, bus.GetAllDeviceMetas(), 1)
			stillPresent, ok := bus.GetDeviceByID(1)
			require.True(t, ok)
			require.Same(t, dev, stillPresent)
			text, err := client.Do(path, `{"write":true}`, params)
			require.NoError(t, err)
			require.Contains(t, text, `"status":400`)
			params["devId"] = "2"
			text, err = client.Do(path, "", params)
			require.NoError(t, err)
			require.Contains(t, text, `"status":404`)
			params["devId"] = "invalid"
			text, err = client.Do(path, "", params)
			require.NoError(t, err)
			require.Contains(t, text, `"status":400`)
		})
	}
}

func TestControllerInfoKeepsSameModelDevicesSeparate(t *testing.T) {
	// Two DS4 outputs with different timestamps must never be selected by VID/PID.
	const path = "bus/{busId}/{devId}/controller-info-v1"
	addr, _, done := handlerTest.StartAPIServer(t, func(r *api.Router, s *serverusb.Server, _ *api.Server) {
		bus, err := virtualbus.NewWithBusID(60410)
		require.NoError(t, err)
		require.NoError(t, s.AddBus(bus))
		for _, year := range []string{"2000", "2031"} {
			dev, err := dualshock4.New(&device.CreateOptions{DeviceSpecific: `{"build_time":"` + year + `-01-01T11:34:00Z"}`})
			require.NoError(t, err)
			_, err = bus.Add(dev)
			require.NoError(t, err)
		}
		r.Register(path, BusDeviceControllerInfoV1(s))
	})
	defer done()
	client := viiperclient.NewTransport(addr)
	for i, year := range []string{"2000", "2031"} {
		text, err := client.Do(path, "", map[string]string{"busId": "60410", "devId": strconv.Itoa(i + 1)})
		require.NoError(t, err)
		var response struct{ FactoryReport []byte }
		require.NoError(t, json.Unmarshal([]byte(text), &response))
		require.Equal(t, year, string(response.FactoryReport[8:12]))
	}
}

func TestControllerInfoXboxOneExposesPublicVersionsWithoutAuthority(t *testing.T) {
	fixture := newXboxRemovalFactoryFixture(t, 60411)
	created := fixture.create(t, 450)
	response := &api.Response{}
	request := &api.Request{Params: map[string]string{"busId": "60411", "devId": created.DevID}}
	require.NoError(t, BusDeviceControllerInfoV1(fixture.server)(request, response, fixture.logger))
	var result struct {
		Details       map[string]string
		Metadata      map[string]any
		FactoryReport []byte
	}
	require.NoError(t, json.Unmarshal([]byte(response.JSON), &result))
	require.Equal(t, "1.2.3.4", result.Details["firmwareVersion"])
	require.Empty(t, result.FactoryReport)
	for _, secret := range []string{created.RemovalToken, "identityAuthorization", "streamToken", "retainedUSB"} {
		if secret != "" {
			require.False(t, strings.Contains(response.JSON, secret), "leaked authority: %s", secret)
		}
	}
}
