//go:build espradio

package bluetooth

import (
	"runtime"

	"tinygo.org/x/espradio"
)

const maxConnections = 1

// Adapter represents the BLE adapter on the ESP32 via espradio VHCI transport.
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
// For WiFi+BLE co-existence, call espradio.Enable() first.
func (a *Adapter) Enable() error {
	if err := espradio.BLEInit(); err != nil {
		return err
	}

	transport := &hciVHCI{}

	a.hci, a.att = newBLEStack(transport)

	a.enable()

	return nil
}

// hciVHCI wraps espradio's BLE VHCI transport to implement the
// unexported hciTransport interface.
type hciVHCI struct {
	t espradio.VHCITransport
}

func (h *hciVHCI) startRead() { runtime.Gosched() }
func (h *hciVHCI) endRead()   {}

func (h *hciVHCI) Buffered() int {
	return h.t.Buffered()
}

func (h *hciVHCI) ReadByte() (byte, error) {
	return h.t.ReadByte()
}

func (h *hciVHCI) Read(buf []byte) (int, error) {
	return h.t.Read(buf)
}

func (h *hciVHCI) Write(buf []byte) (int, error) {
	return h.t.Write(buf)
}
