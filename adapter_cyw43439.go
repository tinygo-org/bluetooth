//go:build cyw43439

package bluetooth

import (
	"machine"

	"log/slog"

	"github.com/soypat/cyw43439"
)

const maxConnections = 1

// Adapter represents a SPI connection to the HCI controller on an attached CYW4349 module.
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
	if debug {
		println("Initializing CYW43439 device")
	}

	dev := cyw43439.NewPicoWDevice()
	cfg := cyw43439.DefaultBluetoothConfig()
	if debug {
		cfg.Logger = slog.New(slog.NewTextHandler(machine.USBCDC, &slog.HandlerOptions{
			Level: slog.LevelDebug - 2,
		}))
	}

	err := dev.Init(cfg)
	if err != nil {
		if debug {
			println("Error initializing CYW43439 device", err.Error())
		}
		return err
	}

	transport := &hciSPI{dev: dev}

	a.hci, a.att = newBLEStack(transport)
	if debug {
		println("Enabling CYW43439 device")
	}

	a.enable()

	if debug {
		println("Enabled CYW43439 device")
	}

	return nil
}

type hciSPI struct {
	dev *cyw43439.Device
}

func (h *hciSPI) startRead() {
}

func (h *hciSPI) endRead() {
}

func (h *hciSPI) Buffered() int {
	return h.dev.BufferedHCI()
}

func (h *hciSPI) ReadByte() (byte, error) {
	var buf [1]byte

	r, err := h.dev.HCIReadWriter()
	if err != nil {
		return 0, err
	}
	if _, err := r.Read(buf[:]); err != nil {
		return 0, err
	}

	return buf[0], nil
}

func (h *hciSPI) Read(buf []byte) (int, error) {
	r, err := h.dev.HCIReadWriter()
	if err != nil {
		return 0, err
	}

	return r.Read(buf)
}

func (h *hciSPI) Write(buf []byte) (int, error) {
	w, err := h.dev.HCIReadWriter()
	if err != nil {
		return 0, err
	}

	return w.Write(buf)
}
