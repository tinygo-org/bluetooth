// This example advertises for 5 minutes after
// - boot
// - disconnect
// and then stops advertising.
package main

import (
	"time"

	"tinygo.org/x/bluetooth"
)

var adapter = bluetooth.DefaultAdapter

var advUntil = time.Now().Add(5 * time.Minute)
var advState = true

func main() {
	must("enable BLE stack", adapter.Enable())
	adv := adapter.DefaultAdvertisement()
	must("config adv", adv.Configure(bluetooth.AdvertisementOptions{
		LocalName: "Go Bluetooth",
	}))
	adapter.SetConnectHandler(func(device bluetooth.Device, connected bool) {
		if connected {
			println("connected, not advertising...")
			advState = false
		} else {
			println("disconnected, advertising...")
			advState = true
			advUntil = time.Now().Add(5 * time.Minute)
		}
	})
	must("start adv", adv.Start())

	println("advertising...")
	address, _ := adapter.Address()
	for {
		if advState && time.Now().After(advUntil) {
			println("timeout, not advertising...")
			advState = false
			must("stop adv", adv.Stop())
		}
		println("Go Bluetooth /", address.MAC.String())
		time.Sleep(time.Second)
	}
}

func must(action string, err error) {
	if err != nil {
		panic("failed to " + action + ": " + err.Error())
	}
}
