//go:build !baremetal

package bluetooth

import (
	"errors"
	"slices"
	"strings"

	"github.com/godbus/dbus/v5"
)

// DeviceDescriptor is a GATT descriptor on a characteristic of a connected
// peripheral device.
type DeviceDescriptor struct {
	uuidWrapper
	descriptor dbus.BusObject
}

// UUID returns the UUID for this DeviceDescriptor.
func (d DeviceDescriptor) UUID() UUID {
	return d.uuidWrapper
}

// DiscoverDescriptors discovers the descriptors of this characteristic. Pass a
// list of descriptor UUIDs you are interested in to this function, or a nil slice
// to return all descriptors. When UUIDs are given the result has the same length
// and order as the request, and an error is returned if any could not be found.
func (c DeviceCharacteristic) DiscoverDescriptors(uuids []UUID) ([]DeviceDescriptor, error) {
	var descs []DeviceDescriptor
	if len(uuids) > 0 {
		descs = make([]DeviceDescriptor, len(uuids))
	}

	// Descriptors live as D-Bus objects directly under the characteristic path,
	// so walk the managed objects and match on that prefix (mirrors how
	// DiscoverCharacteristics finds characteristics under a service).
	charPath := string(c.characteristic.Path())
	var list map[dbus.ObjectPath]map[string]map[string]dbus.Variant
	if err := c.adapter.bluez.Call("org.freedesktop.DBus.ObjectManager.GetManagedObjects", 0).Store(&list); err != nil {
		return nil, err
	}
	objects := make([]string, 0, len(list))
	for objectPath := range list {
		objects = append(objects, string(objectPath))
	}
	slices.Sort(objects)
	for _, objectPath := range objects {
		if !strings.HasPrefix(objectPath, charPath+"/desc") {
			continue
		}
		properties, ok := list[dbus.ObjectPath(objectPath)]["org.bluez.GattDescriptor1"]
		if !ok {
			continue
		}
		duuid, _ := ParseUUID(properties["UUID"].Value().(string))
		desc := DeviceDescriptor{
			uuidWrapper: duuid,
			descriptor:  c.adapter.bus.Object("org.bluez", dbus.ObjectPath(objectPath)),
		}
		if len(uuids) > 0 {
			for i, uuid := range uuids {
				if descs[i] != (DeviceDescriptor{}) {
					continue // already found; supports duplicate UUIDs
				}
				if duuid == uuid {
					descs[i] = desc
					break
				}
			}
		} else {
			descs = append(descs, desc)
		}
	}

	for _, desc := range descs {
		if desc == (DeviceDescriptor{}) {
			return nil, errors.New("bluetooth: could not find some descriptors")
		}
	}
	return descs, nil
}

// Read reads the current descriptor value, copying it into data and returning the
// number of bytes read.
func (d DeviceDescriptor) Read(data []byte) (int, error) {
	options := make(map[string]interface{})
	var result []byte
	if err := d.descriptor.Call("org.bluez.GattDescriptor1.ReadValue", 0, options).Store(&result); err != nil {
		return 0, err
	}
	copy(data, result)
	return len(result), nil
}

// Write writes a new value to the descriptor.
func (d DeviceDescriptor) Write(p []byte) (int, error) {
	options := make(map[string]interface{})
	if err := d.descriptor.Call("org.bluez.GattDescriptor1.WriteValue", 0, p, options).Err; err != nil {
		return 0, err
	}
	return len(p), nil
}
