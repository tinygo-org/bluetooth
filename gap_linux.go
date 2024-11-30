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

var errAdvertisementNotStarted = errors.New("bluetooth: stop advertisement that was not started")
var errAdvertisementAlreadyStarted = errors.New("bluetooth: start advertisement that was already started")

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
	if a.properties != nil {
		panic("todo: configure advertisement a second time")
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

	// Make us discoverable.
	err = a.adapter.adapter.SetProperty("org.bluez.Adapter1.Discoverable", dbus.MakeVariant(true))
	if err != nil {
		return fmt.Errorf("bluetooth: could not start advertisement: %w", err)
	}
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

	// Channel that will be closed when the scan is stopped.
	// Detecting whether the scan is stopped can be done by doing a non-blocking
	// read from it. If it succeeds, the scan is stopped.
	cancelChan := make(chan struct{})
	a.scanCancelChan = cancelChan

	// This appears to be necessary to receive any BLE discovery results at all.
	defer a.adapter.Call("org.bluez.Adapter1.SetDiscoveryFilter", 0)
	err := a.adapter.Call("org.bluez.Adapter1.SetDiscoveryFilter", 0, map[string]interface{}{
		"Transport": "le",
	}).Err
	if err != nil {
		return err
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
	err = a.adapter.Call("org.bluez.Adapter1.StartDiscovery", 0).Err
	if err != nil {
		return err
	}

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
				if interfaceName != "org.bluez.Device1" {
					continue
				}
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

// Device is a connection to a remote peripheral.
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
	defer a.bus.RemoveSignal(signal)
	propertiesChangedMatchOptions := []dbus.MatchOption{dbus.WithMatchInterface("org.freedesktop.DBus.Properties")}
	a.bus.AddMatchSignal(propertiesChangedMatchOptions...)
	defer a.bus.RemoveMatchSignal(propertiesChangedMatchOptions...)

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
					if interfaceName != "org.bluez.Device1" {
						continue
					}
					if sig.Path != device.device.Path() {
						continue
					}
					changes := sig.Body[1].(map[string]dbus.Variant)
					if connected, ok := changes["Connected"].Value().(bool); ok && connected {
						close(connectChan)
					}
				}
			}
		}()
		<-connectChan
	}

	return device, nil
}

// Disconnect from the BLE device. This method is non-blocking and does not
// wait until the connection is fully gone.
func (d Device) Disconnect() error {
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
