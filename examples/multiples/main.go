// This example scans and then connects to multiple Bluetooth peripherals
// that provide the Heart Rate Service (HRS).
//
// Once connected to all the desired devices, it subscribes to notifications.
//
// To run on bare metal microcontroller:
// tinygo flash -target metro-m4-airlift -ldflags="-X main.wanted=D9:2A:A1:5C:ED:56,4D:A1:3C:24:F0:46" -monitor ./examples/multiples/
//
// To run on OS:
// go run ./examples/multiples/ D9:2A:A1:5C:ED:56,64:0B:1D:46:D8:1D
package main

import (
	"context"
	"os"
	"slices"
	"time"

	"tinygo.org/x/bluetooth"
)

var (
	adapter = bluetooth.DefaultAdapter

	heartRateServiceUUID        = bluetooth.ServiceUUIDHeartRate
	heartRateCharacteristicUUID = bluetooth.CharacteristicUUIDHeartRateMeasurement

	exitCtx context.Context
)

func main() {
	exitCtx = initExitHandler()

	println("enabling")

	// Enable BLE interface.
	must("enable BLE stack", adapter.Enable())

	scanResults := make(map[string]bluetooth.ScanResult)
	finished := make(chan bool, 1)

	searchList, _ := connectAddresses()

	// Start scanning.
	println("scanning...")
	err := adapter.Scan(func(adapter *bluetooth.Adapter, result bluetooth.ScanResult) {
		print(".")
		// is the scanned device one of the ones we want?
		if slices.Contains(searchList, result.Address.String()) {
			if _, ok := scanResults[result.Address.String()]; !ok {
				println(".")
				println("found device:", result.Address.String(), result.RSSI, result.LocalName())
				scanResults[result.Address.String()] = result
			}

			if len(scanResults) == len(searchList) {
				println(".")
				adapter.StopScan()
				finished <- true
			}
		}
		select {
		case <-exitCtx.Done():
			println("exiting.")
			os.Exit(0)
		default:
		}
	})
	must("scan", err)

	devices := []bluetooth.Device{}
	select {
	case <-time.After(5 * time.Second):
		failMessage("timed out")
		return
	case <-exitCtx.Done():
		println("exiting.")
		return
	case <-finished:
	}

	defer func() {
		for _, device := range devices {
			device.Disconnect()
		}
	}()

	// now connect to all devices
	for _, result := range scanResults {
		device, err := adapter.Connect(result.Address, bluetooth.ConnectionParams{})
		if err != nil {
			failMessage(err.Error())
			return
		}

		println("connected to", result.Address.String())
		devices = append(devices, device)
	}

	// get services
	println("discovering services/characteristics")

	for _, device := range devices {
		srvcs, err := device.DiscoverServices([]bluetooth.UUID{heartRateServiceUUID})
		must("discover services", err)

		if len(srvcs) == 0 {
			failMessage("could not find heart rate service")
			return
		}

		srvc := srvcs[0]

		println("found service", srvc.UUID().String(), "for device", device.Address.String())

		chars, err := srvc.DiscoverCharacteristics([]bluetooth.UUID{heartRateCharacteristicUUID})
		if err != nil {
			failMessage(err.Error())
			return
		}

		if len(chars) == 0 {
			failMessage("could not find heart rate characteristic")
			return
		}

		char := chars[0]
		addr := device.Address.String()
		println("found characteristic", char.UUID().String(), "for device", addr)

		char.EnableNotifications(func(buf []byte) {
			println(addr, "data:", uint8(buf[1]))
		})
	}

	// wait for exit
	<-exitCtx.Done()
	println("exiting.")
}

func must(action string, err error) {
	if err != nil {
		failMessage("failed to " + action + ": " + err.Error())
		return
	}
}
