//go:build pyportal

package main

import (
	"machine"

	"tinygo.org/x/drivers/ili9341"
	"tinygo.org/x/tinyfont/proggy"
	"tinygo.org/x/tinyterm"
)

var (
	font = &proggy.TinySZ8pt7b
)

func initTerminal() {
	display := ili9341.NewParallel(
		machine.LCD_DATA0,
		machine.TFT_WR,
		machine.TFT_DC,
		machine.TFT_CS,
		machine.TFT_RESET,
		machine.TFT_RD,
	)

	// configure backlight
	backlight := machine.TFT_BACKLIGHT
	backlight.Configure(machine.PinConfig{machine.PinOutput})

	// configure display
	display.Configure(ili9341.Config{})
	display.SetRotation(ili9341.Rotation270)
	display.FillScreen(black)

	backlight.High()

	terminal = tinyterm.NewTerminal(display)
	terminal.Configure(&tinyterm.Config{
		Font:              font,
		FontHeight:        10,
		FontOffset:        6,
		UseSoftwareScroll: true,
	})
}
