//go:build baremetal

package main

import (
	"time"
)

// replace this with the MAC address of the Bluetooth peripheral you want to connect to.
const deviceAddress = "E4:B7:F4:11:8D:33"

func connectAddress() string {
	return deviceAddress
}

// done just blocks forever, allows USB CDC reset for flashing new software.
func done() {
	println("Done.")

	time.Sleep(1 * time.Hour)
}
