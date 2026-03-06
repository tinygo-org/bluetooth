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

type Adapter struct {
	id                   string
	scanCancelChan       chan struct{}
	bus                  *dbus.Conn
	bluez                dbus.BusObject // object at /
	adapter              dbus.BusObject // object at /org/bluez/hciX
	address              string
	defaultAdvertisement *Advertisement

	connectHandler        func(device Device, connected bool)
	connectionMonitorChan chan *dbus.Signal
	monitoringConnections bool
	stopMonitorChan       chan struct{}
}

// NewAdapter creates a new Adapter with the given ID.
//
// Make sure to call Enable() before using it to initialize the adapter.
func NewAdapter(id string) *Adapter {
	return &Adapter{
		id:             id,
		connectHandler: func(device Device, connected bool) {},
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

	// Start connection monitoring if connectHandler is set
	if a.connectHandler != nil {
		a.startConnectionMonitoring()
	}

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

func (a *Adapter) startConnectionMonitoring() error {
	if a.monitoringConnections {
		return nil // already monitoring
	}

	a.connectionMonitorChan = make(chan *dbus.Signal, 10)
	a.stopMonitorChan = make(chan struct{})
	a.bus.Signal(a.connectionMonitorChan)

	if err := a.bus.AddMatchSignal(matchOptionsPropertiesChanged...); err != nil {
		return fmt.Errorf("bluetooth: add dbus match signal: %w", err)
	}

	a.monitoringConnections = true
	go a.handleConnectionSignals()

	return nil
}

func (a *Adapter) handleConnectionSignals() {
	for {
		select {
		case <-a.stopMonitorChan:
			return
		case sig, ok := <-a.connectionMonitorChan:
			if !ok {
				return // channel was closed
			}
			if sig.Name != dbusSignalPropertiesChanged {
				continue
			}
			interfaceName, ok := sig.Body[0].(string)
			if !ok || interfaceName != bluezDevice1Interface {
				continue
			}
			changes, ok := sig.Body[1].(map[string]dbus.Variant)
			if !ok {
				continue
			}
			if connectedVariant, ok := changes["Connected"]; ok {
				connected, ok := connectedVariant.Value().(bool)
				if !ok {
					continue
				}
				device := Device{
					device:  a.bus.Object("org.bluez", sig.Path),
					adapter: a,
				}
				var props map[string]dbus.Variant
				if err := device.device.Call("org.freedesktop.DBus.Properties.GetAll",
					0, bluezDevice1Interface).Store(&props); err != nil {
					continue
				}
				if err := device.parseProperties(&props); err != nil {
					continue
				}
				a.connectHandler(device, connected)
			}
		}
	}
}

func (a *Adapter) StopConnectionMonitoring() error {
	if !a.monitoringConnections {
		return nil
	}
	// Unregister the signal channel from D-Bus before closing
	a.bus.RemoveSignal(a.connectionMonitorChan)
	if err := a.bus.RemoveMatchSignal(matchOptionsPropertiesChanged...); err != nil {
		return fmt.Errorf("bluetooth: remove dbus match signal: %w", err)
	}
	close(a.stopMonitorChan)
	close(a.connectionMonitorChan)
	a.monitoringConnections = false
	return nil
}
