//go:build hci && hci_uart

package bluetooth

import (
	"machine"
)

const maxConnections = 1

// Adapter represents the UART connection to the HCI controller.
type Adapter struct {
	hciAdapter

	// used for software flow control
	cts, rts machine.Pin
}

// DefaultAdapter is the default adapter on the current system.
//
// Make sure to call Enable() before using it to initialize the adapter.
var DefaultAdapter = &Adapter{
	isDefault: true,
	connectHandler: func(device Device, connected bool) {
		return
	},
	connectedDevices: make([]Device, 0, maxConnections),
}

// SetUART sets the UART to use for the HCI connection.
// It must be called before calling Enable().
// Note that the UART must be configured with hardware flow control, or
// SetSoftwareFlowControl() must be called.
func (a *Adapter) SetUART(uart *machine.UART) error {
	a.uart = uart

	return nil
}

// SetSoftwareFlowControl sets the pins to use for software flow control,
// if hardware flow control is not available.
func (a *Adapter) SetSoftwareFlowControl(cts, rts machine.Pin) error {
	a.cts = cts
	a.rts = rts

	return nil
}

// Enable configures the BLE stack. It must be called before any
// Bluetooth-related calls (unless otherwise indicated).
func (a *Adapter) Enable() error {
	a.hci, a.att = newBLEStack(a.uart)

	if a.cts != 0 && a.rts != 0 {
		a.hci.softRTS = a.rts
		a.hci.softRTS.Configure(machine.PinConfig{Mode: machine.PinOutput})
		a.hci.softRTS.High()

		a.hci.softCTS = a.cts
		a.cts.Configure(machine.PinConfig{Mode: machine.PinInput})
	}

	a.enable()
}
