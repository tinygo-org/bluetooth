package bluetooth

import (
	"github.com/go-ole/go-ole"
	"github.com/tinygo-org/bluetooth/winbt"
)

type Adapter struct {
	handler func(Event)
	watcher *winbt.IBluetoothLEAdvertisementWatcher
}

// DefaultAdapter is the default adapter on the system.
//
// Make sure to call Enable() before using it to initialize the adapter.
var DefaultAdapter = Adapter{}

// Enable configures the BLE stack. It must be called before any
// Bluetooth-related calls (unless otherwise indicated).
func (a *Adapter) Enable() error {
	return ole.RoInitialize(1) // initialize with multithreading enabled
}
