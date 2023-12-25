// This example scans and then connects to a specific Bluetooth peripheral
// and then displays all of the services and characteristics.
//
// To run this on a desktop system:
//
//	go run ./examples/discover EE:74:7D:C9:2A:68
//
// To run this on a microcontroller, change the constant value in the file
// "mcu.go" to set the MAC address of the device you want to discover.
// Then, flash to the microcontroller board like this:
//
//	tinygo flash -o circuitplay-bluefruit ./examples/discover
//
// Once the program is flashed to the board, connect to the USB port
// via serial to view the output.
package main

import (
	"strconv"

	"tinygo.org/x/bluetooth"
)

var adapter = bluetooth.DefaultAdapter

func main() {
	wait()

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
	srvcs, err := device.DiscoverServices(nil)
	must("discover services", err)

	// buffer to retrieve characteristic data
	buf := make([]byte, 255)

	for _, srvc := range srvcs {
		println("- service", srvc.UUID().String())

		chars, err := srvc.DiscoverCharacteristics(nil)
		if err != nil {
			println(err)
		}
		for _, char := range chars {
			println("-- characteristic", char.UUID().String())
			mtu, err := char.GetMTU()
			if err != nil {
				println("    mtu: error:", err.Error())
			} else {
				println("    mtu:", mtu)
			}
			n, err := char.Read(buf)
			if err != nil {
				println("    ", err.Error())
			} else {
				println("    data bytes", strconv.Itoa(n))
				println("    value =", string(buf[:n]))
			}
		}
	}

	err = device.Disconnect()
	if err != nil {
		println(err)
	}

	done()
}

func must(action string, err error) {
	if err != nil {
		panic("failed to " + action + ": " + err.Error())
	}
}
