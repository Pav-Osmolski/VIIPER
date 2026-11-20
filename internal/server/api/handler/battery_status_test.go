package handler_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Alia5/VIIPER/device/dualshock4"
	handlerTest "github.com/Alia5/VIIPER/internal/_testing"
	"github.com/Alia5/VIIPER/internal/server/api"
	"github.com/Alia5/VIIPER/internal/server/api/handler"
	"github.com/Alia5/VIIPER/internal/server/usb"
	"github.com/Alia5/VIIPER/viiperclient"
	"github.com/Alia5/VIIPER/virtualbus"
)

func TestBusDeviceBatteryStatusV1UpdatesExactDevice(t *testing.T) {
	const busID = 60323
	var controller *dualshock4.DualShock4
	addr, _, done := handlerTest.StartAPIServer(t,
		func(r *api.Router, server *usb.Server, _ *api.Server) {
			bus, err := virtualbus.NewWithBusID(busID)
			if err != nil {
				t.Fatalf("create bus: %v", err)
			}
			if err := server.AddBus(bus); err != nil {
				t.Fatalf("add bus: %v", err)
			}
			controller, err = dualshock4.New(nil)
			if err != nil {
				t.Fatalf("create ns2pro: %v", err)
			}
			if _, err := bus.Add(controller); err != nil {
				t.Fatalf("add ns2pro: %v", err)
			}
			r.Register("bus/{busId}/{devId}/battery-status-v1",
				handler.BusDeviceBatteryStatusV1(server))
		})
	defer done()

	client := viiperclient.NewTransport(addr)
	payload := `{"version":1,"percent":50,"charging":false,"externalPower":false,"complete":false}`
	line, err := client.Do("bus/{busId}/{devId}/battery-status-v1", payload,
		map[string]string{"busId": "60323", "devId": "1"})
	if err != nil {
		t.Fatalf("status request: %v", err)
	}
	var response map[string]any
	if err := json.Unmarshal([]byte(line), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response["version"] != float64(1) || response["updated"] != true {
		t.Fatalf("unexpected response: %#v", response)
	}
	meta := controller.GetDeviceSpecificArgs()
	if meta["battery_status"] != float64(5) {
		t.Fatalf("status did not reach exact device: %#v", meta)
	}

	bad := strings.Replace(payload, `"version":1`, `"version":2`, 1)
	line, err = client.Do("bus/{busId}/{devId}/battery-status-v1", bad,
		map[string]string{"busId": "60323", "devId": "1"})
	if err != nil {
		t.Fatalf("invalid status transport: %v", err)
	}
	if !strings.Contains(line, `"status":400`) {
		t.Fatalf("invalid status was not rejected: %s", line)
	}
	meta = controller.GetDeviceSpecificArgs()
	if meta["battery_status"] != float64(5) {
		t.Fatalf("rejected status mutated device: %#v", meta)
	}
}
