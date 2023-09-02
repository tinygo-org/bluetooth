// This example program shows using Go routines and channels to coordinate
// BLE scanning.
//
// The first Go routine starts scanning using the BLE adaptor. When it finds
// a new device, it puts the information into a channel so it can be displayed.
//
// The second Go routine is a ticker that puts a "true" value into a channel every 3 seconds.
//
// The main function uses a select{} statement to wait until one of the two channels is unblocked
// by receiving data. If a new device is found, the boolean variable named "found" will
// be set to true, so that the timeout is reset for each 3 second period.
package main

import (
	"time"

	"tinygo.org/x/bluetooth"
)

var (
	adapter = bluetooth.DefaultAdapter
	devices = make(chan bluetooth.ScanResult, 1)
	ticker  = make(chan bool, 1)
	found   = true
)

func main() {
	// Enable BLE interface.
	if err := adapter.Enable(); err != nil {
		panic("failed to enable adaptor:" + err.Error())
	}

	// Start scanning
	go performScan()

	// Start timeout ticker
	go startTicker()

	// Wait for devices to be scanned
	for {
		select {
		case device := <-devices:
			found = true
			println("found device:", device.Address.String(), device.RSSI, device.LocalName())
		case <-ticker:
			if !found {
				println("no devices found in last 3 seconds...")
			}
			found = false
		}
	}
}

func performScan() {
	println("scanning...")

	err := adapter.Scan(func(adapter *bluetooth.Adapter, device bluetooth.ScanResult) {
		devices <- device
	})
	if err != nil {
		panic("failed to scan:" + err.Error())
	}
}

func startTicker() {
	for {
		time.Sleep(3 * time.Second)
		ticker <- true
	}

}
