//go:build !baremetal

// Some documentation for the BlueZ D-Bus interface:
// https://git.kernel.org/pub/scm/bluetooth/bluez.git/tree/doc

package bluetooth

import (
	"context"
	"errors"

	"github.com/muka/go-bluetooth/api"
	"github.com/muka/go-bluetooth/bluez"
	"github.com/muka/go-bluetooth/bluez/profile/adapter"
)

type Adapter struct {
	adapter              *adapter.Adapter1
	id                   string
	cancelChan           chan struct{}
	defaultAdvertisement *Advertisement

	ctx         context.Context             // context for our event watcher, canceled on power off event
	cancel      context.CancelFunc          // cancel function to halt our event watcher context
	propchanged chan *bluez.PropertyChanged // channel that adapter property changes will show up on

	connectHandler     func(device Address, connected bool)
	stateChangeHandler func(newState AdapterState)
}

// DefaultAdapter is the default adapter on the system. On Linux, it is the
// first adapter available.
//
// Make sure to call Enable() before using it to initialize the adapter.
var DefaultAdapter = &Adapter{
	connectHandler: func(device Address, connected bool) {
		return
	},
	stateChangeHandler: func(newState AdapterState) {
		return
	},
}

// Enable configures the BLE stack. It must be called before any
// Bluetooth-related calls (unless otherwise indicated).
func (a *Adapter) Enable() (err error) {
	if a.id == "" {
		a.adapter, err = api.GetDefaultAdapter()
		if err != nil {
			return
		}
		a.id, err = a.adapter.GetAdapterID()
		a.ctx, a.cancel = context.WithCancel(context.Background())
		a.watchForStateChange()
	}
	return nil
}

func (a *Adapter) Address() (MACAddress, error) {
	if a.adapter == nil {
		return MACAddress{}, errors.New("adapter not enabled")
	}
	mac, err := ParseMAC(a.adapter.Properties.Address)
	if err != nil {
		return MACAddress{}, err
	}
	return MACAddress{MAC: mac}, nil
}

func (a *Adapter) watchForStateChange() error {
	var err error
	a.propchanged, err = a.adapter.WatchProperties()
	if err != nil {
		return err
	}

	go func() {
		for {
			select {
			case changed := <-a.propchanged:
				// we will receive a nil if bluez.UnwatchProperties(a, ch) is called, if so we can stop watching
				if changed == nil {
					a.cancel()
					return
				}
				switch changed.Name {
				case "Powered":
					if changed.Value.(bool) {
						a.stateChangeHandler(AdapterStatePoweredOn)
					} else {
						a.stateChangeHandler(AdapterStatePoweredOff)
					}
				}

				continue
			case <-a.ctx.Done():
				return
			}
		}
	}()

	return nil
}
