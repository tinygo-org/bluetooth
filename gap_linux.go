//go:build !baremetal
// +build !baremetal

package bluetooth

import (
	"context"
	"errors"
	"strings"

	"github.com/godbus/dbus/v5"
	"github.com/muka/go-bluetooth/api"
	"github.com/muka/go-bluetooth/bluez"
	"github.com/muka/go-bluetooth/bluez/profile/advertising"
	"github.com/muka/go-bluetooth/bluez/profile/device"
)

var errAdvertisementNotStarted = errors.New("bluetooth: stop advertisement that was not started")

// Address contains a Bluetooth MAC address.
type Address struct {
	MACAddress
}

// Advertisement encapsulates a single advertisement instance.
type Advertisement struct {
	adapter       *Adapter
	advertisement *api.Advertisement
	properties    *advertising.LEAdvertisement1Properties
	cancel        func()
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
	cancel, err := api.ExposeAdvertisement(a.adapter.id, a.properties, uint32(a.properties.Timeout))
	if err != nil {
		return err
	}
	a.cancel = cancel
	return nil
}

// Stop advertisement. May only be called after it has been started.
func (a *Advertisement) Stop() error {
	if a.cancel == nil {
		return errAdvertisementNotStarted
	}
	a.cancel()
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
					case "ManufacturerData":
						// work around for https://github.com/muka/go-bluetooth/issues/163
						mData := make(map[uint16]interface{})
						for k, v := range val.Value().(map[uint16]dbus.Variant) {
							mData[k] = v.Value().(interface{})
						}
						props.ManufacturerData = mData
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

	mData := make(map[uint16][]byte)
	for k, v := range props.ManufacturerData {
		// can be either variant or just byte value
		switch val := v.(type) {
		case dbus.Variant:
			mData[k] = val.Value().([]byte)
		case []byte:
			mData[k] = val
		}
	}

	return ScanResult{
		RSSI:    props.RSSI,
		Address: a,
		AdvertisementPayload: &advertisementFields{
			AdvertisementFields{
				LocalName:        props.Name,
				ServiceUUIDs:     serviceUUIDs,
				ManufacturerData: mData,
			},
		},
	}
}

// Device is a connection to a remote peripheral.
type Device struct {
	device      *device.Device1             // bluez device interface
	ctx         context.Context             // context for our event watcher, canceled on disconnect event
	cancel      context.CancelFunc          // cancel function to halt our event watcher context
	propchanged chan *bluez.PropertyChanged // channel that device property changes will show up on
	adapter     *Adapter                    // the adapter that was used to form this device connection
	address     Address                     // the address of the device
}

// Connect starts a connection attempt to the given peripheral device address.
//
// On Linux and Windows, the IsRandom part of the address is ignored.
func (a *Adapter) Connect(address Address, params ConnectionParams) (*Device, error) {
	devicePath := dbus.ObjectPath(string(a.adapter.Path()) + "/dev_" + strings.Replace(address.MAC.String(), ":", "_", -1))
	dev, err := device.NewDevice1(devicePath)
	if err != nil {
		return nil, err
	}

	device := &Device{
		device:  dev,
		adapter: a,
		address: address,
	}
	device.ctx, device.cancel = context.WithCancel(context.Background())
	device.watchForConnect() // Set this up before we trigger a connection so we can capture the connect event

	if !dev.Properties.Connected {
		// Not yet connected, so do it now.
		// The properties have just been read so this is fresh data.
		err := dev.Connect()
		if err != nil {
			device.cancel() // cancel our watcher routine
			return nil, err
		}
	}

	return device, nil
}

// Disconnect from the BLE device. This method is non-blocking and does not
// wait until the connection is fully gone.
func (d *Device) Disconnect() error {
	// we don't call our cancel function here, instead we wait for the
	// property change in `watchForConnect` and cancel things then
	return d.device.Disconnect()
}

// watchForConnect watches for a signal from the bluez device interface that indicates a Connection/Disconnection.
//
// We can add extra signals to watch for here,
// see https://git.kernel.org/pub/scm/bluetooth/bluez.git/tree/doc/device-api.txt, for a full list
func (d *Device) watchForConnect() error {
	var err error
	d.propchanged, err = d.device.WatchProperties()
	if err != nil {
		return err
	}

	go func() {
		for {
			select {
			case changed := <-d.propchanged:

				// we will receive a nil if bluez.UnwatchProperties(a, ch) is called, if so we can stop watching
				if changed == nil {
					d.cancel()
					return
				}

				switch changed.Name {
				case "Connected":
					// Send off a notification indicating we have connected or disconnected
					d.adapter.connectHandler(d.address, d.device.Properties.Connected)

					if !d.device.Properties.Connected {
						d.cancel()
						return
					}
				}

				continue
			case <-d.ctx.Done():
				return
			}
		}
	}()

	return nil
}
