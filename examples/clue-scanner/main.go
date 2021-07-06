package main

import (
	"fmt"
	"image/color"
	"machine"
	"time"

	"tinygo.org/x/bluetooth"
	"tinygo.org/x/drivers/st7789"
	"tinygo.org/x/tinyterm"
	"tinygo.org/x/tinyterm/fonts/proggy"
)

var (
	display  st7789.Device
	terminal = tinyterm.NewTerminal(&display)

	black = color.RGBA{0, 0, 0, 255}
	font  = &proggy.TinySZ8pt7b

	adapter = bluetooth.DefaultAdapter
)

func main() {
	initDisplay()
	time.Sleep(time.Second)

	fmt.Fprintf(terminal, "\nenable interface...")
	println("enable interface...")
	must("enable BLE interface", adapter.Enable())
	time.Sleep(time.Second)

	println("start scan...")
	fmt.Fprintf(terminal, "\nstart scan...")

	must("start scan", adapter.Scan(scanHandler))

	for {
		time.Sleep(time.Minute)
		println("scanning...")
		fmt.Fprintf(terminal, "\nscanning...")
	}
}

func scanHandler(adapter *bluetooth.Adapter, device bluetooth.ScanResult) {
	println("device:", device.Address.String(), device.RSSI, device.LocalName())
	fmt.Fprintf(terminal, "\n%s %d %s", device.Address.String(), device.RSSI, device.LocalName())
}

func must(action string, err error) {
	if err != nil {
		for {
			println("failed to " + action + ": " + err.Error())
			time.Sleep(time.Second)
		}
	}
}

func initDisplay() {
	machine.SPI1.Configure(machine.SPIConfig{
		Frequency: 8000000,
		SCK:       machine.TFT_SCK,
		SDO:       machine.TFT_SDO,
		SDI:       machine.TFT_SDO,
		Mode:      0,
	})

	display = st7789.New(machine.SPI1,
		machine.TFT_RESET,
		machine.TFT_DC,
		machine.TFT_CS,
		machine.TFT_LITE)
	display.Configure(st7789.Config{
		Rotation:   st7789.ROTATION_180,
		Height:     320,
		FrameRate:  st7789.FRAMERATE_111,
		VSyncLines: st7789.MAX_VSYNC_SCANLINES,
	})
	display.FillScreen(black)

	terminal.Configure(&tinyterm.Config{
		Font:       font,
		FontHeight: 10,
		FontOffset: 6,
	})
}
