// +build !baremetal

package bluetooth

import (
	"github.com/muka/go-bluetooth/api"
	"github.com/muka/go-bluetooth/bluez/profile/adapter"
)

type Adapter struct {
	adapter    *adapter.Adapter1
	id         string
	handler    func(Event)
	cancelScan func()
}

// DefaultAdapter returns the default adapter on the current system. On Linux,
// it will return the first adapter available.
func DefaultAdapter() (*Adapter, error) {
	adapter, err := api.GetDefaultAdapter()
	if err != nil {
		return nil, err
	}
	adapterID, err := adapter.GetAdapterID()
	if err != nil {
		return nil, err
	}
	return &Adapter{
		adapter: adapter,
		id:      adapterID,
	}, nil
}

// Enable configures the BLE stack. It must be called before any
// Bluetooth-related calls (unless otherwise indicated).
//
// The Linux implementation is a no-op.
func (a *Adapter) Enable() error {
	return nil
}
