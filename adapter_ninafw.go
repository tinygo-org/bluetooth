//go:build ninafw

package bluetooth

import (
	"machine"
	"time"
)

const maxConnections = 1

// Adapter represents the HCI connection to the NINA fw using the hardware UART.
type Adapter struct {
	hciAdapter
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

// Enable configures the BLE stack. It must be called before any
// Bluetooth-related calls (unless otherwise indicated).
func (a *Adapter) Enable() error {
	// reset the NINA in BLE mode
	machine.NINA_CS.Configure(machine.PinConfig{Mode: machine.PinOutput})
	machine.NINA_CS.Low()

	if machine.NINA_RESET_INVERTED {
		resetNINAInverted()
	} else {
		resetNINA()
	}

	// serial port for nina chip
	uart := machine.UART_NINA
	cfg := machine.UARTConfig{
		TX:       machine.NINA_TX,
		RX:       machine.NINA_RX,
		BaudRate: machine.NINA_BAUDRATE,
	}
	if !machine.NINA_SOFT_FLOWCONTROL {
		cfg.CTS = machine.NINA_CTS
		cfg.RTS = machine.NINA_RTS
	}

	uart.Configure(cfg)

	transport := &hciUART{uart: uart}
	if machine.NINA_SOFT_FLOWCONTROL {
		machine.NINA_RTS.Configure(machine.PinConfig{Mode: machine.PinOutput})
		machine.NINA_RTS.High()

		machine.NINA_CTS.Configure(machine.PinConfig{Mode: machine.PinInput})
	}

	a.hci, a.att = newBLEStack(transport)
	return a.enable()
}

func resetNINA() {
	machine.NINA_RESETN.Configure(machine.PinConfig{Mode: machine.PinOutput})

	machine.NINA_RESETN.High()
	time.Sleep(100 * time.Millisecond)
	machine.NINA_RESETN.Low()
	time.Sleep(1000 * time.Millisecond)
}

func resetNINAInverted() {
	machine.NINA_RESETN.Configure(machine.PinConfig{Mode: machine.PinOutput})

	machine.NINA_RESETN.Low()
	time.Sleep(100 * time.Millisecond)
	machine.NINA_RESETN.High()
	time.Sleep(1000 * time.Millisecond)
}

type hciUART struct {
	uart *machine.UART
}

func (h *hciUART) startRead() {
	if machine.NINA_SOFT_FLOWCONTROL {
		machine.NINA_RTS.Low()
	}
}

func (h *hciUART) endRead() {
	if machine.NINA_SOFT_FLOWCONTROL {
		machine.NINA_RTS.High()
	}
}

func (h *hciUART) Buffered() int {
	return h.uart.Buffered()
}

func (h *hciUART) ReadByte() (byte, error) {
	return h.uart.ReadByte()
}

const writeAttempts = 200

func (h *hciUART) Write(buf []byte) (int, error) {
	if machine.NINA_SOFT_FLOWCONTROL {
		retries := writeAttempts
		for machine.NINA_CTS.Get() {
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
