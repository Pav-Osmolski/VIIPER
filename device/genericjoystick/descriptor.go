package genericjoystick

import (
	"github.com/Alia5/VIIPER/usb"
	"github.com/Alia5/VIIPER/usb/hid"
)

// pid.codes PR #1317 allocates this pair to the fixed sixteen-button descriptor.
const registeredVendorID, registeredProductID = 0x1209, 0xD10C

func makeDescriptor(buttonCount int, instanceID string, registered bool) usb.Descriptor {
	items := []hid.Item{
		hid.ReportID{ID: ContractVersion},
		hid.Usage{Usage: hid.UsageX}, hid.Usage{Usage: hid.UsageY},
		hid.Usage{Usage: hid.UsageZ}, hid.Usage{Usage: hid.UsageRx},
		hid.Usage{Usage: hid.UsageRy}, hid.Usage{Usage: hid.UsageRz},
		hid.LogicalMinimum{Min: 0}, hid.LogicalMaximum{Max: 65534},
		hid.ReportSize{Bits: 16}, hid.ReportCount{Count: 6},
		hid.Input{Flags: hid.MainData | hid.MainVar | hid.MainAbs},
		hid.UsagePage{Page: hid.UsagePageButton},
		hid.UsageMinimum{Min: 1}, hid.UsageMaximum{Max: uint16(buttonCount)},
		hid.LogicalMinimum{Min: 0}, hid.LogicalMaximum{Max: 1},
		hid.ReportSize{Bits: 1}, hid.ReportCount{Count: uint16(buttonCount)},
		hid.Input{Flags: hid.MainData | hid.MainVar | hid.MainAbs},
	}
	if buttonCount == 12 {
		items = append(items, hid.ReportCount{Count: 4}, hid.Input{Flags: hid.MainConst})
	}
	items = append(items,
		hid.UsagePage{Page: hid.UsagePageGenericDesktop}, hid.Usage{Usage: 0x39},
		hid.LogicalMinimum{Min: 0}, hid.LogicalMaximum{Max: 7},
		hid.PhysicalMinimum{Min: 0}, hid.PhysicalMaximum{Max: 315}, hid.Unit{Value: 0x14},
		hid.ReportSize{Bits: 4}, hid.ReportCount{Count: 1},
		hid.Input{Flags: hid.MainData | hid.MainVar | hid.MainAbs | hid.MainNullState},
		hid.Unit{Value: 0}, hid.ReportSize{Bits: 4}, hid.Input{Flags: hid.MainConst},
	)
	report := hid.ReportDescriptor{Items: []hid.Item{
		hid.UsagePage{Page: hid.UsagePageGenericDesktop}, hid.Usage{Usage: hid.UsageJoystick},
		hid.Collection{Kind: hid.CollectionApplication, Items: items},
	}}
	// Historical diagnostic fixtures keep their distinct laboratory identities.
	vendorID, productID := uint16(0xFFFF), uint16(0xD100+buttonCount)
	productName := "VIIPER Experimental Legacy Joystick"
	if registered {
		vendorID, productID = registeredVendorID, registeredProductID
		productName = "VIIPER Generic Joystick"
	}
	return usb.Descriptor{
		Device: usb.DeviceDescriptor{
			BcdUSB: 0x0200, BMaxPacketSize0: 64, IDVendor: vendorID,
			IDProduct: productID, BcdDevice: 0x0100,
			IManufacturer: 1, IProduct: 2, ISerialNumber: 3, BNumConfigurations: 1, Speed: 2,
		},
		Interfaces: []usb.InterfaceConfig{{
			Descriptor: usb.InterfaceDescriptor{BNumEndpoints: 1, BInterfaceClass: 3},
			HID: &usb.HIDFunction{
				Descriptor:       usb.HIDDescriptor{BcdHID: 0x0111, Descriptors: []usb.HIDSubDescriptor{{Type: usb.ReportDescType}}},
				ReportDescriptor: report,
			},
			Endpoints: []usb.EndpointDescriptor{{BEndpointAddress: 0x81, BMAttributes: 3, WMaxPacketSize: ReportSize, BInterval: 4}},
		}},
		Strings: map[uint8]string{0: "\u0409", 1: "VIIPER", 2: productName, 3: "VIIPER-GJ1-" + instanceID},
	}
}
