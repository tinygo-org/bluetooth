// +build !baremetal

// Some documentation for the BlueZ D-Bus interface:
// https://git.kernel.org/pub/scm/bluetooth/bluez.git/tree/doc

package bluetooth

import (
	"github.com/muka/go-bluetooth/api"
	"github.com/muka/go-bluetooth/bluez/profile/adapter"
)

type Adapter struct {
	adapter              *adapter.Adapter1
	id                   string
	cancelScan           func()
	defaultAdvertisement *Advertisement
}

// DefaultAdapter is the default adapter on the system. On Linux, it is the
// first adapter available.
//
// Make sure to call Enable() before using it to initialize the adapter.
var DefaultAdapter = &Adapter{}

// Enable configures the BLE stack. It must be called before any
// Bluetooth-related calls (unless otherwise indicated).
func (a *Adapter) Enable() (err error) {
	if a.id == "" {
		a.adapter, err = api.GetDefaultAdapter()
		if err != nil {
			return
		}
		a.id, err = a.adapter.GetAdapterID()
	}
	return nil
}
