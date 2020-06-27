// +build !baremetal

package bluetooth

import (
	"errors"
	"strings"
	"time"

	"github.com/muka/go-bluetooth/bluez"
	"github.com/muka/go-bluetooth/bluez/profile/gatt"
)

// DeviceService is a BLE service on a connected peripheral device.
type DeviceService struct {
	service *gatt.GattService1
}

// DiscoverServices starts a service discovery procedure. Pass a list of service
// UUIDs you are interested in to this function. Either a slice of all services
// is returned (of the same length as the requested UUIDs and in the same
// order), or if some services could not be discovered an error is returned.
//
// Passing a nil slice of UUIDs will currently result in zero services being
// returned, but this may be changed in the future to return a complete list of
// services.
//
// On Linux with BlueZ, this just waits for the ServicesResolved signal (if
// services haven't been resolved yet) and uses this list of cached services.
func (d *Device) DiscoverServices(uuids []UUID) ([]DeviceService, error) {
	for {
		resolved, err := d.device.GetServicesResolved()
		if err != nil {
			return nil, err
		}
		if resolved {
			break
		}
		// This is a terrible hack, but I couldn't find another way.
		time.Sleep(10 * time.Millisecond)
	}

	services := make([]DeviceService, len(uuids))
	servicesFound := 0

	// Iterate through all objects managed by BlueZ, hoping to find the services
	// we're looking for.
	om, err := bluez.GetObjectManager()
	if err != nil {
		return nil, err
	}
	list, err := om.GetManagedObjects()
	if err != nil {
		return nil, err
	}
	for objectPath := range list {
		if !strings.HasPrefix(string(objectPath), string(d.device.Path())+"/service") {
			continue
		}
		suffix := string(objectPath)[len(d.device.Path()+"/"):]
		if len(strings.Split(suffix, "/")) != 1 {
			continue
		}
		service, err := gatt.NewGattService1(objectPath)
		if err != nil {
			return nil, err
		}
		for i, uuid := range uuids {
			if service.Properties.UUID != uuid.String() {
				// Not one of the services we're looking for.
				continue
			}
			if services[i].service != nil {
				// There is more than one service with the same UUID?
				// Don't overwrite it, to keep the servicesFound count correct.
				continue
			}
			services[i].service = service
			servicesFound++
			break
		}
	}

	if servicesFound != len(uuids) {
		return nil, errors.New("bluetooth: could not find some services")
	}

	return services, nil
}

// DeviceCharacteristic is a BLE characteristic on a connected peripheral
// device.
type DeviceCharacteristic struct {
	characteristic *gatt.GattCharacteristic1
}

// DiscoverCharacteristics discovers characteristics in this service. Pass a
// list of characteristic UUIDs you are interested in to this function. Either a
// list of all requested services is returned, or if some services could not be
// discovered an error is returned. If there is no error, the characteristics
// slice has the same length as the UUID slice with characteristics in the same
// order in the slice as in the requested UUID list.
//
// Passing a nil slice of UUIDs will currently result in zero characteristics
// being returned, but this may be changed in the future to return a complete
// list of characteristics.
func (s *DeviceService) DiscoverCharacteristics(uuids []UUID) ([]DeviceCharacteristic, error) {
	chars := make([]DeviceCharacteristic, len(uuids))
	characteristicsFound := 0

	// Iterate through all objects managed by BlueZ, hoping to find the
	// characteristic we're looking for.
	om, err := bluez.GetObjectManager()
	if err != nil {
		return nil, err
	}
	list, err := om.GetManagedObjects()
	if err != nil {
		return nil, err
	}
	for objectPath := range list {
		if !strings.HasPrefix(string(objectPath), string(s.service.Path())+"/char") {
			continue
		}
		suffix := string(objectPath)[len(s.service.Path()+"/"):]
		if len(strings.Split(suffix, "/")) != 1 {
			continue
		}
		char, err := gatt.NewGattCharacteristic1(objectPath)
		if err != nil {
			return nil, err
		}
		for i, uuid := range uuids {
			if char.Properties.UUID != uuid.String() {
				// Not one of the characteristics we're looking for.
				continue
			}
			if chars[i].characteristic != nil {
				// There is more than one characteristic with the same UUID?
				// Don't overwrite it, to keep the servicesFound count correct.
				continue
			}
			chars[i].characteristic = char
			characteristicsFound++
			break
		}
	}

	if characteristicsFound != len(uuids) {
		return nil, errors.New("bluetooth: could not find some characteristics")
	}

	return chars, nil
}

// WriteWithoutResponse replaces the characteristic value with a new value. The
// call will return before all data has been written. A limited number of such
// writes can be in flight at any given time. This call is also known as a
// "write command" (as opposed to a write request).
func (c DeviceCharacteristic) WriteWithoutResponse(p []byte) (n int, err error) {
	err = c.characteristic.WriteValue(p, nil)
	if err != nil {
		return 0, err
	}
	return len(p), nil
}

// EnableNotifications enables notifications in the Client Characteristic
// Configuration Descriptor (CCCD). This means that most peripherals will send a
// notification with a new value every time the value of the characteristic
// changes.
func (c DeviceCharacteristic) EnableNotifications(callback func(buf []byte)) error {
	ch, err := c.characteristic.WatchProperties()
	if err != nil {
		return err
	}
	go func() {
		for update := range ch {
			if update.Interface == "org.bluez.GattCharacteristic1" && update.Name == "Value" {
				callback(update.Value.([]byte))
			}
		}
	}()
	return c.characteristic.StartNotify()
}
