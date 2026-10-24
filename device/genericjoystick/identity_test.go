package genericjoystick

import (
	"encoding/binary"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/Alia5/VIIPER/device"
)

func TestIndependentDefaultDevicesHaveDifferentUSBSerials(t *testing.T) {
	first, second := newTestJoystick(t, 12), newTestJoystick(t, 12)
	if first.GetDescriptor().Strings[3] == second.GetDescriptor().Strings[3] {
		t.Fatal("two devices share a USB serial")
	}
}

func TestTwoJoystickLifetimesKeepInputAndReadinessIndependent(t *testing.T) {
	first, second := newTestJoystick(t, 12), newTestJoystick(t, 12)
	a, _ := first.acquireProducer()
	b, _ := second.acquireProducer()
	at := time.Now()
	first.USBConfigurationChanged(1)
	second.USBConfigurationChanged(1)
	if err := first.publish(a, InputState{X: 1234, Buttons: 1, Hat: 8}, at); err != nil {
		t.Fatal(err)
	}
	if err := second.publish(b, InputState{X: -4321, Buttons: 2, Hat: 8}, at); err != nil {
		t.Fatal(err)
	}
	left, right := present(t, first, at.Add(time.Millisecond)), present(t, second, at.Add(time.Millisecond))
	if int(binary.LittleEndian.Uint16(left[1:]))-32767 != 1234 || int(binary.LittleEndian.Uint16(right[1:]))-32767 != -4321 {
		t.Fatal("controller axes crossed between instances")
	}
	first.releaseProducer(a)
	first.USBConfigurationChanged(0)
	if !second.GetDeviceSpecificArgs()["inputConfigured"].(bool) {
		t.Fatal("retiring first device cleared second readiness")
	}
	if err := second.publish(b, InputState{X: 2000, Buttons: 2, Hat: 8}, at.Add(2*time.Millisecond)); err != nil {
		t.Fatal(err)
	}
	continued := present(t, second, at.Add(3*time.Millisecond))
	if int(binary.LittleEndian.Uint16(continued[1:]))-32767 != 2000 || binary.LittleEndian.Uint16(continued[13:]) != 2 {
		t.Fatal("second device stopped or adopted first controller state")
	}
}

func TestSuppliedIdentitySurvivesRecreationAndDoesNotChangeHIDLayout(t *testing.T) {
	create := func(id string) *Joystick {
		t.Helper()
		args, _ := json.Marshal(CreateOptions{Version: 1, Experimental: true, ButtonCount: 12, InstanceID: id})
		j, err := New(&device.CreateOptions{DeviceSpecific: string(args)})
		if err != nil {
			t.Fatal(err)
		}
		return j
	}
	const id = "0123456789ABCDEF0123456789ABCDEF"
	first, again := create(id), create(strings.ToLower(id))
	other := create("FEDCBA9876543210FEDCBA9876543210")
	if first.GetDescriptor().Strings[3] != again.GetDescriptor().Strings[3] || first.GetDescriptor().Strings[3] == other.GetDescriptor().Strings[3] {
		t.Fatal("identity depends on creation order or is shared between controllers")
	}
	if first.GetDeviceSpecificArgs()["instanceId"] != id {
		t.Fatal("missing confirmed identity")
	}
	a, _ := first.GetDescriptor().Interfaces[0].HID.ReportDescriptor.Bytes()
	b, _ := other.GetDescriptor().Interfaces[0].HID.ReportDescriptor.Bytes()
	if string(a) != string(b) {
		t.Fatal("instance identity changed the HID layout")
	}
	for _, invalid := range []string{"short", strings.Repeat("G", 32), strings.Repeat("A", 34), "../" + strings.Repeat("A", 29)} {
		args, _ := json.Marshal(CreateOptions{Version: 1, Experimental: true, ButtonCount: 12, InstanceID: invalid})
		if _, err := New(&device.CreateOptions{DeviceSpecific: string(args)}); err == nil {
			t.Fatalf("accepted invalid identity %q", invalid)
		}
	}
}
