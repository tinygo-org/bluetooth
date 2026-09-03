//go:build !baremetal

// Some documentation for the BlueZ D-Bus interface:
// https://git.kernel.org/pub/scm/bluetooth/bluez.git/tree/doc

package bluetooth

import (
	"errors"
	"fmt"

	"github.com/godbus/dbus/v5"
)

const defaultAdapter = "hci0"

var _ BLEAdapter = (*Adapter)(nil)

type Adapter struct {
	id                   string
	scanCancelChan       chan struct{}
	bus                  *dbus.Conn
	bluez                dbus.BusObject // object at /
	adapter              dbus.BusObject // object at /org/bluez/hciX
	address              string
	defaultAdvertisement *Advertisement

	connectHandler     func(device Device, connected bool)
	stateChangeHandler func(newState AdapterState)
}

// NewAdapter creates a new Adapter with the given ID.
//
// Make sure to call Enable() before using it to initialize the adapter.
func NewAdapter(id string) *Adapter {
	return &Adapter{
		id:                 id,
		connectHandler:     func(device Device, connected bool) {},
		stateChangeHandler: func(newState AdapterState) {},
	}
}

// DefaultAdapter is the default adapter on the system. On Linux, it is the
// first adapter available.
//
// Make sure to call Enable() before using it to initialize the adapter.
var DefaultAdapter = NewAdapter(defaultAdapter)

// Enable configures the BLE stack. It must be called before any
// Bluetooth-related calls (unless otherwise indicated).
func (a *Adapter) Enable() (err error) {
	bus, err := dbus.SystemBus()
	if err != nil {
		return err
	}
	a.bus = bus
	a.bluez = a.bus.Object("org.bluez", dbus.ObjectPath("/"))
	a.adapter = a.bus.Object("org.bluez", dbus.ObjectPath("/org/bluez/"+a.id))
	addr, err := a.adapter.GetProperty("org.bluez.Adapter1.Address")
	if err != nil {
		if err, ok := err.(dbus.Error); ok && err.Name == "org.freedesktop.DBus.Error.UnknownObject" {
			return fmt.Errorf("bluetooth: adapter %s does not exist", a.adapter.Path())
		}
		return fmt.Errorf("could not activate BlueZ adapter: %w", err)
	}
	addr.Store(&a.address)

	if err := a.watchForStateChange(); err != nil {
		return err
	}
	return nil
}

// Reset clears BlueZ state so a subsequent Enable() rebuilds it.
// Mostly a no-op on Linux; provided for interface symmetry.
func (a *Adapter) Reset() error {
	a.bus = nil
	a.bluez = nil
	a.adapter = nil
	a.address = ""
	a.scanCancelChan = nil
	return nil
}

func (a *Adapter) Address() (MACAddress, error) {
	if a.address == "" {
		return MACAddress{}, errors.New("adapter not enabled")
	}
	mac, err := ParseMAC(a.address)
	if err != nil {
		return MACAddress{}, err
	}
	return MACAddress{MAC: mac}, nil
}

// SetStateChangeHandler sets a handler function to be called whenever the adaptor's
// state changes.
func (a *Adapter) SetStateChangeHandler(c func(newState AdapterState)) {
	a.stateChangeHandler = c
}

// State returns the current state of the adapter.
func (a *Adapter) State() AdapterState {
	if a.adapter == nil {
		return AdapterStateUnknown
	}

	prop, err := a.adapter.GetProperty("org.bluez.Adapter1.Powered")
	if err != nil {
		return AdapterStateUnknown
	}
	powered, ok := prop.Value().(bool)
	if !ok {
		return AdapterStateUnknown
	}
	if powered {
		return AdapterStatePoweredOn
	}
	return AdapterStatePoweredOff
}

// watchForStateChange watches for D-Bus PropertiesChanged signals from the
// BlueZ adapter interface and reports Powered/Unpowered transitions to the
// registered state-change handler.
//
// We can watch for extra adapter properties here, see
// https://git.kernel.org/pub/scm/bluetooth/bluez.git/tree/doc/org.bluez.Adapter.rst
// for the full list.
func (a *Adapter) watchForStateChange() error {
	matchOptions := []dbus.MatchOption{
		dbus.WithMatchInterface("org.freedesktop.DBus.Properties"),
		dbus.WithMatchMember("PropertiesChanged"),
		dbus.WithMatchArg(dbusPropertiesChangedInterfaceName, "org.bluez.Adapter1"),
	}
	if err := a.bus.AddMatchSignal(matchOptions...); err != nil {
		return err
	}

	signal := make(chan *dbus.Signal)
	a.bus.Signal(signal)

	go func() {
		for sig := range signal {
			if sig.Name != dbusSignalPropertiesChanged {
				continue
			}
			// Only react to property changes on the adapter interface.
			if interfaceName, ok := sig.Body[dbusPropertiesChangedInterfaceName].(string); !ok || interfaceName != "org.bluez.Adapter1" {
				continue
			}
			changes, ok := sig.Body[dbusPropertiesChangedDictionary].(map[string]dbus.Variant)
			if !ok {
				continue
			}
			if powered, ok := changes["Powered"].Value().(bool); ok {
				if powered {
					a.stateChangeHandler(AdapterStatePoweredOn)
				} else {
					a.stateChangeHandler(AdapterStatePoweredOff)
				}
			}
		}
	}()

	return nil
}
