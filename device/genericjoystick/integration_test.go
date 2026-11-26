package genericjoystick_test

import (
	"context"
	"fmt"
	"strconv"
	"testing"

	viiperTesting "github.com/Alia5/VIIPER/_testing"
	"github.com/Alia5/VIIPER/device"
	"github.com/Alia5/VIIPER/device/genericjoystick"
	_ "github.com/Alia5/VIIPER/internal/devicecatalog"
	"github.com/Alia5/VIIPER/internal/server/api"
	"github.com/Alia5/VIIPER/internal/server/api/handler"
	"github.com/Alia5/VIIPER/usbip"
	"github.com/Alia5/VIIPER/viiperclient"
	"github.com/Alia5/VIIPER/virtualbus"
)

// The test servers use ephemeral localhost ports and never attach a Windows driver.
func joystickServers(t *testing.T) (*virtualbus.VirtualBus, *viiperclient.Client, *viiperTesting.TestUsbIpClient) {
	t.Helper()
	s := viiperTesting.NewTestServer(t)
	t.Cleanup(func() { s.UsbServer.Close() })
	t.Cleanup(func() { s.ApiServer.Close() })
	r := s.ApiServer.Router()
	r.Register("bus/{id}/add", handler.BusDeviceAdd(s.UsbServer, s.ApiServer))
	r.Register("bus/{id}/remove", handler.BusDeviceRemove(s.UsbServer))
	r.RegisterStream("bus/{busId}/{deviceid}", api.DeviceStreamHandler(s.UsbServer))
	if err := s.ApiServer.Start(); err != nil {
		t.Fatal(err)
	}
	bus, err := virtualbus.NewWithBusID(1)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = bus.Close() })
	s.UsbServer.AddBus(bus)
	return bus, viiperclient.New(s.ApiServer.Addr()), viiperTesting.NewUsbIpClient(t, s.UsbServer.Addr())
}

func TestLocalhostManagementAndUSBIPReports(t *testing.T) {
	bus, client, usb := joystickServers(t)
	created, err := client.DeviceAddCtx(context.Background(), bus.BusID(), "genericjoystick", &device.CreateOptions{DeviceSpecific: `{"version":1,"registered":true,"buttonCount":16}`})
	if err != nil {
		t.Fatal(err)
	}
	if created.Type != "genericjoystick" || created.DeviceSpecific["version"] != float64(1) || created.DeviceSpecific["experimental"] != false ||
		created.DeviceSpecific["vendorId"] != float64(0x1209) || created.DeviceSpecific["productId"] != float64(0xD10C) {
		t.Fatalf("wrong management receipt %+v", created)
	}
	devices, err := usb.ListDevices()
	if err != nil || len(devices) != 1 {
		t.Fatalf("enumeration: %+v %v", devices, err)
	}
	if devices[0].IDVendor != 0x1209 || devices[0].IDProduct != 0xD10C {
		t.Fatal("unexpected registered USB/IP identity")
	}
	imported, err := usb.AttachDevice(devices[0].BusID)
	if err != nil {
		t.Fatal(err)
	}
	defer imported.Conn.Close()
	// Reproduce Windows HID startup, which the old lab omitted: configure the
	// USB device before acquiring a producer. Configuration fences old leases.
	setup := [8]byte{0, 9, 1, 0, 0, 0, 0, 0} // SET_CONFIGURATION(1)
	if err := usb.Submit(imported.Conn, usbip.DirOut, 0, nil, &setup); err != nil {
		t.Fatal(err)
	}
	neutral := make([]byte, genericjoystick.ReportSize)
	genericjoystick.NewInputState().BuildReportInto(neutral)
	if _, err := usb.PollInputReport(imported.Conn, neutral, viiperTesting.IntegrationTimeout); err != nil {
		t.Fatal(err)
	}
	stream, err := client.OpenStream(context.Background(), bus.BusID(), created.DevID)
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()
	for _, state := range []genericjoystick.InputState{
		{Hat: 8}, {X: 32767, Y: -32767, Z: 123, Rx: 456, Ry: -789, Rz: 1000, Buttons: 0x8001, Hat: 1}, {Hat: 8},
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

func TestRegisteredJoysticksKeepSerialsAndReportsThroughRecreation(t *testing.T) {
	bus, client, usb := joystickServers(t)
	ctx := context.Background()
	const a = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	const b = "BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB"

	type joystick struct {
		id       string
		serial   string
		imported *viiperTesting.ImportResult
		stream   *viiperclient.DeviceStream
	}
	checkReport := func(device joystick, state genericjoystick.InputState) {
		t.Helper()
		expected := make([]byte, genericjoystick.ReportSize)
		state.BuildReportInto(expected)
		if _, err := usb.PollInputReport(device.imported.Conn, expected, viiperTesting.IntegrationTimeout); err != nil {
			t.Fatalf("%s: %v", device.serial, err)
		}
	}
	open := func(identity string) joystick {
		t.Helper()
		options := fmt.Sprintf(`{"version":1,"registered":true,"buttonCount":16,"instanceId":%q}`, identity)
		receipt, err := client.DeviceAddCtx(ctx, bus.BusID(), "genericjoystick", &device.CreateOptions{DeviceSpecific: options})
		if err != nil {
			t.Fatal(err)
		}
		if receipt.DeviceSpecific["instanceId"] != identity || receipt.DeviceSpecific["experimental"] != false {
			t.Fatalf("unexpected receipt: %+v", receipt.DeviceSpecific)
		}
		number, err := strconv.ParseUint(receipt.DevID, 10, 32)
		if err != nil {
			t.Fatal(err)
		}
		registered, found := bus.GetDeviceByID(uint32(number))
		if !found {
			t.Fatalf("registration %s is missing", receipt.DevID)
		}
		serial := registered.GetDescriptor().Strings[3]
		imported, err := usb.AttachDevice(fmt.Sprintf("%d-%s", bus.BusID(), receipt.DevID))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = imported.Conn.Close() })
		if imported.Exported.IDVendor != 0x1209 || imported.Exported.IDProduct != 0xD10C {
			t.Fatalf("unexpected USB/IP identity for %s", serial)
		}
		setup := [8]byte{0, 9, 1, 0, 0, 0, 0, 0} // SET_CONFIGURATION(1)
		if err := usb.Submit(imported.Conn, usbip.DirOut, 0, nil, &setup); err != nil {
			t.Fatal(err)
		}
		stream, err := client.OpenStream(ctx, bus.BusID(), receipt.DevID)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = stream.Close() })
		opened := joystick{receipt.DevID, serial, imported, stream}
		checkReport(opened, genericjoystick.InputState{Hat: 8})
		return opened
	}
	publish := func(device joystick, state genericjoystick.InputState) {
		t.Helper()
		if err := device.stream.WriteBinary(state); err != nil {
			t.Fatal(err)
		}
		checkReport(device, state)
	}
	remove := func(device joystick) {
		t.Helper()
		if _, err := client.DeviceRemoveCtx(ctx, bus.BusID(), device.id); err != nil {
			t.Fatal(err)
		}
		_ = device.stream.Close()
		_ = device.imported.Conn.Close()
	}

	first, second := open(a), open(b)
	if first.serial == second.serial || first.serial != "VIIPER-GJ1-"+a || second.serial != "VIIPER-GJ1-"+b {
		t.Fatalf("unexpected serials: %q and %q", first.serial, second.serial)
	}
	stateA := genericjoystick.InputState{X: 23010, Buttons: 0x8000, Hat: 1}
	stateB := genericjoystick.InputState{X: -11800, Buttons: 1, Hat: 6}
	publish(first, stateA)
	publish(second, stateB)
	checkReport(first, stateA)
	remove(first)
	stateB.X = -32767
	publish(second, stateB)
	reconnected := open(a)
	if reconnected.serial != first.serial {
		t.Fatalf("recreated controller got serial %q, want %q", reconnected.serial, first.serial)
	}
	publish(reconnected, stateA)
	checkReport(second, stateB)
	remove(reconnected)
	remove(second)
	devices, err := usb.ListDevices()
	if err != nil || len(devices) != 0 {
		t.Fatalf("exports after removal: %+v, %v", devices, err)
	}
}
