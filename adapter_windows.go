package bluetooth

import (
	"github.com/aykevl/go-bluetooth/winbt"
	"github.com/go-ole/go-ole"
)

type Adapter struct {
	handler func(Event)
	watcher *winbt.IBluetoothLEAdvertisementWatcher
}

var defaultAdapter Adapter

// DefaultAdapter returns the default adapter on the current system.
func DefaultAdapter() (*Adapter, error) {
	return &defaultAdapter, nil
}

// Enable configures the BLE stack. It must be called before any
// Bluetooth-related calls (unless otherwise indicated).
func (a *Adapter) Enable() error {
	return ole.RoInitialize(1) // initialize with multithreading enabled
}
