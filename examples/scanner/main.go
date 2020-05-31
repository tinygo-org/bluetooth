package main

import (
	"github.com/tinygo-org/bluetooth"
)

func main() {
	// Enable BLE interface.
	adapter, err := bluetooth.DefaultAdapter()
	must("get default adapter", err)
	must("enable adapter", adapter.Enable())

	// Start scanning.
	println("scanning...")
	err = adapter.Scan(func(adapter *bluetooth.Adapter, device bluetooth.ScanResult) {
		println("found device:", device.Address.String(), device.RSSI, device.LocalName())
	})
	must("start scan", err)
}

func must(action string, err error) {
	if err != nil {
		panic("failed to " + action + ": " + err.Error())
	}
}
