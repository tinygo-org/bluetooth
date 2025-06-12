//go:build !baremetal

package bluetooth

import (
	"errors"
	"fmt"
	"strings"
	"sync/atomic"

	"github.com/godbus/dbus/v5"
	"github.com/godbus/dbus/v5/prop"
)

const (
	// Match rule constants for D-Bus signals.
	//
	// See [DBusPropertiesLink] for more information.

	dbusPropertiesChangedInterfaceName = 0
	dbusPropertiesChangedDictionary    = 1
	dbusPropertiesChangedInvalidated   = 2

	dbusInterfacesAddedDictionary = 1

	dbusSignalInterfacesAdded   = "org.freedesktop.DBus.ObjectManager.InterfacesAdded"
	dbusSignalPropertiesChanged = "org.freedesktop.DBus.Properties.PropertiesChanged"

	bluezDevice1Interface = "org.bluez.Device1"
	bluezDevice1Address   = "Address"
	bluezDevice1Connected = "Connected"
)

var (
	// See [DBusPropertiesLink] for more information.
	matchOptionsPropertiesChanged = []dbus.MatchOption{dbus.WithMatchInterface("org.freedesktop.DBus.Properties"),
		dbus.WithMatchMember("PropertiesChanged"),
		dbus.WithMatchArg(dbusPropertiesChangedInterfaceName, "org.bluez.Device1")}

	// See [DBusObjectManagerLink] for more information.
	matchOptionsInterfacesAdded = []dbus.MatchOption{dbus.WithMatchInterface("org.freedesktop.DBus.ObjectManager"),
		dbus.WithMatchMember("InterfacesAdded")}
)

// [DBusPropertiesLink]: https://dbus.freedesktop.org/doc/dbus-specification.html#standard-interfaces-properties
// [DBusObjectManagerLink]: https://dbus.freedesktop.org/doc/dbus-specification.html#standard-interfaces-objectmanager

var errAdvertisementNotStarted = errors.New("bluetooth: advertisement is not started")
var errAdvertisementAlreadyStarted = errors.New("bluetooth: advertisement is already started")
var errAdaptorNotPowered = errors.New("bluetooth: adaptor is not powered")

// Unique ID per advertisement (to generate a unique object path).
var advertisementID uint64

// Address contains a Bluetooth MAC address.
type Address struct {
	MACAddress
}

// Advertisement encapsulates a single advertisement instance.
type Advertisement struct {
	adapter    *Adapter
	properties *prop.Properties
	path       dbus.ObjectPath
	started    bool

	// D-Bus Signals
	sigCh chan *dbus.Signal
}

// DefaultAdvertisement returns the default advertisement instance but does not
// configure it.
func (a *Adapter) DefaultAdvertisement() *Advertisement {
	if a.defaultAdvertisement == nil {
		a.defaultAdvertisement = &Advertisement{
			adapter: a,
		}
	}
	return a.defaultAdvertisement
}

// Configure this advertisement.
//
// On Linux with BlueZ, it is not possible to set the advertisement interval.
func (a *Advertisement) Configure(options AdvertisementOptions) error {
	if a.started {
		return errAdvertisementAlreadyStarted
	}

	var serviceUUIDs []string
	for _, uuid := range options.ServiceUUIDs {
		serviceUUIDs = append(serviceUUIDs, uuid.String())
	}
	var serviceData = make(map[string]interface{})
	for _, element := range options.ServiceData {
		serviceData[element.UUID.String()] = element.Data
	}

	// Convert map[uint16][]byte to map[uint16]any because that's what BlueZ needs.
	manufacturerData := map[uint16]any{}
	for _, element := range options.ManufacturerData {
		manufacturerData[element.CompanyID] = element.Data
	}

	// Build an org.bluez.LEAdvertisement1 object, to be exported over DBus.
	// See:
	// https://git.kernel.org/pub/scm/bluetooth/bluez.git/tree/doc/org.bluez.LEAdvertisement.rst
	id := atomic.AddUint64(&advertisementID, 1)
	a.path = dbus.ObjectPath(fmt.Sprintf("/org/tinygo/bluetooth/advertisement%d", id))
	propsSpec := map[string]map[string]*prop.Prop{
		"org.bluez.LEAdvertisement1": {
			"Type":             {Value: "broadcast"},
			"ServiceUUIDs":     {Value: serviceUUIDs},
			"ManufacturerData": {Value: manufacturerData},
			"LocalName":        {Value: options.LocalName},
			"ServiceData":      {Value: serviceData, Writable: true},
			// The documentation states:
			// > Timeout of the advertisement in seconds. This defines the
			// > lifetime of the advertisement.
			// however, the value 0 also works, and presumably means "no
			// timeout".
			"Timeout": {Value: uint16(0)},
			// TODO: MinInterval and MaxInterval (experimental as of BlueZ 5.71)
		},
	}
	props, err := prop.Export(a.adapter.bus, a.path, propsSpec)
	if err != nil {
		return err
	}
	a.properties = props

	if options.LocalName != "" {
		// In BlueZ AdvertisementOptions.LocalName will be sent in Extended
		// Advertising Data and it will not change the Adapter alias.  Setting
		// this property will update the name in the initial advertising data.
		call := a.adapter.adapter.Call("org.freedesktop.DBus.Properties.Set", 0,
			"org.bluez.Adapter1", "Alias", dbus.MakeVariant(options.LocalName))
		if call.Err != nil {
			return fmt.Errorf("set adapter alias: %w", call.Err)
		}
	}

	return nil
}

// Start advertisement. May only be called after it has been configured.
func (a *Advertisement) Start() error {
	// Register our advertisement object to start advertising.
	err := a.adapter.adapter.Call("org.bluez.LEAdvertisingManager1.RegisterAdvertisement", 0, a.path, map[string]interface{}{}).Err
	if err != nil {
		if err, ok := err.(dbus.Error); ok && err.Name == "org.bluez.Error.AlreadyExists" {
			return errAdvertisementAlreadyStarted
		}
		return fmt.Errorf("bluetooth: could not start advertisement: %w", err)
	}

	if a.adapter.connectHandler != nil {
		a.sigCh = make(chan *dbus.Signal)
		a.adapter.bus.Signal(a.sigCh)

		if err := a.adapter.bus.AddMatchSignal(matchOptionsPropertiesChanged...); err != nil {
			return fmt.Errorf("bluetooth: add dbus match signal: PropertiesChanged: %w", err)
		}

		if err := a.adapter.bus.AddMatchSignal(matchOptionsInterfacesAdded...); err != nil {
			return fmt.Errorf("bluetooth: add dbus match signal: InterfacesAdded: %w", err)
		}

		go a.handleDBusSignals()
	}

	// Make us discoverable.
	err = a.adapter.adapter.SetProperty("org.bluez.Adapter1.Discoverable", dbus.MakeVariant(true))
	if err != nil {
		return fmt.Errorf("bluetooth: could not start advertisement: %w", err)
	}
	a.started = true
	return nil
}

// Stop advertisement. May only be called after it has been started.
func (a *Advertisement) Stop() error {
	err := a.adapter.adapter.Call("org.bluez.LEAdvertisingManager1.UnregisterAdvertisement", 0, a.path).Err
	if err != nil {
		if err, ok := err.(dbus.Error); ok && err.Name == "org.bluez.Error.DoesNotExist" {
			return errAdvertisementNotStarted
		}
		return fmt.Errorf("bluetooth: could not stop advertisement: %w", err)
	}
	a.started = false

	if a.sigCh != nil {
		defer close(a.sigCh)
		if err := a.adapter.bus.RemoveMatchSignal(matchOptionsPropertiesChanged...); err != nil {
			return fmt.Errorf("bluetooth: remove dbus match signal: PropertiesChanged: %w", err)
		}
		if err := a.adapter.bus.RemoveMatchSignal(matchOptionsInterfacesAdded...); err != nil {
			return fmt.Errorf("bluetooth: remove dbus match signal: InterfacesAdded: %w", err)
		}
		a.adapter.bus.RemoveSignal(a.sigCh)
	}
	return nil
}

// Scan starts a BLE scan. It is stopped by a call to StopScan. A common pattern
// is to cancel the scan when a particular device has been found.
//
// On Linux with BlueZ, incoming packets cannot be observed directly. Instead,
// existing devices are watched for property changes. This closely simulates the
// behavior as if the actual packets were observed, but it has flaws: it is
// possible some events are missed and perhaps even possible that some events
// are duplicated.
func (a *Adapter) Scan(callback func(*Adapter, ScanResult)) error {
	if a.scanCancelChan != nil {
		return errScanning
	}

	signal := make(chan *dbus.Signal)
	a.bus.Signal(signal)
	defer a.bus.RemoveSignal(signal)

	propertiesChangedMatchOptions := []dbus.MatchOption{dbus.WithMatchInterface("org.freedesktop.DBus.Properties")}
	a.bus.AddMatchSignal(propertiesChangedMatchOptions...)
	defer a.bus.RemoveMatchSignal(propertiesChangedMatchOptions...)

	newObjectMatchOptions := []dbus.MatchOption{dbus.WithMatchInterface("org.freedesktop.DBus.ObjectManager")}
	a.bus.AddMatchSignal(newObjectMatchOptions...)
	defer a.bus.RemoveMatchSignal(newObjectMatchOptions...)

	// Check if the adapter is powered on.
	powered, err := a.adapter.GetProperty("org.bluez.Adapter1.Powered")
	if err != nil {
		return err
	}
	if !powered.Value().(bool) {
		return errAdaptorNotPowered
	}

	// Channel that will be closed when the scan is stopped.
	// Detecting whether the scan is stopped can be done by doing a non-blocking
	// read from it. If it succeeds, the scan is stopped.
	cancelChan := make(chan struct{})
	a.scanCancelChan = cancelChan

	// This appears to be necessary to receive any BLE discovery results at all.
	defer a.adapter.Call("org.bluez.Adapter1.SetDiscoveryFilter", 0)
	err = a.adapter.Call("org.bluez.Adapter1.SetDiscoveryFilter", 0, map[string]interface{}{
		"Transport": "le",
	}).Err
	if err != nil {
		return err
	}

	// Go through all connected devices and present the connected devices as
	// scan results. Also save the properties so that the full list of
	// properties is known on a PropertiesChanged signal. We can't present the
	// list of cached devices as scan results as devices may be cached for a
	// long time, long after they have moved out of range.
	var deviceList map[dbus.ObjectPath]map[string]map[string]dbus.Variant
	err = a.bluez.Call("org.freedesktop.DBus.ObjectManager.GetManagedObjects", 0).Store(&deviceList)
	if err != nil {
		return err
	}
	devices := make(map[dbus.ObjectPath]map[string]dbus.Variant)
	for path, v := range deviceList {
		device, ok := v["org.bluez.Device1"]
		if !ok {
			continue // not a device
		}
		if !strings.HasPrefix(string(path), string(a.adapter.Path())) {
			continue // not part of our adapter
		}
		if device["Connected"].Value().(bool) {
			callback(a, makeScanResult(device))
			select {
			case <-cancelChan:
				return nil
			default:
			}
		}
		devices[path] = device
	}

	// Instruct BlueZ to start discovering.
	// NOTE: We must call Go here, not Call, because it can block if adapter is
	// powered off, or was recently powered off.
	startDiscovery := a.adapter.Go("org.bluez.Adapter1.StartDiscovery", 0, nil)

	for {
		// Check whether the scan is stopped. This is necessary to avoid a race
		// condition between the signal channel and the cancelScan channel when
		// the callback calls StopScan() (no new callbacks may be called after
		// StopScan is called).
		select {
		case <-cancelChan:
			return a.adapter.Call("org.bluez.Adapter1.StopDiscovery", 0).Err
		default:
		}

		select {
		case <-startDiscovery.Done:
			if startDiscovery.Err != nil {
				close(cancelChan)
				a.scanCancelChan = nil
				return startDiscovery.Err
			}
		case sig := <-signal:
			// This channel receives anything that we watch for, so we'll have
			// to check for signals that are relevant to us.
			switch sig.Name {
			case "org.freedesktop.DBus.ObjectManager.InterfacesAdded":
				objectPath := sig.Body[0].(dbus.ObjectPath)
				interfaces := sig.Body[1].(map[string]map[string]dbus.Variant)
				rawprops, ok := interfaces["org.bluez.Device1"]
				if !ok {
					continue
				}
				devices[objectPath] = rawprops
				callback(a, makeScanResult(rawprops))
			case "org.freedesktop.DBus.Properties.PropertiesChanged":
				interfaceName := sig.Body[0].(string)
				switch interfaceName {
				case "org.bluez.Adapter1":
					// check power state
					changes := sig.Body[1].(map[string]dbus.Variant)
					if powered, ok := changes["Powered"]; ok && !powered.Value().(bool) {
						// adapter is powered off, stop the scan
						close(cancelChan)
						a.scanCancelChan = nil
						return errAdaptorNotPowered
					} else if discovering, ok := changes["Discovering"]; ok && !discovering.Value().(bool) {
						// adapter stopped discovering unexpectedly (e.g. due to external event)
						close(cancelChan)
						a.scanCancelChan = nil
						return errScanStopped
					}

				case "org.bluez.Device1":
					changes := sig.Body[1].(map[string]dbus.Variant)
					device, ok := devices[sig.Path]
					if !ok {
						// This shouldn't happen, but protect against it just in
						// case.
						continue
					}
					for k, v := range changes {
						device[k] = v
					}
					callback(a, makeScanResult(device))

				default:
					continue
				}
			}
		case <-cancelChan:
			continue
		}
	}

	// unreachable
}

// StopScan stops any in-progress scan. It can be called from within a Scan
// callback to stop the current scan. If no scan is in progress, an error will
// be returned.
func (a *Adapter) StopScan() error {
	if a.scanCancelChan == nil {
		return errNotScanning
	}
	close(a.scanCancelChan)
	a.scanCancelChan = nil
	return nil
}

// makeScanResult creates a ScanResult from a raw DBus device.
func makeScanResult(props map[string]dbus.Variant) ScanResult {
	// Assume the Address property is well-formed.
	addr, _ := ParseMAC(props["Address"].Value().(string))

	// Create a list of UUIDs.
	var serviceUUIDs []UUID
	for _, uuid := range props["UUIDs"].Value().([]string) {
		// Assume the UUID is well-formed.
		parsedUUID, _ := ParseUUID(uuid)
		serviceUUIDs = append(serviceUUIDs, parsedUUID)
	}

	a := Address{MACAddress{MAC: addr}}
	a.SetRandom(props["AddressType"].Value().(string) == "random")

	var manufacturerData []ManufacturerDataElement
	if mdata, ok := props["ManufacturerData"].Value().(map[uint16]dbus.Variant); ok {
		for k, v := range mdata {
			manufacturerData = append(manufacturerData, ManufacturerDataElement{
				CompanyID: k,
				Data:      v.Value().([]byte),
			})
		}
	}

	// Get optional properties.
	localName, _ := props["Name"].Value().(string)
	rssi, _ := props["RSSI"].Value().(int16)

	var serviceData []ServiceDataElement
	if sdata, ok := props["ServiceData"].Value().(map[string]dbus.Variant); ok {
		for k, v := range sdata {
			uuid, err := ParseUUID(k)
			if err != nil {
				continue
			}
			serviceData = append(serviceData, ServiceDataElement{
				UUID: uuid,
				Data: v.Value().([]byte),
			})
		}
	}

	return ScanResult{
		RSSI:    rssi,
		Address: a,
		AdvertisementPayload: &advertisementFields{
			AdvertisementFields{
				LocalName:        localName,
				ServiceUUIDs:     serviceUUIDs,
				ManufacturerData: manufacturerData,
				ServiceData:      serviceData,
			},
		},
	}
}

// Device is a connection to a remote bluetooth device.
type Device struct {
	Address Address // the MAC address of the device

	device  dbus.BusObject // bluez device interface
	adapter *Adapter       // the adapter that was used to form this device connection
}

// Connect starts a connection attempt to the given peripheral device address.
//
// On Linux and Windows, the IsRandom part of the address is ignored.
func (a *Adapter) Connect(address Address, params ConnectionParams) (Device, error) {
	devicePath := dbus.ObjectPath(string(a.adapter.Path()) + "/dev_" + strings.Replace(address.MAC.String(), ":", "_", -1))
	device := Device{
		Address: address,
		device:  a.bus.Object("org.bluez", devicePath),
		adapter: a,
	}

	// Already start watching for property changes. We do this before reading
	// the Connected property below to avoid a race condition: if the device
	// were connected between the two calls the signal wouldn't be picked up.
	signal := make(chan *dbus.Signal)
	a.bus.Signal(signal)
	defer close(signal)
	defer a.bus.RemoveSignal(signal)
	propertiesChangedMatchOptions := []dbus.MatchOption{dbus.WithMatchInterface("org.freedesktop.DBus.Properties")}
	a.bus.AddMatchSignal(propertiesChangedMatchOptions...)
	defer a.bus.RemoveMatchSignal(propertiesChangedMatchOptions...)

	powered, err := a.adapter.GetProperty("org.bluez.Adapter1.Powered")
	if err != nil {
		return Device{}, err
	}
	if !powered.Value().(bool) {
		return Device{}, errAdaptorNotPowered
	}

	// Read whether this device is already connected.
	connected, err := device.device.GetProperty("org.bluez.Device1.Connected")
	if err != nil {
		return Device{}, err
	}

	// Connect to the device, if not already connected.
	if !connected.Value().(bool) {
		// Start connecting (async).
		err := device.device.Call("org.bluez.Device1.Connect", 0).Err
		if err != nil {
			return Device{}, fmt.Errorf("bluetooth: failed to connect: %w", err)
		}

		// Wait until the device has connected.
		connectChan := make(chan struct{})
		go func() {
			for sig := range signal {
				switch sig.Name {
				case "org.freedesktop.DBus.Properties.PropertiesChanged":
					interfaceName := sig.Body[0].(string)
					switch interfaceName {
					case "org.bluez.Adapter1":
						// check power state
						changes := sig.Body[1].(map[string]dbus.Variant)
						for k, v := range changes {
							if k == "Powered" && !v.Value().(bool) {
								// adapter is powered off, stop the scan
								err = errAdaptorNotPowered
								close(connectChan)
							}
						}
					case "org.bluez.Device1":
						if sig.Path != device.device.Path() {
							continue
						}
						changes := sig.Body[1].(map[string]dbus.Variant)
						if connected, ok := changes["Connected"].Value().(bool); ok && connected {
							close(connectChan)
						}
					}
				}
			}
		}()
		<-connectChan

		if err != nil {
			return Device{}, err
		}
	}

	if a.connectHandler != nil {
		a.connectHandler(device, true)
	}

	return device, nil
}

// Disconnect from the BLE device. This method is non-blocking and does not
// wait until the connection is fully gone.
func (d Device) Disconnect() error {
	if d.adapter.connectHandler != nil {
		d.adapter.connectHandler(d, false)
	}

	// we don't call our cancel function here, instead we wait for the
	// property change in `watchForConnect` and cancel things then
	return d.device.Call("org.bluez.Device1.Disconnect", 0).Err
}

// RequestConnectionParams requests a different connection latency and timeout
// of the given device connection. Fields that are unset will be left alone.
// Whether or not the device will actually honor this, depends on the device and
// on the specific parameters.
//
// On Linux, this call doesn't do anything because BlueZ doesn't support
// changing the connection latency.
func (d Device) RequestConnectionParams(params ConnectionParams) error {
	return nil
}

// SetRandomAddress sets the random address to be used for advertising.
func (a *Adapter) SetRandomAddress(mac MAC) error {
	addr, err := a.adapter.GetProperty("org.bluez.Adapter1.Address")
	if err != nil {
		if err, ok := err.(dbus.Error); ok && err.Name == "org.freedesktop.DBus.Error.UnknownObject" {
			return fmt.Errorf("bluetooth: adapter %s does not exist", a.adapter.Path())
		}
		return fmt.Errorf("could not get adapter address: %w", err)
	}
	a.address = mac.String()
	if err := addr.Store(&a.address); err != nil {
		return fmt.Errorf("could not set adapter address: %w", err)
	}

	if err := a.adapter.SetProperty("org.bluez.Adapter1.AddressType", "random"); err != nil {
		return fmt.Errorf("could not set adapter address type: %w", err)
	}

	return nil
}

func (a *Advertisement) handleDBusSignals() {
	for {
		select {
		case sig, ok := <-a.sigCh:
			if !ok {
				return // channel closed
			}

			device := Device{
				device:  a.adapter.bus.Object("org.bluez", sig.Path),
				adapter: a.adapter,
			}

			switch sig.Name {
			case dbusSignalInterfacesAdded:
				interfaces := sig.Body[dbusInterfacesAddedDictionary].(map[string]map[string]dbus.Variant)

				// InterfacesAdded signal also contains all known properties so
				// so we do not need to call org.freedesktop.DBus.Properties.GetAll
				props, ok := interfaces[bluezDevice1Interface]
				if !ok {
					continue
				}

				if err := device.parseProperties(&props); err != nil {
					continue
				}

				if connected, ok := props[bluezDevice1Connected].Value().(bool); ok {
					a.adapter.connectHandler(device, connected)
				}
			case dbusSignalPropertiesChanged:
				// Skip any signals that are not the Device1 interface.
				if interfaceName, ok := sig.Body[dbusPropertiesChangedInterfaceName].(string); !ok || interfaceName != bluezDevice1Interface {
					continue
				}

				// Get all changed properties and skip any signals that are not
				// compliant with the Device1 interface.
				changes, ok := sig.Body[dbusPropertiesChangedDictionary].(map[string]dbus.Variant)
				if !ok {
					continue
				}

				// Call the connect handler if the Connected property has changed.
				if connected, ok := changes[bluezDevice1Connected].Value().(bool); ok {
					// The only property received is the changed property "Connected",
					// so we have to get the other properties from D-Bus.
					var props map[string]dbus.Variant
					if err := device.device.Call("org.freedesktop.DBus.Properties.GetAll",
						0,
						bluezDevice1Interface).Store(&props); err != nil {
						continue
					}

					if err := device.parseProperties(&props); err != nil {
						continue
					}

					a.adapter.connectHandler(device, connected)
				}
			}
		}
	}
}

// parseProperties will set fields from provided properties
//
// For all possible properties see:
// https://github.com/luetzel/bluez/blob/master/doc/device-api.txt
func (d *Device) parseProperties(props *map[string]dbus.Variant) error {
	for prop, v := range *props {
		switch prop {
		case bluezDevice1Address:
			if addrStr, ok := v.Value().(string); ok {
				mac, err := ParseMAC(addrStr)
				if err != nil {
					return fmt.Errorf("ParseMAC: %w", err)
				}
				d.Address = Address{MACAddress: MACAddress{MAC: mac}}
			}
		}
	}

	return nil
}
