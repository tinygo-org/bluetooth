// This example pairs this host with a peripheral, or removes the bond with it.
//
//	go run ./examples/pair EE:74:7D:C9:2A:68 [unpair]
package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"tinygo.org/x/bluetooth"
)

var adapter = bluetooth.DefaultAdapter

func main() {
	if len(os.Args) < 2 {
		fmt.Println("usage: pair <address> [unpair]")
		os.Exit(2)
	}
	address := os.Args[1]
	unpair := len(os.Args) > 2 && os.Args[2] == "unpair"

	must("enable BLE stack", adapter.Enable())

	fmt.Println("scanning for", address)
	ch := make(chan bluetooth.ScanResult, 1)
	err := adapter.Scan(func(adapter *bluetooth.Adapter, result bluetooth.ScanResult) {
		if result.Address.String() == address {
			adapter.StopScan()
			ch <- result
		}
	})
	must("scan", err)
	result := <-ch

	device, err := adapter.Connect(result.Address, bluetooth.ConnectionParams{})
	must("connect", err)
	defer device.Disconnect()
	fmt.Println("connected to", address)

	if unpair {
		must("unpair", device.Unpair())
		fmt.Println("unpaired")
		return
	}

	stdin := bufio.NewReader(os.Stdin)
	must("pair", device.Pair(bluetooth.PairingParams{
		IOCapabilities: bluetooth.IOCapsKeyboardDisplay,
		PasskeyDisplayHandler: func(device bluetooth.Device, passkey string) {
			fmt.Println("enter this passkey on the peripheral:", passkey)
		},
		PasskeyEntryHandler: func(device bluetooth.Device) {
			fmt.Print("passkey shown by the peripheral: ")
			passkey, _ := stdin.ReadString('\n')
			if err := device.EnterPasskey(strings.TrimSpace(passkey)); err != nil {
				fmt.Println("enter passkey:", err)
			}
		},
		PasskeyComparisonHandler: func(device bluetooth.Device, passkey string) {
			fmt.Printf("does the peripheral show %s? [y/N] ", passkey)
			answer, _ := stdin.ReadString('\n')
			if err := device.ConfirmPasskey(strings.TrimSpace(answer) == "y"); err != nil {
				fmt.Println("confirm passkey:", err)
			}
		},
	}))
	fmt.Println("paired")
}

func must(action string, err error) {
	if err != nil {
		fmt.Println("failed to " + action + ": " + err.Error())
		os.Exit(1)
	}
}
