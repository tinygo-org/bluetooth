// Example that demonstrates BLE Just Works pairing in the peripheral role,
// currently supported on Nordic SoftDevices only.
//
// Just Works encrypts the link with no user interaction, but gives no
// protection against a man-in-the-middle: any nearby device can pair.
//
// It advertises as a keyboard accessory (HID over GATT, appearance 961) but
// never actually sends a key - see examples/hidkeyboard for a functional
// keyboard. The point here is that hosts generally only show and interact
// with recognized accessory types (HID, audio, ...) through their standard
// Bluetooth settings UI; a peripheral exposing only a custom GATT service
// typically needs a separate GATT browser app to even see, let alone
// exercise. A HID accessory, wired up with nothing but a Battery Service
// whose level counts down from 100 to 1 (and back) every two seconds, lets
// pairing, encryption and notifications all be verified from the OS's own
// Bluetooth settings on most platforms, no extra app required.
//
// The bond survives a reset of this device: the central can reconnect and
// re-encrypt the link without pairing again. If pairing ever gets into a bad
// state (for example the central was deleted from its own Bluetooth
// settings but this device still thinks it is bonded), delete the device
// from the central's Bluetooth settings and send 'r' on this device's serial
// console to remove the bond here too - both sides need to forget each
// other for a clean re-pair.
//
// See examples/pairing-passkey for a pairing method with
// man-in-the-middle protection.
package main

import (
	"machine"
	"time"

	"tinygo.org/x/bluetooth"
)

var adapter = bluetooth.DefaultAdapter

// Standard keyboard report map: 8 modifier bits, 1 reserved byte, 5 LED
// output bits and 6 key codes (the same report layout as the USB boot
// protocol, without report IDs). No report is ever actually sent here; its
// presence is what lets hosts recognize this as a keyboard accessory.
var reportMap = []byte{
	0x05, 0x01, // Usage Page (Generic Desktop)
	0x09, 0x06, // Usage (Keyboard)
	0xA1, 0x01, // Collection (Application)
	0x05, 0x07, //   Usage Page (Key Codes)
	0x19, 0xE0, //   Usage Minimum (224)
	0x29, 0xE7, //   Usage Maximum (231)
	0x15, 0x00, //   Logical Minimum (0)
	0x25, 0x01, //   Logical Maximum (1)
	0x75, 0x01, //   Report Size (1)
	0x95, 0x08, //   Report Count (8)
	0x81, 0x02, //   Input (Data, Variable, Absolute): modifier keys
	0x95, 0x01, //   Report Count (1)
	0x75, 0x08, //   Report Size (8)
	0x81, 0x01, //   Input (Constant): reserved byte
	0x95, 0x06, //   Report Count (6)
	0x75, 0x08, //   Report Size (8)
	0x15, 0x00, //   Logical Minimum (0)
	0x25, 0x65, //   Logical Maximum (101)
	0x05, 0x07, //   Usage Page (Key Codes)
	0x19, 0x00, //   Usage Minimum (0)
	0x29, 0x65, //   Usage Maximum (101)
	0x81, 0x00, //   Input (Data, Array): key codes
	0xC0, // End Collection
}

var batteryLevel bluetooth.Characteristic

func main() {
	time.Sleep(3 * time.Second)
	println("starting")
	must("enable BLE stack", adapter.Enable())

	must("enable pairing", adapter.EnablePairing(bluetooth.PairingParams{
		IOCapabilities: bluetooth.IOCapsNone,
		LESC:           true,
		PairingCompleteHandler: func(device bluetooth.Device, err error) {
			if err != nil {
				println("pairing failed:", err.Error())
			} else {
				println("pairing complete")
			}
		},
	}))

	// The device information service with the PnP ID characteristic is
	// required by the HID over GATT profile.
	must("add device information service", adapter.AddService(&bluetooth.Service{
		UUID: bluetooth.ServiceUUIDDeviceInformation,
		Characteristics: []bluetooth.CharacteristicConfig{
			{
				UUID:  bluetooth.CharacteristicUUIDManufacturerNameString,
				Value: []byte("TinyGo"),
				Flags: bluetooth.CharacteristicReadPermission,
			},
			{
				UUID: bluetooth.CharacteristicUUIDPnPID,
				// Vendor ID source (0x02 = USB), vendor 0x1915, product
				// 0x0001, version 0x0001 (all little-endian).
				Value: []byte{0x02, 0x15, 0x19, 0x01, 0x00, 0x01, 0x00},
				Flags: bluetooth.CharacteristicReadPermission,
			},
		},
	}))

	// A visible sign that the encrypted link actually works: the level
	// counts down over notifications sent on this same link.
	must("add battery service", adapter.AddService(&bluetooth.Service{
		UUID: bluetooth.ServiceUUIDBattery,
		Characteristics: []bluetooth.CharacteristicConfig{
			{
				Handle:       &batteryLevel,
				UUID:         bluetooth.CharacteristicUUIDBatteryLevel,
				Value:        []byte{100},
				Flags:        bluetooth.CharacteristicReadPermission | bluetooth.CharacteristicNotifyPermission,
				ReadSecurity: bluetooth.SecurityEncrypted,
			},
		},
	}))

	// The HID service itself. No input report is ever written; it only
	// needs to exist for hosts to recognize the keyboard appearance below.
	must("add HID service", adapter.AddService(&bluetooth.Service{
		UUID: bluetooth.ServiceUUIDHumanInterfaceDevice,
		Characteristics: []bluetooth.CharacteristicConfig{
			{
				UUID: bluetooth.CharacteristicUUIDHIDInformation,
				// bcdHID 1.11, no country code, normally connectable.
				Value: []byte{0x11, 0x01, 0x00, 0x02},
				Flags: bluetooth.CharacteristicReadPermission,
			},
			{
				UUID:         bluetooth.CharacteristicUUIDReportMap,
				Value:        reportMap,
				Flags:        bluetooth.CharacteristicReadPermission,
				ReadSecurity: bluetooth.SecurityEncrypted,
			},
			{
				UUID:         bluetooth.CharacteristicUUIDProtocolMode,
				Value:        []byte{1}, // report protocol
				Flags:        bluetooth.CharacteristicReadPermission | bluetooth.CharacteristicWriteWithoutResponsePermission,
				ReadSecurity: bluetooth.SecurityEncrypted,
			},
			{
				UUID:         bluetooth.CharacteristicUUIDReport,
				Value:        make([]byte, 8),
				Flags:        bluetooth.CharacteristicReadPermission | bluetooth.CharacteristicNotifyPermission,
				ReadSecurity: bluetooth.SecurityEncrypted,
				Descriptors: []bluetooth.DescriptorConfig{
					{
						UUID:         bluetooth.New16BitUUID(0x2908), // Report Reference
						Value:        []byte{0, 1},                   // report ID 0, input report
						ReadSecurity: bluetooth.SecurityEncrypted,
					},
				},
			},
			{
				UUID:          bluetooth.CharacteristicUUIDHIDControlPoint,
				Value:         []byte{0},
				Flags:         bluetooth.CharacteristicWriteWithoutResponsePermission,
				WriteSecurity: bluetooth.SecurityEncrypted,
			},
		},
	}))

	adv := adapter.DefaultAdvertisement()
	must("config adv", adv.Configure(bluetooth.AdvertisementOptions{
		LocalName:    "TinyGo JustWorks",
		ServiceUUIDs: []bluetooth.UUID{bluetooth.ServiceUUIDHumanInterfaceDevice},
		Appearance:   961, // keyboard
	}))
	must("start adv", adv.Start())
	println("advertising as 'TinyGo JustWorks'...")
	println("send 'r' on the serial console to remove the bond")

	const tick = 100 * time.Millisecond
	const batteryInterval = 2 * time.Second
	level := byte(100)
	var elapsed time.Duration
	for {
		if b, err := machine.Serial.ReadByte(); err == nil && b == 'r' {
			println("removing bond...")
			if err := adapter.RemoveBond(); err != nil {
				println("failed to remove bond:", err.Error())
			} else {
				println("bond removed: the next central to connect can pair fresh")
			}
		}
		elapsed += tick
		if elapsed >= batteryInterval {
			elapsed = 0
			level--
			if level == 0 {
				level = 100
			}
			batteryLevel.Write([]byte{level})
		}
		time.Sleep(tick)
	}
}

func must(action string, err error) {
	if err != nil {
		panic("failed to " + action + ": " + err.Error())
	}
}
