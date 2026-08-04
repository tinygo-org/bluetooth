package main

import (
	"time"

	"tinygo.org/x/bluetooth"
)

var adapter = bluetooth.DefaultAdapter

func main() {
	time.Sleep(time.Second) // wait for the system to initialize
	println("Starting BLE")

	// Enable BLE interface.
	must("enable BLE stack", adapter.Enable())

	// On the HCI backends scanning is active by default, so a device that puts
	// its name in a scan response is reported twice: once for the
	// advertisement and once for the scan response. Uncomment to listen only,
	// at the cost of missing that data.
	// adapter.SetScanType(bluetooth.ScanTypePassive)

	// Start scanning.
	println("scanning...")
	err := adapter.Scan(func(adapter *bluetooth.Adapter, device bluetooth.ScanResult) {
		println("found device:", device.Address.String(), device.RSSI, device.LocalName())
	})
	must("start scan", err)
}

func must(action string, err error) {
	if err != nil {
		panic("failed to " + action + ": " + err.Error())
	}
}
