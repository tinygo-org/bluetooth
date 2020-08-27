package bluetooth

import (
	"github.com/JuulLabs-OSS/cbgo"
	"github.com/tinygo-org/bluetooth/macbt"
)

type Adapter struct {
	cm  cbgo.CentralManager
	cmd macbt.CMDelegate
}

// DefaultAdapter is the default adapter on the system.
//
// Make sure to call Enable() before using it to initialize the adapter.
var DefaultAdapter = &Adapter{}

// Enable configures the BLE stack. It must be called before any
// Bluetooth-related calls (unless otherwise indicated).
func (a *Adapter) Enable() error {
	a.cm = cbgo.NewCentralManager(nil)
	return nil
}
