package handler_test

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/Alia5/VIIPER/device"
	"github.com/Alia5/VIIPER/device/dualshock4"
	"github.com/Alia5/VIIPER/usbip"
	"github.com/stretchr/testify/require"
	"strconv"
	"testing"

	"github.com/Alia5/VIIPER/device/dualsense"
	handlerTest "github.com/Alia5/VIIPER/internal/_testing"
	"github.com/Alia5/VIIPER/internal/server/api"
	"github.com/Alia5/VIIPER/internal/server/api/handler"
	"github.com/Alia5/VIIPER/internal/server/usb"
	"github.com/Alia5/VIIPER/viiperclient"
	"github.com/Alia5/VIIPER/virtualbus"
)

func TestBusDeviceMicrophoneInterfaceStatusUsesNarrowDeviceLookup(t *testing.T) {
	const busID = 60321
	var controller *dualsense.DualSense
	addr, _, done := handlerTest.StartAPIServer(t,
		func(r *api.Router, server *usb.Server, _ *api.Server) {
			bus, err := virtualbus.NewWithBusID(busID)
			if err != nil {
				t.Fatalf("create bus: %v", err)
			}
			if err := server.AddBus(bus); err != nil {
				t.Fatalf("add bus: %v", err)
			}
			controller, err = dualsense.New(nil)
			if err != nil {
				t.Fatalf("create DualSense: %v", err)
			}
			if _, err := bus.Add(controller); err != nil {
				t.Fatalf("add DualSense: %v", err)
			}
			r.Register("bus/{busId}/{devId}/microphone-interface",
				handler.BusDeviceMicrophoneInterfaceStatus(server))
		})
	defer done()

	client := viiperclient.NewTransport(addr)
	readStatus := func() map[string]any {
		t.Helper()
		line, err := client.Do("bus/{busId}/{devId}/microphone-interface", nil,
			map[string]string{"busId": "60321", "devId": "1"})
		if err != nil {
			t.Fatalf("status request: %v", err)
		}
		var status map[string]any
		if err := json.Unmarshal([]byte(line), &status); err != nil {
			t.Fatalf("decode status: %v", err)
		}
		return status
	}

	if active, ok := readStatus()["active"].(bool); !ok || active {
		t.Fatalf("initial microphone status was not inactive")
	}
	controller.SetInterfaceAltSetting(dualsense.InterfaceMicrophone, 1)
	status := readStatus()
	if active, ok := status["active"].(bool); !ok || !active {
		t.Fatalf("active alternate setting was not visible: %#v", status)
	}
	if _, broadMetadataLeak := status["serialNumber"]; broadMetadataLeak {
		t.Fatalf("narrow endpoint returned broad metadata: %#v", status)
	}
}

// Reproduce issue #80 through the same narrow API used by DS4Windows,
// including its Xbox-output audio-only companion alias.
func TestDualShock4MicrophoneStatusThroughActualRegisteredAliases(t *testing.T) {
	for i, alias := range []string{"dualshock4", "dualshock4audioduplexv3", "dualshock4audioonlyduplexv3"} {
		t.Run(alias, func(t *testing.T) {
			busID := uint32(60330 + i)
			registration := api.GetRegistration(alias)
			require.NotNil(t, registration)
			dev, err := registration.CreateDevice(&device.CreateOptions{DeviceSpecific: fmt.Sprintf(`{"serial_number":"%016x"}`, 80000+i)})
			require.NoError(t, err)
			controller := dev.(*dualshock4.DualShock4)
			addr, _, done := handlerTest.StartAPIServer(t, func(r *api.Router, server *usb.Server, _ *api.Server) {
				bus, err := virtualbus.NewWithBusID(busID)
				require.NoError(t, err)
				require.NoError(t, server.AddBus(bus))
				_, err = bus.Add(controller)
				require.NoError(t, err)
				other, err := dualshock4.New(nil)
				require.NoError(t, err)
				other.SetInterfaceAltSetting(dualshock4.InterfaceMicrophone, 1)
				_, err = bus.Add(other)
				require.NoError(t, err)
				r.Register("bus/{busId}/{devId}/microphone-interface", handler.BusDeviceMicrophoneInterfaceStatus(server))
			})
			defer done()
			client := viiperclient.NewTransport(addr)
			read := func() map[string]any {
				t.Helper()
				line, err := client.Do("bus/{busId}/{devId}/microphone-interface", nil, map[string]string{"busId": strconv.FormatUint(uint64(busID), 10), "devId": "1"})
				require.NoError(t, err)
				var status map[string]any
				require.NoError(t, json.Unmarshal([]byte(line), &status))
				require.NotContains(t, status, "status", line)
				return status
			}
			require.Equal(t, false, read()["active"], "another active microphone must not affect this device")
			controller.SetInterfaceAltSetting(dualshock4.InterfaceMicrophone, 1)
			require.Equal(t, true, read()["active"])
			pcm := make([]byte, dualshock4.USBMicrophoneClientFrameSize)
			for j := range pcm {
				pcm[j] = 0x1a
			}
			for j := 0; j < 6; j++ {
				controller.QueueMicrophonePCMFrame(pcm)
			}
			before := read()
			require.Equal(t, float64(6*len(pcm)), before["queuedBytes"])
			require.Equal(t, before, read(), "status reads must not drain PCM or change counters")
			require.NotContains(t, before, "serialNumber")
			require.NotContains(t, before, "batteryStatus")
			packet := controller.HandleTransfer(context.Background(), uint32(dualshock4.EndpointMicrophoneIn), usbip.DirIn, nil)
			require.NotEmpty(t, packet)
			require.Contains(t, packet, byte(0x1a), "active capture must deliver queued nonzero PCM")
			after := read()
			require.Greater(t, after["packetsRead"].(float64), before["packetsRead"].(float64))
			controller.SetInterfaceAltSetting(dualshock4.InterfaceMicrophone, 0)
			stopped := read()
			require.Equal(t, false, stopped["active"])
			require.Equal(t, float64(0), stopped["queuedBytes"])
		})
	}
}
