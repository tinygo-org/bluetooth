//go:build !baremetal
// +build !baremetal

package bluetooth

import (
	"strings"

	"github.com/godbus/dbus/v5"
	"github.com/muka/go-bluetooth/api"
	"github.com/muka/go-bluetooth/bluez/profile/advertising"
	"github.com/muka/go-bluetooth/bluez/profile/device"
)

// Address contains a Bluetooth MAC address.
type Address struct {
	MACAddress
}

// Advertisement encapsulates a single advertisement instance.
type Advertisement struct {
	adapter       *Adapter
	advertisement *api.Advertisement
	properties    *advertising.LEAdvertisement1Properties
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
	if a.advertisement != nil {
		panic("todo: configure advertisement a second time")
	}

	a.properties = &advertising.LEAdvertisement1Properties{
		Type:      advertising.AdvertisementTypeBroadcast,
		Timeout:   1<<16 - 1,
		LocalName: options.LocalName,
	}
	for _, uuid := range options.ServiceUUIDs {
		a.properties.ServiceUUIDs = append(a.properties.ServiceUUIDs, uuid.String())
	}

	return nil
}

// Start advertisement. May only be called after it has been configured.
func (a *Advertisement) Start() error {
	if a.advertisement != nil {
		panic("todo: start advertisement a second time")
	}
	_, err := api.ExposeAdvertisement(a.adapter.id, a.properties, uint32(a.properties.Timeout))
	if err != nil {
		return err
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
	if a.cancelChan != nil {
		return errScanning
	}

	// Channel that will be closed when the scan is stopped.
	// Detecting whether the scan is stopped can be done by doing a non-blocking
	// read from it. If it succeeds, the scan is stopped.
	cancelChan := make(chan struct{})
	a.cancelChan = cancelChan

	// This appears to be necessary to receive any BLE discovery results at all.
	defer a.adapter.SetDiscoveryFilter(nil)
	err := a.adapter.SetDiscoveryFilter(map[string]interface{}{
		"Transport": "le",
	})
	if err != nil {
		return err
	}

	bus, err := dbus.SystemBus()
	if err != nil {
		return err
	}

	signal := make(chan *dbus.Signal)
	bus.Signal(signal)
	defer bus.RemoveSignal(signal)

	propertiesChangedMatchOptions := []dbus.MatchOption{dbus.WithMatchInterface("org.freedesktop.DBus.Properties")}
	bus.AddMatchSignal(propertiesChangedMatchOptions...)
	defer bus.RemoveMatchSignal(propertiesChangedMatchOptions...)

	newObjectMatchOptions := []dbus.MatchOption{dbus.WithMatchInterface("org.freedesktop.DBus.ObjectManager")}
	bus.AddMatchSignal(newObjectMatchOptions...)
	defer bus.RemoveMatchSignal(newObjectMatchOptions...)

	// Go through all connected devices and present the connected devices as
	// scan results. Also save the properties so that the full list of
	// properties is known on a PropertiesChanged signal. We can't present the
	// list of cached devices as scan results as devices may be cached for a
	// long time, long after they have moved out of range.
	deviceList, err := a.adapter.GetDevices()
	if err != nil {
		return err
	}
	devices := make(map[dbus.ObjectPath]*device.Device1Properties)
	for _, dev := range deviceList {
		if dev.Properties.Connected {
			callback(a, makeScanResult(dev.Properties))
			select {
			case <-cancelChan:
				return nil
			default:
			}
		}
		devices[dev.Path()] = dev.Properties
	}

	// Instruct BlueZ to start discovering.
	err = a.adapter.StartDiscovery()
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
			a.adapter.StopDiscovery()
			return nil
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
				var props *device.Device1Properties
				props, _ = props.FromDBusMap(rawprops)
				devices[objectPath] = props
				callback(a, makeScanResult(props))
			case "org.freedesktop.DBus.Properties.PropertiesChanged":
				interfaceName := sig.Body[0].(string)
				if interfaceName != "org.bluez.Device1" {
					continue
				}
				changes := sig.Body[1].(map[string]dbus.Variant)
				props := devices[sig.Path]
				for field, val := range changes {
					switch field {
					case "RSSI":
						props.RSSI = val.Value().(int16)
					case "Name":
						props.Name = val.Value().(string)
					case "UUIDs":
						props.UUIDs = val.Value().([]string)
					}
				}
				callback(a, makeScanResult(props))
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
	if a.cancelChan == nil {
		return errNotScanning
	}
	close(a.cancelChan)
	a.cancelChan = nil
	return nil
}

// makeScanResult creates a ScanResult from a Device1 object.
func makeScanResult(props *device.Device1Properties) ScanResult {
	// Assume the Address property is well-formed.
	addr, _ := ParseMAC(props.Address)

	// Create a list of UUIDs.
	var serviceUUIDs []UUID
	for _, uuid := range props.UUIDs {
		// Assume the UUID is well-formed.
		parsedUUID, _ := ParseUUID(uuid)
		serviceUUIDs = append(serviceUUIDs, parsedUUID)
	}

	a := Address{MACAddress{MAC: addr}}
	a.SetRandom(props.AddressType == "random")

	return ScanResult{
		RSSI:    props.RSSI,
		Address: a,
		AdvertisementPayload: &advertisementFields{
			AdvertisementFields{
				LocalName:    props.Name,
				ServiceUUIDs: serviceUUIDs,
			},
		},
	}
}

// Device is a connection to a remote peripheral.
type Device struct {
	device *device.Device1
}

// Connect starts a connection attempt to the given peripheral device address.
//
// On Linux and Windows, the IsRandom part of the address is ignored.
func (a *Adapter) Connect(address Addresser, params ConnectionParams) (*Device, error) {
	adr := address.(Address)
	devicePath := dbus.ObjectPath(string(a.adapter.Path()) + "/dev_" + strings.Replace(adr.MAC.String(), ":", "_", -1))
	dev, err := device.NewDevice1(devicePath)
	if err != nil {
		return nil, err
	}

	if !dev.Properties.Connected {
		// Not yet connected, so do it now.
		// The properties have just been read so this is fresh data.
		err := dev.Connect()
		if err != nil {
			return nil, err
		}
	}

	// TODO: a proper async callback.
	a.connectHandler(nil, true)

	return &Device{
		device: dev,
	}, nil
}

// Disconnect from the BLE device. This method is non-blocking and does not
// wait until the connection is fully gone.
func (d *Device) Disconnect() error {
	return d.device.Disconnect()
}
