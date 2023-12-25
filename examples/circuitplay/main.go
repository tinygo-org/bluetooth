// This example is intended to be used with the Adafruit Circuitplay Bluefruit board.
// It allows you to control the color of the built-in NeoPixel LEDS while they animate
// in a circular pattern.
package main

import (
	"image/color"
	"machine"
	"time"

	"tinygo.org/x/bluetooth"
	"tinygo.org/x/drivers/ws2812"
)

var adapter = bluetooth.DefaultAdapter

// TODO: use atomics to access this value.
var ledColor = [3]byte{0xff, 0x00, 0x00} // start out with red
var leds [10]color.RGBA

var (
	serviceUUID = [16]byte{0xa0, 0xb4, 0x00, 0x01, 0x92, 0x6d, 0x4d, 0x61, 0x98, 0xdf, 0x8c, 0x5c, 0x62, 0xee, 0x53, 0xb3}
	charUUID    = [16]byte{0xa0, 0xb4, 0x00, 0x02, 0x92, 0x6d, 0x4d, 0x61, 0x98, 0xdf, 0x8c, 0x5c, 0x62, 0xee, 0x53, 0xb3}
)

var neo machine.Pin = machine.NEOPIXELS
var led machine.Pin = machine.LED
var ws ws2812.Device
var rg bool

var connected bool
var disconnected bool = true

func main() {
	println("starting")

	led.Configure(machine.PinConfig{Mode: machine.PinOutput})
	neo.Configure(machine.PinConfig{Mode: machine.PinOutput})
	ws = ws2812.New(neo)

	adapter.SetConnectHandler(func(d bluetooth.Device, c bool) {
		connected = c

		if !connected && !disconnected {
			clearLEDS()
			disconnected = true
		}

		if connected {
			disconnected = false
		}
	})

	must("enable BLE stack", adapter.Enable())
	adv := adapter.DefaultAdvertisement()
	must("config adv", adv.Configure(bluetooth.AdvertisementOptions{
		LocalName: "TinyGo colors",
	}))
	must("start adv", adv.Start())

	var ledColorCharacteristic bluetooth.Characteristic
	must("add service", adapter.AddService(&bluetooth.Service{
		UUID: bluetooth.NewUUID(serviceUUID),
		Characteristics: []bluetooth.CharacteristicConfig{
			{
				Handle: &ledColorCharacteristic,
				UUID:   bluetooth.NewUUID(charUUID),
				Value:  ledColor[:],
				Flags:  bluetooth.CharacteristicReadPermission | bluetooth.CharacteristicWritePermission | bluetooth.CharacteristicWriteWithoutResponsePermission,
				WriteEvent: func(client bluetooth.Connection, offset int, value []byte) {
					if offset != 0 || len(value) != 3 {
						return
					}
					ledColor[0] = value[0]
					ledColor[1] = value[1]
					ledColor[2] = value[2]
				},
			},
		},
	}))

	for {
		rg = !rg
		if connected {
			writeLEDS()
		}
		led.Set(rg)
		time.Sleep(100 * time.Millisecond)
	}
}

func must(action string, err error) {
	if err != nil {
		panic("failed to " + action + ": " + err.Error())
	}
}

func writeLEDS() {
	for i := range leds {
		rg = !rg
		if rg {
			leds[i] = color.RGBA{R: ledColor[0], G: ledColor[1], B: ledColor[2]}
		} else {
			leds[i] = color.RGBA{R: 0x00, G: 0x00, B: 0x00}
		}
	}

	ws.WriteColors(leds[:])
}

func clearLEDS() {
	for i := range leds {
		leds[i] = color.RGBA{R: 0x00, G: 0x00, B: 0x00}
	}

	ws.WriteColors(leds[:])
}
