package bluetooth

import (
	"errors"
	"syscall/js"
)

// Address contains a Bluetooth address which on WASM is an opaque device ID
// string, since WebBluetooth does not expose MAC addresses.
type Address struct {
	// The opaque device identifier provided by the browser.
	ID string
}

// IsRandom is not applicable for WebBluetooth.
func (ad Address) IsRandom() bool {
	return false
}

// SetRandom is not applicable for WebBluetooth.
func (ad *Address) SetRandom(val bool) {
}

// Set the address from a device ID string.
func (ad *Address) Set(val string) {
	ad.ID = val
}

// String returns the device ID as a string.
func (ad Address) String() string {
	return ad.ID
}

// jsDeviceMap stores BluetoothDevice JS objects keyed by device ID so they can
// be retrieved when connecting after a scan.
var jsDeviceMap = map[string]js.Value{}

// jsDisconnectListenerMap stores gattserverdisconnected listeners keyed by
// device ID so listeners can be replaced on reconnect and released on
// disconnect.
var jsDisconnectListenerMap = map[string]js.Func{}

var _ GAPDevice = Device{}

// Device is a connection to a remote peripheral via WebBluetooth.
type Device struct {
	Address Address

	device js.Value // BluetoothDevice
	server js.Value // BluetoothRemoteGATTServer
}

// Scan starts a BLE scan by opening the browser's device request dialog.
//
// Note: WebBluetooth does not support continuous background scanning.
// This opens a browser-level device picker. The callback is called once
// for the selected device. Call StopScan from the callback to finish scanning.
//
// The function blocks until StopScan is called or a device is selected.
func (a *Adapter) Scan(callback func(*Adapter, ScanResult)) error {
	if callback == nil {
		return errors.New("bluetooth: must provide callback to Scan function")
	}

	// WebBluetooth requires requestDevice which shows a picker.
	options := js.Global().Get("Object").New()
	options.Set("acceptAllDevices", true)

	// Pass requested services as optionalServices so they can be accessed
	// after connecting. Without this, DiscoverServices will fail with a
	// SecurityError.
	if len(a.RequestedServices) > 0 {
		svcs := js.Global().Get("Array").New(len(a.RequestedServices))
		for i, uuid := range a.RequestedServices {
			svcs.SetIndex(i, js.ValueOf(uuid.String()))
		}
		options.Set("optionalServices", svcs)
	}

	promise := a.bluetooth.Call("requestDevice", options)
	jsDevice, err := await(promise)
	if err != nil {
		return err
	}

	deviceID := jsDevice.Get("id").String()
	name := ""
	if n := jsDevice.Get("name"); !n.IsUndefined() {
		name = n.String()
	}

	// Store the JS device object so it can be retrieved during Connect.
	jsDeviceMap[deviceID] = jsDevice

	callback(a, ScanResult{
		Address: Address{ID: deviceID},
		RSSI:    0,
		AdvertisementPayload: &advertisementFields{
			AdvertisementFields{
				LocalName: name,
			},
		},
	})

	return nil
}

// StopScan stops any in-progress scan. For WebBluetooth this is a no-op
// since requestDevice completes after the user selects a device.
func (a *Adapter) StopScan() error {
	return nil
}

// Connect starts a connection attempt to the given peripheral device address.
//
// On WebBluetooth, the device must have been discovered via Scan first, so the
// BluetoothDevice JS object is available.
func (a *Adapter) Connect(address Address, params ConnectionParams) (Device, error) {
	jsDevice, ok := jsDeviceMap[address.ID]
	if !ok {
		return Device{}, errors.New("bluetooth: device not found, must call Scan first on WASM")
	}

	gatt := jsDevice.Get("gatt")
	if gatt.IsUndefined() {
		return Device{}, errors.New("bluetooth: device does not support GATT")
	}

	server, err := await(gatt.Call("connect"))
	if err != nil {
		return Device{}, err
	}

	d := Device{
		Address: address,
		device:  jsDevice,
		server:  server,
	}

	a.setDisconnectHandler(d)
	a.connectHandler(d, true)

	return d, nil
}

func (a *Adapter) setDisconnectHandler(d Device) {
	deviceID := d.Address.ID
	if listener, ok := jsDisconnectListenerMap[deviceID]; ok {
		d.device.Call("removeEventListener", "gattserverdisconnected", listener)
		listener.Release()
		delete(jsDisconnectListenerMap, deviceID)
	}

	var listener js.Func
	listener = js.FuncOf(func(this js.Value, args []js.Value) any {
		device := Device{
			Address: d.Address,
			device:  d.device,
			server:  d.device.Get("gatt"),
		}

		d.device.Call("removeEventListener", "gattserverdisconnected", listener)
		listener.Release()
		delete(jsDisconnectListenerMap, deviceID)

		a.connectHandler(device, false)
		return nil
	})

	jsDisconnectListenerMap[deviceID] = listener
	d.device.Call("addEventListener", "gattserverdisconnected", listener)
}

// Disconnect from the BLE device.
func (d Device) Disconnect() error {
	if !d.server.IsUndefined() && d.server.Get("connected").Bool() {
		d.server.Call("disconnect")
	}
	return nil
}

// Connected returns whether the device is currently connected.
func (d Device) Connected() (bool, error) {
	if d.server.IsUndefined() {
		return false, nil
	}
	return d.server.Get("connected").Bool(), nil
}

// RequestConnectionParams requests a different connection latency and timeout.
//
// This is not supported by WebBluetooth and is a no-op.
func (d Device) RequestConnectionParams(params ConnectionParams) error {
	return nil
}
