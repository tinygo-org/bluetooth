// Beacon that writes to the serial port only while the USB cable is connected.
// Without this check, a battery powered board stops when the cable is removed,
// because machine.Serial.Write blocks when its buffer is full.
//
//	tinygo flash -target xiao-ble -monitor ./examples/usbvbus
package main

import (
	"time"

	"tinygo.org/x/bluetooth"
)

var adapter = bluetooth.DefaultAdapter

func main() {
	must("enable BLE stack", adapter.Enable())

	adv := adapter.DefaultAdvertisement()
	must("config adv", adv.Configure(bluetooth.AdvertisementOptions{
		LocalName: "Go USB VBus",
	}))
	must("start adv", adv.Start())

	for count := 0; ; count++ {
		present, err := adapter.USBVBusPresent()
		if err != nil {
			// The board cannot tell, so write the output anyway.
			present = true
		}
		if present {
			println("advertising", count)
		}
		time.Sleep(time.Second)
	}
}

func must(action string, err error) {
	if err != nil {
		panic("failed to " + action + ": " + err.Error())
	}
}
