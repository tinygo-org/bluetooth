//go:build hci && hci_uart

package bluetooth

import (
	"machine"
)

const maxConnections = 1

// Adapter represents a "plain" UART connection to the HCI controller.
type Adapter struct {
	hciAdapter

	uart *machine.UART

	// used for software flow control
	cts, rts machine.Pin
}

// DefaultAdapter is the default adapter on the current system.
//
// Make sure to call Enable() before using it to initialize the adapter.
var DefaultAdapter = &Adapter{
	hciAdapter: hciAdapter{
		isDefault: true,
		connectHandler: func(device Device, connected bool) {
			return
		},
		connectedDevices: make([]Device, 0, maxConnections),
	},
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
	transport := &hciUART{uart: a.uart}
	if a.cts != 0 && a.rts != 0 {
		transport.rts = a.rts
		a.rts.Configure(machine.PinConfig{Mode: machine.PinOutput})
		a.rts.High()

		transport.cts = a.cts
		a.cts.Configure(machine.PinConfig{Mode: machine.PinInput})
	}

	a.hci, a.att = newBLEStack(transport)
	a.enable()

	return nil
}

type hciUART struct {
	uart *machine.UART

	// used for software flow control
	cts, rts machine.Pin
}

func (h *hciUART) startRead() {
	if h.rts != machine.NoPin {
		h.rts.Low()
	}
}

func (h *hciUART) endRead() {
	if h.rts != machine.NoPin {
		h.rts.High()
	}
}

func (h *hciUART) Buffered() int {
	return h.uart.Buffered()
}

func (h *hciUART) ReadByte() (byte, error) {
	return h.uart.ReadByte()
}

func (h *hciUART) Read(buf []byte) (int, error) {
	return h.uart.Read(buf)
}

const writeAttempts = 200

func (h *hciUART) Write(buf []byte) (int, error) {
	if h.cts != machine.NoPin {
		retries := writeAttempts
		for h.cts.Get() {
			retries--
			if retries == 0 {
				return 0, ErrHCITimeout
			}
		}
	}

	n, err := h.uart.Write(buf)
	if err != nil {
		return 0, err
	}

	return n, nil
}
