package bluetooth

import (
	"errors"
	"syscall/js"
)

var _ BLEAdapter = (*Adapter)(nil)

// Adapter represents the WebBluetooth adapter accessed via navigator.bluetooth.
type Adapter struct {
	bluetooth      js.Value
	connectHandler func(device Device, connected bool)

	// devices holds the BluetoothDevice objects returned by Scan, keyed by
	// device ID, because Connect needs them again.
	devices map[string]js.Value

	// disconnectListeners holds the gattserverdisconnected listeners, keyed
	// by device ID.
	disconnectListeners map[string]js.Func

	scanning bool

	// RequestedServices is the list of service UUIDs to give as
	// optionalServices to navigator.bluetooth.requestDevice().
	//
	// The browser refuses access to a service that is not in this list. Set
	// this before you call Scan.
	RequestedServices []UUID
}

// DefaultAdapter is the default adapter using the navigator.bluetooth API.
//
// Make sure to call Enable() before using it to initialize the adapter.
var DefaultAdapter = &Adapter{
	connectHandler: func(device Device, connected bool) {},
}

// Enable configures the BLE stack. It must be called before any
// Bluetooth-related calls (unless otherwise indicated).
func (a *Adapter) Enable() error {
	navigator := js.Global().Get("navigator")
	if navigator.IsUndefined() {
		return errors.New("bluetooth: navigator is not available")
	}
	bt := navigator.Get("bluetooth")
	if bt.IsUndefined() {
		return errors.New("bluetooth: WebBluetooth is not supported in this browser")
	}
	a.bluetooth = bt

	// Keep the known devices when Enable runs more than once.
	if a.devices == nil {
		a.devices = map[string]js.Value{}
		a.disconnectListeners = map[string]js.Func{}
	}
	return nil
}
