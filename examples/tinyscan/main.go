package main

import (
	"fmt"
	"image/color"
	"time"

	"tinygo.org/x/bluetooth"
	"tinygo.org/x/tinyterm"
)

var (
	terminal *tinyterm.Terminal

	black   = color.RGBA{0, 0, 0, 255}
	adapter = bluetooth.DefaultAdapter
)

func main() {
	initTerminal()

	terminalOutput("enable interface...")

	must("enable BLE interface", adapter.Enable())
	time.Sleep(time.Second)

	terminalOutput("start scan...")

	must("start scan", adapter.Scan(scanHandler))

	for {
		time.Sleep(time.Minute)
		terminalOutput("scanning...")
	}
}

func scanHandler(adapter *bluetooth.Adapter, device bluetooth.ScanResult) {
	msg := fmt.Sprintf("%s %d %s", device.Address.String(), device.RSSI, device.LocalName())
	terminalOutput(msg)
}

func must(action string, err error) {
	if err != nil {
		for {
			terminalOutput("failed to " + action + ": " + err.Error())

			time.Sleep(time.Second)
		}
	}
}

func terminalOutput(s string) {
	println(s)
	fmt.Fprintf(terminal, "\n%s", s)
}
