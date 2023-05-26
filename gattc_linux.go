//go:build !baremetal
// +build !baremetal

package bluetooth

import (
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/muka/go-bluetooth/bluez"
	"github.com/muka/go-bluetooth/bluez/profile/gatt"
)

var (
	errDupNotif = errors.New("unclosed notifications")
)

// UUIDWrapper is a type alias for UUID so we ensure no conflicts with
// struct method of the same name.
type uuidWrapper = UUID

// DeviceService is a BLE service on a connected peripheral device.
type DeviceService struct {
	uuidWrapper

	service *gatt.GattService1
}

// UUID returns the UUID for this DeviceService.
func (s *DeviceService) UUID() UUID {
	return s.uuidWrapper
}

// DiscoverServices starts a service discovery procedure. Pass a list of service
// UUIDs you are interested in to this function. Either a slice of all services
// is returned (of the same length as the requested UUIDs and in the same
// order), or if some services could not be discovered an error is returned.
//
// Passing a nil slice of UUIDs will return a complete list of
// services.
//
// On Linux with BlueZ, this just waits for the ServicesResolved signal (if
// services haven't been resolved yet) and uses this list of cached services.
func (d *Device) DiscoverServices(uuids []UUID) ([]DeviceService, error) {
	start := time.Now()

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
		if time.Since(start) > 10*time.Second {
			return nil, errors.New("timeout on DiscoverServices")
		}
	}

	services := []DeviceService{}
	uuidServices := make(map[string]string)
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
	objects := make([]string, 0, len(list))
	for objectPath := range list {
		objects = append(objects, string(objectPath))
	}
	sort.Strings(objects)
	for _, objectPath := range objects {
		if !strings.HasPrefix(objectPath, string(d.device.Path())+"/service") {
			continue
		}
		suffix := objectPath[len(d.device.Path()+"/"):]
		if len(strings.Split(suffix, "/")) != 1 {
			continue
		}
		service, err := gatt.NewGattService1(dbus.ObjectPath(objectPath))
		if err != nil {
			return nil, err
		}

		if len(uuids) > 0 {
			found := false
			for _, uuid := range uuids {
				if service.Properties.UUID == uuid.String() {
					// One of the services we're looking for.
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		if _, ok := uuidServices[service.Properties.UUID]; ok {
			// There is more than one service with the same UUID?
			// Don't overwrite it, to keep the servicesFound count correct.
			continue
		}

		uuid, _ := ParseUUID(service.Properties.UUID)
		ds := DeviceService{uuidWrapper: uuid,
			service: service,
		}

		services = append(services, ds)
		servicesFound++
		uuidServices[service.Properties.UUID] = service.Properties.UUID
	}

	if servicesFound < len(uuids) {
		return nil, errors.New("bluetooth: could not find some services")
	}

	return services, nil
}

// DeviceCharacteristic is a BLE characteristic on a connected peripheral
// device.
type DeviceCharacteristic struct {
	uuidWrapper

	characteristic *gatt.GattCharacteristic1
	property       chan *bluez.PropertyChanged // channel where notifications are reported
}

// UUID returns the UUID for this DeviceCharacteristic.
func (c *DeviceCharacteristic) UUID() UUID {
	return c.uuidWrapper
}

// DiscoverCharacteristics discovers characteristics in this service. Pass a
// list of characteristic UUIDs you are interested in to this function. Either a
// list of all requested services is returned, or if some services could not be
// discovered an error is returned. If there is no error, the characteristics
// slice has the same length as the UUID slice with characteristics in the same
// order in the slice as in the requested UUID list.
//
// Passing a nil slice of UUIDs will return a complete
// list of characteristics.
func (s *DeviceService) DiscoverCharacteristics(uuids []UUID) ([]DeviceCharacteristic, error) {
	var chars []DeviceCharacteristic
	if len(uuids) > 0 {
		// The caller wants to get a list of characteristics in a specific
		// order.
		chars = make([]DeviceCharacteristic, len(uuids))
	}

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
	objects := make([]string, 0, len(list))
	for objectPath := range list {
		objects = append(objects, string(objectPath))
	}
	sort.Strings(objects)
	for _, objectPath := range objects {
		if !strings.HasPrefix(objectPath, string(s.service.Path())+"/char") {
			continue
		}
		suffix := objectPath[len(s.service.Path()+"/"):]
		if len(strings.Split(suffix, "/")) != 1 {
			continue
		}
		characteristic, err := gatt.NewGattCharacteristic1(dbus.ObjectPath(objectPath))
		if err != nil {
			return nil, err
		}
		cuuid, _ := ParseUUID(characteristic.Properties.UUID)
		char := DeviceCharacteristic{
			uuidWrapper:    cuuid,
			characteristic: characteristic,
		}

		if len(uuids) > 0 {
			// The caller wants to get a list of characteristics in a specific
			// order. Check whether this is one of those.
			for i, uuid := range uuids {
				if chars[i] != (DeviceCharacteristic{}) {
					// To support multiple identical characteristics, we need to
					// ignore the characteristics that are already found. See:
					// https://github.com/tinygo-org/bluetooth/issues/131
					continue
				}
				if cuuid == uuid {
					// one of the characteristics we're looking for.
					chars[i] = char
					break
				}
			}
		} else {
			// The caller wants to get all characteristics, in any order.
			chars = append(chars, char)
		}
	}

	// Check that we have found all characteristics.
	for _, char := range chars {
		if char == (DeviceCharacteristic{}) {
			return nil, errors.New("bluetooth: could not find some characteristics")
		}
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
//
// Users may call EnableNotifications with a nil callback to disable notifications.
func (c *DeviceCharacteristic) EnableNotifications(callback func(buf []byte)) error {
	switch callback {
	default:
		if c.property != nil {
			return errDupNotif
		}

		ch, err := c.characteristic.WatchProperties()
		if err != nil {
			return err
		}

		err = c.characteristic.StartNotify()
		if err != nil {
			_ = c.characteristic.UnwatchProperties(ch)
			return err
		}
		c.property = ch

		go func() {
			for update := range ch {
				if update == nil {
					continue
				}
				if update.Interface == "org.bluez.GattCharacteristic1" && update.Name == "Value" {
					callback(update.Value.([]byte))
				}
			}
		}()

		return nil

	case nil:
		if c.property == nil {
			return nil
		}

		e1 := c.characteristic.StopNotify()
		e2 := c.characteristic.UnwatchProperties(c.property)
		c.property = nil

		// FIXME(sbinet): use errors.Join(e1, e2)
		if e1 != nil {
			return e1
		}

		if e2 != nil {
			return e2
		}

		return nil
	}
}

// GetMTU returns the MTU for the characteristic.
func (c DeviceCharacteristic) GetMTU() (uint16, error) {
	mtu, err := c.characteristic.GetProperty("MTU")
	if err != nil {
		return uint16(0), err
	}
	return mtu.Value().(uint16), nil
}

// Read reads the current characteristic value.
func (c *DeviceCharacteristic) Read(data []byte) (int, error) {
	options := make(map[string]interface{})
	result, err := c.characteristic.ReadValue(options)
	if err != nil {
		return 0, err
	}
	copy(data, result)
	return len(result), nil
}
