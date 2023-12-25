// Test for setting connection parameters.
//
// To test this feature, run this either on a desktop OS or by flashing it to a
// device with TinyGo. Then connect to it from a BLE connection debugger, for
// example nRF Connect on Android. After a second, you should see in the log of
// the BLE app that the connection latency has been updated. It might look
// something like this:
//
//	Connection parameters updated (interval: 510.0ms, latency: 0, timeout: 10000ms)
package main

import (
	"time"

	"tinygo.org/x/bluetooth"
)

var (
	adapter   = bluetooth.DefaultAdapter
	newDevice chan bluetooth.Device
)

func main() {
	must("enable BLE stack", adapter.Enable())

	newDevice = make(chan bluetooth.Device, 1)
	adapter.SetConnectHandler(func(device bluetooth.Device, connected bool) {
		// If this is a new device, signal it to the separate goroutine.
		if connected {
			select {
			case newDevice <- device:
			default:
			}
		}
	})

	// Start advertising, so we can be found.
	const name = "Go BLE test"
	adv := adapter.DefaultAdvertisement()
	adv.Configure(bluetooth.AdvertisementOptions{
		LocalName: name,
	})
	adv.Start()
	println("advertising:", name)

	for device := range newDevice {
		println("connection from device:", device.Address.String())

		// Discover services and characteristics.
		svcs, err := device.DiscoverServices(nil)
		if err != nil {
			println("  failed to resolve services:", err)
		}
		for _, svc := range svcs {
			println("  service:", svc.UUID().String())
			chars, err := svc.DiscoverCharacteristics(nil)
			if err != nil {
				println("    failed to resolve characteristics:", err)
			}
			for _, char := range chars {
				println("    characteristic:", char.UUID().String())
			}
		}

		// Update connection parameters (as a test).
		time.Sleep(time.Second)
		err = device.RequestConnectionParams(bluetooth.ConnectionParams{
			MinInterval: bluetooth.NewDuration(495 * time.Millisecond),
			MaxInterval: bluetooth.NewDuration(510 * time.Millisecond),
			Timeout:     bluetooth.NewDuration(10 * time.Second),
		})
		if err != nil {
			println("  failed to update connection parameters:", err)
			continue
		}
		println("  updated connection parameters")
	}
}

func must(action string, err error) {
	if err != nil {
		panic("failed to " + action + ": " + err.Error())
	}
}
