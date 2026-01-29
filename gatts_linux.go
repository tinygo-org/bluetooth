//go:build !baremetal

package bluetooth

import (
	"fmt"
	"strconv"
	"sync/atomic"

	"github.com/godbus/dbus/v5"
	"github.com/godbus/dbus/v5/prop"
)

// Unique ID per service (to generate a unique object path).
var serviceID uint64

// Characteristic is a single characteristic in a service. It has an UUID and a
// value.
type Characteristic struct {
	char        *bluezChar
	permissions CharacteristicPermissions
}

// A small ObjectManager for a single service.
type objectManager struct {
	objects map[dbus.ObjectPath]map[string]map[string]*prop.Prop
}

// This method implements org.freedesktop.DBus.ObjectManager.
func (om *objectManager) GetManagedObjects() (map[dbus.ObjectPath]map[string]map[string]dbus.Variant, *dbus.Error) {
	// Convert from a map with *prop.Prop keys, to a map with dbus.Variant keys.
	objects := map[dbus.ObjectPath]map[string]map[string]dbus.Variant{}
	for path, object := range om.objects {
		obj := make(map[string]map[string]dbus.Variant)
		objects[path] = obj
		for iface, props := range object {
			ifaceObj := make(map[string]dbus.Variant)
			obj[iface] = ifaceObj
			for k, v := range props {
				ifaceObj[k] = dbus.MakeVariant(v.Value)
			}
		}
	}
	return objects, nil
}

// Object that implements org.bluez.GattCharacteristic1 to be exported over
// DBus. Here is the documentation:
// https://git.kernel.org/pub/scm/bluetooth/bluez.git/tree/doc/org.bluez.GattCharacteristic.rst
type bluezChar struct {
	props      *prop.Properties
	writeEvent func(client Connection, offset int, value []byte)
}

func (c *bluezChar) ReadValue(options map[string]dbus.Variant) ([]byte, *dbus.Error) {
	// TODO: should we use the offset value? The BlueZ documentation doesn't
	// clearly specify this. The go-bluetooth library doesn't, but I believe it
	// should be respected.
	value := c.props.GetMust("org.bluez.GattCharacteristic1", "Value").([]byte)
	return value, nil
}

func (c *bluezChar) WriteValue(value []byte, options map[string]dbus.Variant) *dbus.Error {
	if c.writeEvent != nil {
		// BlueZ doesn't seem to tell who did the write, so pass 0 always as the
		// connection ID.
		client := Connection(0)
		offset, _ := options["offset"].Value().(uint16)
		c.writeEvent(client, int(offset), value)
	}
	return nil
}

func (a *Adapter) StopServiceAdvertisement() error {
	// TODO: implement on linux

	return nil
}

// AddService creates a new service with the characteristics listed in the
// Service struct.
func (a *Adapter) AddService(s *Service) error {
	// Create a unique DBus path for this service.
	id := atomic.AddUint64(&serviceID, 1)
	path := dbus.ObjectPath(fmt.Sprintf("/org/tinygo/bluetooth/service%d", id))

	// All objects that will be part of the ObjectManager.
	objects := map[dbus.ObjectPath]map[string]map[string]*prop.Prop{}

	// Define the service to be exported over DBus.
	serviceSpec := map[string]map[string]*prop.Prop{
		"org.bluez.GattService1": {
			"UUID":    {Value: s.UUID.String()},
			"Primary": {Value: true},
		},
	}
	objects[path] = serviceSpec

	for i, char := range s.Characteristics {
		// Calculate Flags field.
		bluezCharFlags := []string{
			"broadcast",              // bit 0
			"read",                   // bit 1
			"write-without-response", // bit 2
			"write",                  // bit 3
			"notify",                 // bit 4
			"indicate",               // bit 5
		}
		var flags []string
		for i := 0; i < len(bluezCharFlags); i++ {
			if (char.Flags>>i)&1 != 0 {
				flags = append(flags, bluezCharFlags[i])
			}
		}

		// Export the properties of this characteristic.
		charPath := path + dbus.ObjectPath("/char"+strconv.Itoa(i))
		propsSpec := map[string]map[string]*prop.Prop{
			"org.bluez.GattCharacteristic1": {
				"UUID":    {Value: char.UUID.String()},
				"Service": {Value: path},
				"Flags":   {Value: flags},
				"Value":   {Value: char.Value, Writable: true, Emit: prop.EmitTrue},
			},
		}
		objects[charPath] = propsSpec
		props, err := prop.Export(a.bus, charPath, propsSpec)
		if err != nil {
			return err
		}

		// Export the methods of this characteristic.
		obj := &bluezChar{
			props:      props,
			writeEvent: char.WriteEvent,
		}
		err = a.bus.Export(obj, charPath, "org.bluez.GattCharacteristic1")
		if err != nil {
			return err
		}

		// Keep the object around for Characteristic.Write.
		if char.Handle != nil {
			char.Handle.permissions = char.Flags
			char.Handle.char = obj
		}
	}

	// Export all objects that are part of our service.
	om := &objectManager{
		objects: objects,
	}
	err := a.bus.Export(om, path, "org.freedesktop.DBus.ObjectManager")
	if err != nil {
		return err
	}

	// Register our service.
	return a.adapter.Call("org.bluez.GattManager1.RegisterApplication", 0, path, map[string]dbus.Variant(nil)).Err
}

// Write replaces the characteristic value with a new value.
func (c *Characteristic) Write(p []byte) (n int, err error) {
	if len(p) == 0 {
		return 0, nil // nothing to do
	}

	if c.char.writeEvent != nil {
		c.char.writeEvent(0, 0, p)
	}
	gattError := c.char.props.Set("org.bluez.GattCharacteristic1", "Value", dbus.MakeVariant(p))
	if gattError != nil {
		return 0, gattError
	}
	return len(p), nil
}
