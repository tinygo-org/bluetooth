//go:build baremetal

package main

import (
	"time"
)

// DeviceAddress is the MAC address of the Bluetooth peripheral you want to connect to.
// Replace this by using -ldflags="-X main.DeviceAddress=[MAC ADDRESS]"
// where [MAC ADDRESS] is the actual MAC address of the peripheral.
// For example:
// tinygo flash -target circuitplay-bluefruit -ldflags="-X main.DeviceAddress=7B:36:98:8C:41:1C" ./examples/discover/
var DeviceAddress string

func connectAddress() string {
	return DeviceAddress
}

// wait on baremetal, proceed immediately on desktop OS.
func wait() {
	time.Sleep(3 * time.Second)
}

// done just blocks forever, allows USB CDC reset for flashing new software.
func done() {
	println("Done.")

	time.Sleep(1 * time.Hour)
}
