package main

// This example implements a NUS (Nordic UART Service) client. See nusserver for
// details.

import (
	"tinygo.org/x/bluetooth"
	"tinygo.org/x/bluetooth/rawterm"
)

var (
	serviceUUID = bluetooth.ServiceUUIDNordicUART
	rxUUID      = bluetooth.CharacteristicUUIDUARTRX
	txUUID      = bluetooth.CharacteristicUUIDUARTTX
)

var adapter = bluetooth.DefaultAdapter

func main() {
	// Enable BLE interface.
	err := adapter.Enable()
	if err != nil {
		println("could not enable the BLE stack:", err.Error())
		return
	}

	// The address to connect to. Set during scanning and read afterwards.
	var foundDevice bluetooth.ScanResult

	// Scan for NUS peripheral.
	println("Scanning...")
	err = adapter.Scan(func(adapter *bluetooth.Adapter, result bluetooth.ScanResult) {
		if !result.AdvertisementPayload.HasServiceUUID(serviceUUID) {
			return
		}
		foundDevice = result

		// Stop the scan.
		err := adapter.StopScan()
		if err != nil {
			// Unlikely, but we can't recover from this.
			println("failed to stop the scan:", err.Error())
		}
	})
	if err != nil {
		println("could not start a scan:", err.Error())
		return
	}

	// Found a device: print this event.
	if name := foundDevice.LocalName(); name == "" {
		print("Connecting to ", foundDevice.Address.String(), "...")
		println()
	} else {
		print("Connecting to ", name, " (", foundDevice.Address.String(), ")...")
		println()
	}

	// Found a NUS peripheral. Connect to it.
	device, err := adapter.Connect(foundDevice.Address, bluetooth.ConnectionParams{})
	if err != nil {
		println("Failed to connect:", err.Error())
		return
	}

	// Connected. Look up the Nordic UART Service.
	println("Discovering service...")
	services, err := device.DiscoverServices([]bluetooth.UUID{serviceUUID})
	if err != nil {
		println("Failed to discover the Nordic UART Service:", err.Error())
		return
	}
	service := services[0]

	// Get the two characteristics present in this service.
	chars, err := service.DiscoverCharacteristics([]bluetooth.UUID{rxUUID, txUUID})
	if err != nil {
		println("Failed to discover RX and TX characteristics:", err.Error())
		return
	}
	rx := chars[0]
	tx := chars[1]

	// Enable notifications to receive incoming data.
	err = tx.EnableNotifications(func(value []byte) {
		for _, c := range value {
			rawterm.Putchar(c)
		}
	})
	if err != nil {
		println("Failed to enable TX notifications:", err.Error())
		return
	}

	println("Connected. Exit console using Ctrl-X.")
	rawterm.Configure()
	defer rawterm.Restore()
	var line []byte
	for {
		ch := rawterm.Getchar()
		line = append(line, ch)

		// Send the current line to the central.
		if ch == '\x18' {
			// The user pressed Ctrl-X, exit the program.
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
				// This performs a "write command" aka "write without response".
				_, err := rx.WriteWithoutResponse(part)
				if err != nil {
					println("could not send:", err.Error())
				}
			}
		}
	}
}
