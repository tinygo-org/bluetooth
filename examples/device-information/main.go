// example that demonstrates how to create a BLE peripheral device with the Device Information Service.
package main

import (
	"time"

	"tinygo.org/x/bluetooth"
)

var adapter = bluetooth.DefaultAdapter

var (
	localName = "TinyGo Device"

	manufacturerName               = "TinyGo"
	manufacturerNameCharacteristic bluetooth.Characteristic

	modelNumber               = "Model 1"
	modelNumberCharacteristic bluetooth.Characteristic

	serialNumber               = "123456"
	serialNumberCharacteristic bluetooth.Characteristic

	softwareVersion               = "1.0.0"
	softwareVersionCharacteristic bluetooth.Characteristic

	firmwareVersion               = "1.0.0"
	firmwareVersionCharacteristic bluetooth.Characteristic

	hardwareVersion               = "1.0.0"
	hardwareVersionCharacteristic bluetooth.Characteristic
)

func main() {
	println("starting")
	must("enable BLE stack", adapter.Enable())
	adv := adapter.DefaultAdvertisement()
	must("config adv", adv.Configure(bluetooth.AdvertisementOptions{
		LocalName:    localName,
		ServiceUUIDs: []bluetooth.UUID{bluetooth.ServiceUUIDDeviceInformation},
	}))
	must("start adv", adv.Start())

	must("add service", adapter.AddService(&bluetooth.Service{
		UUID: bluetooth.ServiceUUIDDeviceInformation,
		Characteristics: []bluetooth.CharacteristicConfig{
			{
				Handle: &manufacturerNameCharacteristic,
				UUID:   bluetooth.CharacteristicUUIDManufacturerNameString,
				Value:  []byte(manufacturerName),
				Flags:  bluetooth.CharacteristicReadPermission,
			},
			{
				Handle: &modelNumberCharacteristic,
				UUID:   bluetooth.CharacteristicUUIDModelNumberString,
				Value:  []byte(modelNumber),
				Flags:  bluetooth.CharacteristicReadPermission,
			},
			{
				Handle: &serialNumberCharacteristic,
				UUID:   bluetooth.CharacteristicUUIDSerialNumberString,
				Value:  []byte(serialNumber),
				Flags:  bluetooth.CharacteristicReadPermission,
			},
			{
				Handle: &softwareVersionCharacteristic,
				UUID:   bluetooth.CharacteristicUUIDSoftwareRevisionString,
				Value:  []byte(softwareVersion),
				Flags:  bluetooth.CharacteristicReadPermission,
			},
			{
				Handle: &firmwareVersionCharacteristic,
				UUID:   bluetooth.CharacteristicUUIDFirmwareRevisionString,
				Value:  []byte(firmwareVersion),
				Flags:  bluetooth.CharacteristicReadPermission,
			},
			{
				Handle: &hardwareVersionCharacteristic,
				UUID:   bluetooth.CharacteristicUUIDHardwareRevisionString,
				Value:  []byte(hardwareVersion),
				Flags:  bluetooth.CharacteristicReadPermission,
			},
		},
	}))

	for {
		time.Sleep(time.Second)
	}
}

func must(action string, err error) {
	if err != nil {
		panic("failed to " + action + ": " + err.Error())
	}
}
