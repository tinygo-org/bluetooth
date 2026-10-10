//go:build baremetal

package main

import (
	"context"
	"errors"
	"strings"
	"time"
)

// Devices are the MAC addresses of the Bluetooth peripherals you want to connect to.
// Replace this by using -ldflags="-X main.Devices='[MAC ADDRESS],[MAC ADDRESS]'"
// where [MAC ADDRESS] is the actual MAC address of the peripheral.
// For example:
// tinygo flash -target nano-rp2040 -ldflags="-X main.Devices='7B:36:98:8C:41:1C,7B:36:98:8C:41:1D" ./examples/heartrate-monitor/
var Devices string

func initExitHandler() context.Context {
	return context.Background()
}

func connectAddresses() ([]string, error) {
	addrs := strings.Split(Devices, ",")
	if len(addrs) == 0 {
		return nil, errors.New("no devices specified")
	}

	return addrs, nil
}

func failMessage(msg string) {
	for {
		println(msg)
		time.Sleep(1 * time.Second)
	}
}
