package main

import (
	"os"

	"tinygo.org/x/bluetooth"
)

var adapter = bluetooth.DefaultAdapter

func main() {
	if len(os.Args) < 2 {
		println("usage: discover [local name]")
		os.Exit(1)
	}

	// look for device with specific name
	name := os.Args[1]

	// Enable BLE interface.
	must("enable BLE stack", adapter.Enable())

	ch := make(chan bluetooth.ScanResult, 1)

	// Start scanning.
	println("scanning...")
	err := adapter.Scan(func(adapter *bluetooth.Adapter, result bluetooth.ScanResult) {
		println("found device:", result.Address.String(), result.RSSI, result.LocalName())
		if result.LocalName() == name {
			adapter.StopScan()
			ch <- result
		}
	})

	var device *bluetooth.Device
	select {
	case result := <-ch:
		device, err = adapter.Connect(result.Address, bluetooth.ConnectionParams{})
		if err != nil {
			println(err.Error())
			os.Exit(1)
		}

		println("connected to ", result.LocalName())
	}

	// get services
	println("discovering services/characteristics")
	srvcs, err := device.DiscoverServices(nil)
	for _, srvc := range srvcs {
		println("- service", srvc.UUID().String())

		chars, _ := srvc.DiscoverCharacteristics(nil)
		for _, char := range chars {
			println("-- characteristic", char.UUID().String())
		}
	}

	must("start scan", err)
}

func must(action string, err error) {
	if err != nil {
		panic("failed to " + action + ": " + err.Error())
	}
}
