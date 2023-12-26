package bluetooth

import (
	"errors"
	"time"

	"github.com/tinygo-org/cbgo"
)

// DiscoverServices starts a service discovery procedure. Pass a list of service
// UUIDs you are interested in to this function. Either a slice of all services
// is returned (of the same length as the requested UUIDs and in the same
// order), or if some services could not be discovered an error is returned.
//
// Passing a nil slice of UUIDs will return a complete list of
// services.
func (d *Device) DiscoverServices(uuids []UUID) ([]DeviceService, error) {
	d.prph.DiscoverServices([]cbgo.UUID{})

	// clear cache of services
	d.services = make(map[UUID]DeviceService)

	// wait on channel for service discovery
	select {
	case <-d.servicesChan:
		svcs := []DeviceService{}
		for _, dsvc := range d.prph.Services() {
			dsvcuuid, _ := ParseUUID(dsvc.UUID().String())
			// add if in our original list
			if len(uuids) > 0 {
				found := false
				for _, uuid := range uuids {
					if dsvcuuid.String() == uuid.String() {
						// one of the services we're looking for.
						found = true
						break
					}
				}
				if !found {
					continue
				}
			}

			svc := DeviceService{
				deviceService: &deviceService{
					uuidWrapper: dsvcuuid,
					device:      d,
					service:     dsvc,
				},
			}
			svcs = append(svcs, svc)
			d.services[svc.uuidWrapper] = svc
		}
		return svcs, nil
	case <-time.NewTimer(10 * time.Second).C:
		return nil, errors.New("timeout on DiscoverServices")
	}
}

// uuidWrapper is a type alias for UUID so we ensure no conflicts with
// struct method of the same name.
type uuidWrapper = UUID

// DeviceService is a BLE service on a connected peripheral device.
type DeviceService struct {
	*deviceService // embdedded as pointer to enable returning by []value in DiscoverServices
}

type deviceService struct {
	uuidWrapper

	device *Device

	service         cbgo.Service
	characteristics []DeviceCharacteristic
}

// UUID returns the UUID for this DeviceService.
func (s *DeviceService) UUID() UUID {
	return s.uuidWrapper
}

// DiscoverCharacteristics discovers characteristics in this service. Pass a
// list of characteristic UUIDs you are interested in to this function. Either a
// list of all requested services is returned, or if some services could not be
// discovered an error is returned. If there is no error, the characteristics
// slice has the same length as the UUID slice with characteristics in the same
// order in the slice as in the requested UUID list.
//
// Passing a nil slice of UUIDs will return a complete list of
// characteristics.
func (s *DeviceService) DiscoverCharacteristics(uuids []UUID) ([]DeviceCharacteristic, error) {
	cbuuids := []cbgo.UUID{}

	s.device.prph.DiscoverCharacteristics(cbuuids, s.service)

	// clear cache of characteristics
	s.characteristics = make([]DeviceCharacteristic, 0)

	// wait on channel for characteristic discovery
	select {
	case <-s.device.charsChan:
		var chars []DeviceCharacteristic
		if len(uuids) > 0 {
			// The caller wants to get a list of characteristics in a specific
			// order.
			chars = make([]DeviceCharacteristic, len(uuids))
		}
		for _, dchar := range s.service.Characteristics() {
			dcuuid, _ := ParseUUID(dchar.UUID().String())
			if len(uuids) > 0 {
				// The caller wants to get a list of characteristics in a
				// specific order. Check whether this is one of those.
				for i, uuid := range uuids {
					if chars[i] != (DeviceCharacteristic{}) {
						// To support multiple identical characteristics, we
						// need to ignore the characteristics that are already
						// found. See:
						// https://github.com/tinygo-org/bluetooth/issues/131
						continue
					}
					if dcuuid == uuid {
						// one of the characteristics we're looking for.
						chars[i] = s.makeCharacteristic(dcuuid, dchar)
						break
					}
				}
			} else {
				// The caller wants to get all characteristics, in any order.
				chars = append(chars, s.makeCharacteristic(dcuuid, dchar))
			}
		}
		for _, char := range chars {
			if char == (DeviceCharacteristic{}) {
				return nil, errors.New("bluetooth: did not find all requested characteristic")
			}
		}
		return chars, nil
	case <-time.NewTimer(10 * time.Second).C:
		return nil, errors.New("timeout on DiscoverCharacteristics")
	}
}

// Small helper to create a DeviceCharacteristic object.
func (s *DeviceService) makeCharacteristic(uuid UUID, dchar cbgo.Characteristic) DeviceCharacteristic {
	char := DeviceCharacteristic{
		deviceCharacteristic: &deviceCharacteristic{
			uuidWrapper:    uuid,
			service:        s,
			characteristic: dchar,
		},
	}
	s.characteristics = append(s.characteristics, char)
	return char
}

// DeviceCharacteristic is a BLE characteristic on a connected peripheral
// device.
type DeviceCharacteristic struct {
	*deviceCharacteristic
}

type deviceCharacteristic struct {
	uuidWrapper

	service *DeviceService

	characteristic cbgo.Characteristic
	callback       func(buf []byte)
	readChan       chan error
	writeChan      chan error
}

// UUID returns the UUID for this DeviceCharacteristic.
func (c *DeviceCharacteristic) UUID() UUID {
	return c.uuidWrapper
}

// Write replaces the characteristic value with a new value. The
// call will return after all data has been written.
func (c DeviceCharacteristic) Write(p []byte) (n int, err error) {
	c.writeChan = make(chan error)
	c.service.device.prph.WriteCharacteristic(p, c.characteristic, true)

	// wait for result
	select {
	case <-time.NewTimer(10 * time.Second).C:
		err = errors.New("timeout on Write()")
	case err = <-c.writeChan:
	}

	c.writeChan = nil
	if err != nil {
		return 0, err
	}

	return len(p), nil
}

// WriteWithoutResponse replaces the characteristic value with a new value. The
// call will return before all data has been written. A limited number of such
// writes can be in flight at any given time. This call is also known as a
// "write command" (as opposed to a write request).
func (c DeviceCharacteristic) WriteWithoutResponse(p []byte) (n int, err error) {
	c.service.device.prph.WriteCharacteristic(p, c.characteristic, false)

	return len(p), nil
}

// EnableNotifications enables notifications in the Client Characteristic
// Configuration Descriptor (CCCD). This means that most peripherals will send a
// notification with a new value every time the value of the characteristic
// changes.
func (c DeviceCharacteristic) EnableNotifications(callback func(buf []byte)) error {
	if callback == nil {
		return errors.New("must provide a callback for EnableNotifications")
	}

	c.callback = callback
	c.service.device.prph.SetNotify(true, c.characteristic)

	return nil
}

// GetMTU returns the MTU for the characteristic.
func (c DeviceCharacteristic) GetMTU() (uint16, error) {
	return uint16(c.service.device.prph.MaximumWriteValueLength(false)), nil
}

// Read reads the current characteristic value.
func (c *deviceCharacteristic) Read(data []byte) (n int, err error) {
	c.readChan = make(chan error)
	c.service.device.prph.ReadCharacteristic(c.characteristic)

	// wait for result
	select {
	case err := <-c.readChan:
		c.readChan = nil
		if err != nil {
			return 0, err
		}
	case <-time.NewTimer(10 * time.Second).C:
		c.readChan = nil
		return 0, errors.New("timeout on Read()")
	}

	copy(data, c.characteristic.Value())
	return len(c.characteristic.Value()), nil
}
