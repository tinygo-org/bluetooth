// +build !baremetal

package bluetooth

import (
	"github.com/muka/go-bluetooth/api"
	"github.com/muka/go-bluetooth/bluez/profile/adapter"
	"github.com/muka/go-bluetooth/bluez/profile/advertising"
	"github.com/muka/go-bluetooth/bluez/profile/device"
)

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
	if a.cancelScan != nil {
		return errScanning
	}

	// This appears to be necessary to receive any BLE discovery results at all.
	defer a.adapter.SetDiscoveryFilter(nil)
	err := a.adapter.SetDiscoveryFilter(map[string]interface{}{
		"Transport": "le",
	})
	if err != nil {
		return err
	}

	// Instruct BlueZ to start discovering.
	err = a.adapter.StartDiscovery()
	if err != nil {
		return err
	}

	// Listen for newly found devices.
	discoveryChan, cancelChan, err := a.adapter.OnDeviceDiscovered()
	if err != nil {
		return err
	}
	a.cancelScan = cancelChan

	// Obtain a list of cached devices to watch.
	// BlueZ won't show advertisement data as it is discovered. Instead, it
	// caches all the data and only produces events for changes. Worse: it
	// doesn't seem to remove cached devices for a long time (3 minutes?) so
	// simply reading the list of cached devices won't tell you what devices are
	// actually around right now.
	// Luckily, there is a workaround. When any value changes, you can be sure a
	// new advertisement packet has been received. The RSSI value changes almost
	// every time it seems so just watching property changes is enough to get a
	// near-accurate view of the current state of the world around the listening
	// device.
	devices, err := a.adapter.GetDevices()
	if err != nil {
		return err
	}
	for _, dev := range devices {
		a.startWatchingDevice(dev, callback)
	}

	// Iterate through new devices as they become visible.
	for result := range discoveryChan {
		if result.Type != adapter.DeviceAdded {
			continue
		}

		// We only got a DBus object path, so turn that into a Device1 object.
		dev, err := device.NewDevice1(result.Path)
		if err != nil || dev == nil {
			continue
		}

		// Signal to the API client that a new device has been found.
		callback(a, makeScanResult(dev))

		// Start watching this new device for when there are property changes.
		a.startWatchingDevice(dev, callback)
	}

	return nil
}

// StopScan stops any in-progress scan. It can be called from within a Scan
// callback to stop the current scan. If no scan is in progress, an error will
// be returned.
func (a *Adapter) StopScan() error {
	if a.cancelScan == nil {
		return errNotScanning
	}
	a.adapter.StopDiscovery()
	cancel := a.cancelScan
	a.cancelScan = nil
	cancel()
	return nil
}

// makeScanResult creates a ScanResult from a Device1 object.
func makeScanResult(dev *device.Device1) ScanResult {
	// Assume the Address property is well-formed.
	addr, _ := ParseMAC(dev.Properties.Address)

	// Create a list of UUIDs.
	var serviceUUIDs []UUID
	for _, uuid := range dev.Properties.UUIDs {
		// Assume the UUID is well-formed.
		parsedUUID, _ := ParseUUID(uuid)
		serviceUUIDs = append(serviceUUIDs, parsedUUID)
	}

	return ScanResult{
		RSSI: dev.Properties.RSSI,
		Address: Address{
			MAC:      addr,
			IsRandom: dev.Properties.AddressType == "random",
		},
		AdvertisementPayload: &advertisementFields{
			AdvertisementFields{
				LocalName:    dev.Properties.Name,
				ServiceUUIDs: serviceUUIDs,
			},
		},
	}
}

// startWatchingDevice starts watching for property changes in the device.
// Errors are ignored (for example, if watching the device failed).
// The dev object will be owned by the function and will be modified as
// properties change.
func (a *Adapter) startWatchingDevice(dev *device.Device1, callback func(*Adapter, ScanResult)) {
	ch, err := dev.WatchProperties()
	if err != nil {
		// Assume the device has disappeared or something.
		return
	}
	go func() {
		for change := range ch {
			// Update the device with the changed property.
			props, _ := dev.Properties.ToMap()
			props[change.Name] = change.Value
			dev.Properties, _ = dev.Properties.FromMap(props)

			// Signal to the API client that a property changed, as if this was
			// an incoming BLE advertisement packet.
			callback(a, makeScanResult(dev))
		}
	}()
}
