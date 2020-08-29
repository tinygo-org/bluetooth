package bluetooth

import (
	"errors"
	"time"

	"github.com/JuulLabs-OSS/cbgo"
)

// DiscoverServices starts a service discovery procedure. Pass a list of service
// UUIDs you are interested in to this function. Either a slice of all services
// is returned (of the same length as the requested UUIDs and in the same
// order), or if some services could not be discovered an error is returned.
func (d *Device) DiscoverServices(uuids []UUID) ([]DeviceService, error) {
	cbuuids := []cbgo.UUID{}
	for _, u := range uuids {
		uuid, _ := cbgo.ParseUUID(u.String())
		cbuuids = append(cbuuids, uuid)
	}

	d.prph.DiscoverServices(cbuuids)

	// wait on channel for service discovery
	select {
	case <-d.servicesChan:
		svcs := []DeviceService{}
		for _, dsvc := range d.prph.Services() {
			uuid, _ := ParseUUID(dsvc.UUID().String())
			svc := DeviceService{
				UUID:    uuid,
				device:  d,
				service: dsvc,
			}
			svcs = append(svcs, svc)
		}
		return svcs, nil
	case <-time.NewTimer(10 * time.Second).C:
		return nil, errors.New("timeout on DiscoverServices")
	}
}

// DeviceService is a BLE service on a connected peripheral device.
type DeviceService struct {
	UUID

	device *Device

	service cbgo.Service
}

// Device returns the Device for this service.
func (s *DeviceService) Device() *Device {
	return s.device
}

// DiscoverCharacteristics discovers characteristics in this service. Pass a
// list of characteristic UUIDs you are interested in to this function. Either a
// list of all requested services is returned, or if some services could not be
// discovered an error is returned. If there is no error, the characteristics
// slice has the same length as the UUID slice with characteristics in the same
// order in the slice as in the requested UUID list.
func (s *DeviceService) DiscoverCharacteristics(uuids []UUID) ([]DeviceCharacteristic, error) {
	cbuuids := []cbgo.UUID{}
	for _, u := range uuids {
		uuid, _ := cbgo.ParseUUID(u.String())
		cbuuids = append(cbuuids, uuid)
	}

	s.Device().Peripheral().DiscoverCharacteristics(cbuuids, s.service)

	// wait on channel for characteristic discovery
	select {
	case <-s.Device().CharsChan():
		chars := []DeviceCharacteristic{}
		for _, dchar := range s.service.Characteristics() {
			uuid, _ := ParseUUID(dchar.UUID().String())
			char := DeviceCharacteristic{
				UUID:           uuid,
				service:        s,
				characteristic: dchar,
			}
			chars = append(chars, char)
		}
		return chars, nil
	case <-time.NewTimer(10 * time.Second).C:
		return nil, errors.New("timeout on DiscoverCharacteristics")
	}
}

// DeviceCharacteristic is a BLE characteristic on a connected peripheral
// device.
type DeviceCharacteristic struct {
	UUID

	service *DeviceService

	characteristic cbgo.Characteristic
	callback       func(buf []byte)
}

// WriteWithoutResponse replaces the characteristic value with a new value. The
// call will return before all data has been written. A limited number of such
// writes can be in flight at any given time. This call is also known as a
// "write command" (as opposed to a write request).
func (c DeviceCharacteristic) WriteWithoutResponse(p []byte) (n int, err error) {
	c.service.Device().Peripheral().WriteCharacteristic(p, c.characteristic, false)

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

	return nil
}
