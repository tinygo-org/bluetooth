// This example scans and then connects to a specific Bluetooth peripheral
// that can provide the Heart Rate Service (HRS).
//
// Once connected, it subscribes to notifications for the data value, and
// displays it.
// The Heart Rate Measurement characteristic is a variable-length structure (array) containing a Flags field, a Heart
// Rate Measurement Value field and, based on the contents of the Flags field, may contain additional fields
// such as Energy Expended or RR-Interval.
// More info can be found here: https://www.bluetooth.com/specifications/specs/gatt-specification-supplement-6/
// In this example only the heart rate is used, this is the second element in the array of bytes.
//
// To run this on a desktop system:
//
//	go run ./examples/heartrate-monitor EE:74:7D:C9:2A:68
//
// To run this on a microcontroller, change the constant value in the file
// "mcu.go" to set the MAC address of the device you want to discover.
// Then, flash to the microcontroller board like this:
//
//	tinygo flash -o circuitplay-bluefruit ./examples/heartrate-monitor
//
// Once the program is flashed to the board, connect to the USB port
// via serial to view the output.
package main

import (
	"tinygo.org/x/bluetooth"
)

var (
	adapter = bluetooth.DefaultAdapter

	heartRateServiceUUID        = bluetooth.ServiceUUIDHeartRate
	heartRateCharacteristicUUID = bluetooth.CharacteristicUUIDHeartRateMeasurement
)

func main() {
	println("enabling")

	// Enable BLE interface.
	must("enable BLE stack", adapter.Enable())

	ch := make(chan bluetooth.ScanResult, 1)

	// Start scanning.
	println("scanning...")
	err := adapter.Scan(func(adapter *bluetooth.Adapter, result bluetooth.ScanResult) {
		println("found device:", result.Address.String(), result.RSSI, result.LocalName())
		if result.Address.String() == connectAddress() {
			adapter.StopScan()
			ch <- result
		}
	})

	var device bluetooth.Device
	select {
	case result := <-ch:
		device, err = adapter.Connect(result.Address, bluetooth.ConnectionParams{})
		if err != nil {
			println(err.Error())
			return
		}

		println("connected to ", result.Address.String())
	}

	// get services
	println("discovering services/characteristics")
	srvcs, err := device.DiscoverServices([]bluetooth.UUID{heartRateServiceUUID})
	must("discover services", err)

	if len(srvcs) == 0 {
		panic("could not find heart rate service")
	}

	srvc := srvcs[0]

	println("found service", srvc.UUID().String())

	chars, err := srvc.DiscoverCharacteristics([]bluetooth.UUID{heartRateCharacteristicUUID})
	if err != nil {
		println(err)
	}

	if len(chars) == 0 {
		panic("could not find heart rate characteristic")
	}

	char := chars[0]
	println("found characteristic", char.UUID().String())

	char.EnableNotifications(func(buf []byte) {
		println("data:", uint8(buf[1]))
	})

	select {}
}

func must(action string, err error) {
	if err != nil {
		panic("failed to " + action + ": " + err.Error())
	}
}
