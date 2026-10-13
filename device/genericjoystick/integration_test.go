package genericjoystick_test

import (
	"context"
	"testing"

	viiperTesting "github.com/Alia5/VIIPER/_testing"
	"github.com/Alia5/VIIPER/device"
	"github.com/Alia5/VIIPER/device/genericjoystick"
	_ "github.com/Alia5/VIIPER/internal/devicecatalog"
	"github.com/Alia5/VIIPER/internal/server/api"
	"github.com/Alia5/VIIPER/internal/server/api/handler"
	"github.com/Alia5/VIIPER/viiperclient"
	"github.com/Alia5/VIIPER/virtualbus"
)

// Real localhost API and USB/IP sockets, but no Windows driver or native attach.
func TestLocalhostManagementAndUSBIPReports(t *testing.T) {
	s := viiperTesting.NewTestServer(t)
	defer s.UsbServer.Close()
	defer s.ApiServer.Close()
	r := s.ApiServer.Router()
	r.Register("bus/{id}/add", handler.BusDeviceAdd(s.UsbServer, s.ApiServer))
	r.RegisterStream("bus/{busId}/{deviceid}", api.DeviceStreamHandler(s.UsbServer))
	if err := s.ApiServer.Start(); err != nil {
		t.Fatal(err)
	}
	bus, err := virtualbus.NewWithBusID(1)
	if err != nil {
		t.Fatal(err)
	}
	defer bus.Close()
	s.UsbServer.AddBus(bus)
	client := viiperclient.New(s.ApiServer.Addr())
	stream, created, err := client.AddDeviceAndConnect(context.Background(), bus.BusID(), "genericjoystick", &device.CreateOptions{DeviceSpecific: `{"version":1,"experimental":true,"buttonCount":12}`})
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()
	if created.Type != "genericjoystick" || created.DeviceSpecific["version"] != float64(1) {
		t.Fatalf("wrong management receipt %+v", created)
	}
	usb := viiperTesting.NewUsbIpClient(t, s.UsbServer.Addr())
	devices, err := usb.ListDevices()
	if err != nil || len(devices) != 1 {
		t.Fatalf("enumeration: %+v %v", devices, err)
	}
	if devices[0].IDVendor != 0xFFFF || devices[0].IDProduct != 0xD10C {
		t.Fatal("unexpected prototype identity")
	}
	imported, err := usb.AttachDevice(devices[0].BusID)
	if err != nil {
		t.Fatal(err)
	}
	defer imported.Conn.Close()
	for _, state := range []genericjoystick.InputState{
		{Hat: 8}, {X: 32767, Y: -32767, Z: 123, Rx: 456, Ry: -789, Rz: 1000, Buttons: 1, Hat: 1}, {Hat: 8},
	} {
		if err := stream.WriteBinary(state); err != nil {
			t.Fatal(err)
		}
		expected := make([]byte, genericjoystick.ReportSize)
		state.BuildReportInto(expected)
		if _, err := usb.PollInputReport(imported.Conn, expected, viiperTesting.IntegrationTimeout); err != nil {
			t.Fatal(err)
		}
	}
}
