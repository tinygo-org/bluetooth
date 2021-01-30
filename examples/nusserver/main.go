package main

// This example implements a NUS (Nordic UART Service) peripheral.
// I can't find much official documentation on the protocol, but this can be
// helpful:
// https://learn.adafruit.com/introducing-adafruit-ble-bluetooth-low-energy-friend/uart-service
//
// Code to interact with a raw terminal is in separate files with build tags.

import (
	"tinygo.org/x/bluetooth"
	"tinygo.org/x/bluetooth/rawterm"
)

var (
	serviceUUID = bluetooth.ServiceUUIDNordicUART
	rxUUID      = bluetooth.CharacteristicUUIDUARTRX
	txUUID      = bluetooth.CharacteristicUUIDUARTTX
)

func main() {
	println("starting")
	adapter := bluetooth.DefaultAdapter
	must("enable BLE stack", adapter.Enable())
	adv := adapter.DefaultAdvertisement()
	must("config adv", adv.Configure(bluetooth.AdvertisementOptions{
		LocalName:    "NUS", // Nordic UART Service
		ServiceUUIDs: []bluetooth.UUID{serviceUUID},
	}))
	must("start adv", adv.Start())

	var rxChar bluetooth.Characteristic
	var txChar bluetooth.Characteristic
	must("add service", adapter.AddService(&bluetooth.Service{
		UUID: serviceUUID,
		Characteristics: []bluetooth.CharacteristicConfig{
			{
				Handle: &rxChar,
				UUID:   rxUUID,
				Flags:  bluetooth.CharacteristicWritePermission | bluetooth.CharacteristicWriteWithoutResponsePermission,
				WriteEvent: func(client bluetooth.Connection, offset int, value []byte) {
					txChar.Write(value)
					for _, c := range value {
						rawterm.Putchar(c)
					}
				},
			},
			{
				Handle: &txChar,
				UUID:   txUUID,
				Flags:  bluetooth.CharacteristicNotifyPermission | bluetooth.CharacteristicReadPermission,
			},
		},
	}))

	rawterm.Configure()
	defer rawterm.Restore()
	print("NUS console enabled, use Ctrl-X to exit\r\n")
	var line []byte
	for {
		ch := rawterm.Getchar()
		rawterm.Putchar(ch)
		line = append(line, ch)

		// Send the current line to the central.
		if ch == '\x18' {
			// The user pressed Ctrl-X, exit the terminal.
			break
		} else if ch == '\n' {
			sendbuf := line // copy buffer
			// Reset the slice while keeping the buffer in place.
			line = line[:0]

			// Send the sendbuf after breaking it up in pieces.
			for len(sendbuf) != 0 {
				// Chop off up to 20 bytes from the sendbuf.
				partlen := 20
				if len(sendbuf) < 20 {
					partlen = len(sendbuf)
				}
				part := sendbuf[:partlen]
				sendbuf = sendbuf[partlen:]
				// This also sends a notification.
				_, err := txChar.Write(part)
				must("send notification", err)
			}
		}
	}
}

func must(action string, err error) {
	if err != nil {
		panic("failed to " + action + ": " + err.Error())
	}
}
