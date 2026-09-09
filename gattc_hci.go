//go:build hci || ninafw || cyw43439 || espradio

package bluetooth

import (
	"errors"

	"tinygo.org/x/bluetooth/hci"
)

var (
	errNotYetImplemented         = errors.New("bluetooth: not yet implemented")
	errNoWrite                   = errors.New("bluetooth: write not permitted")
	errNoWriteWithoutResponse    = errors.New("bluetooth: write without response not permitted")
	errWriteFailed               = errors.New("bluetooth: write failed")
	errNoRead                    = errors.New("bluetooth: read not permitted")
	errReadFailed                = errors.New("bluetooth: read failed")
	errNoNotify                  = errors.New("bluetooth: notify/indicate not permitted")
	errEnableNotificationsFailed = errors.New("bluetooth: enable notifications failed")
	errServiceNotFound           = errors.New("bluetooth: service not found")
	errCharacteristicNotFound    = errors.New("bluetooth: characteristic not found")
)

var (
	_ GATTCService        = (*DeviceService)(nil)
	_ GATTCCharacteristic = (*DeviceCharacteristic)(nil)
)

const (
	maxDefaultServicesToDiscover        = 8
	maxDefaultCharacteristicsToDiscover = 16
)

const (
	charPropertyBroadcast            = 0x01
	charPropertyRead                 = 0x02
	charPropertyWriteWithoutResponse = 0x04
	charPropertyWrite                = 0x08
	charPropertyNotify               = 0x10
	charPropertyIndicate             = 0x20
)

// DeviceService is a BLE service on a connected peripheral device.
type DeviceService struct {
	uuid UUID

	device                 Device
	startHandle, endHandle uint16
}

// UUID returns the UUID for this DeviceService.
func (s DeviceService) UUID() UUID {
	return s.uuid
}

// DiscoverServices starts a service discovery procedure. Pass a list of service
// UUIDs you are interested in to this function. Either a slice of all services
// is returned (of the same length as the requested UUIDs and in the same
// order), or if some services could not be discovered an error is returned.
//
// Passing a nil slice of UUIDs will return a complete list of
// services.
func (d Device) DiscoverServices(uuids []UUID) ([]DeviceService, error) {
	if debug {
		println("DiscoverServices")
	}

	services := make([]DeviceService, 0, maxDefaultServicesToDiscover)
	foundServices := make(map[UUID]DeviceService)

	cd, err := d.adapter.att.ConnectionData(d.handle)
	if err != nil {
		return nil, err
	}

	startHandle := uint16(0x0001)
	endHandle := uint16(0xffff)
	for endHandle == uint16(0xffff) {
		err := d.adapter.att.ReadByGroupReq(d.handle, startHandle, endHandle, hci.UUIDPrimaryService)
		if err != nil {
			if err == ErrATTAttributeNotFound {
				break
			}
			return nil, err
		}

		if debug {
			println("found services", len(cd.Services))
		}

		if len(cd.Services) == 0 {
			break
		}

		for _, rawService := range cd.Services {
			if len(uuids) == 0 || uuidIn(rawService.UUID, uuids) {
				foundServices[rawService.UUID] =
					DeviceService{
						device:      d,
						uuid:        rawService.UUID,
						startHandle: rawService.StartHandle,
						endHandle:   rawService.EndHandle,
					}
			}

			startHandle = rawService.EndHandle + 1
			if startHandle == 0x0000 {
				endHandle = 0x0000
			}
		}

		// reset raw services
		cd.Services = []hci.Service{}

		// did we find them all?
		if len(foundServices) == len(uuids) {
			break
		}
	}

	switch {
	case len(uuids) > 0:
		// put into correct order
		for _, uuid := range uuids {
			s, ok := foundServices[uuid]
			if !ok {
				return nil, errServiceNotFound
			}

			services = append(services, s)
		}
	default:
		for _, s := range foundServices {
			services = append(services, s)
		}
	}

	return services, nil
}

// DeviceCharacteristic is a BLE characteristic on a connected peripheral
// device.
type DeviceCharacteristic struct {
	uuid UUID

	service     *DeviceService
	permissions CharacteristicPermissions
	handle      uint16
	properties  uint8
	callback    func(buf []byte)
}

// UUID returns the UUID for this DeviceCharacteristic.
func (c DeviceCharacteristic) UUID() UUID {
	return c.uuid
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
func (s DeviceService) DiscoverCharacteristics(uuids []UUID) ([]DeviceCharacteristic, error) {
	if debug {
		println("DiscoverCharacteristics")
	}

	characteristics := make([]DeviceCharacteristic, 0, maxDefaultCharacteristicsToDiscover)
	foundCharacteristics := make(map[UUID]DeviceCharacteristic)

	cd, err := s.device.adapter.att.ConnectionData(s.device.handle)
	if err != nil {
		return nil, err
	}

	startHandle := s.startHandle
	endHandle := s.endHandle
	for startHandle < endHandle {
		err := s.device.adapter.att.ReadByTypeReq(s.device.handle, startHandle, endHandle, hci.UUIDCharacteristic)
		switch {
		case err == ErrATTOp:
			attErr, _ := s.device.adapter.att.LastError(s.device.handle)
			if attErr.Opcode == hci.OpReadByTypeReq && attErr.Code == ErrAttNotFound {
				// no characteristics found
				break
			}
		case err != nil:
			return nil, err
		}

		if debug {
			println("found characteristics", len(cd.Characteristics))
		}

		if len(cd.Characteristics) == 0 {
			break
		}

		for _, rawCharacteristic := range cd.Characteristics {
			if len(uuids) == 0 || uuidIn(rawCharacteristic.UUID, uuids) {
				dc := DeviceCharacteristic{
					service:     &s,
					uuid:        rawCharacteristic.UUID,
					handle:      rawCharacteristic.ValueHandle,
					properties:  rawCharacteristic.Properties,
					permissions: CharacteristicPermissions(rawCharacteristic.Properties),
				}

				foundCharacteristics[rawCharacteristic.UUID] = dc
			}

			startHandle = rawCharacteristic.ValueHandle + 1
		}

		// reset raw characteristics
		cd.Characteristics = []hci.Characteristic{}

		// did we find them all?
		if len(foundCharacteristics) == len(uuids) {
			break
		}
	}

	switch {
	case len(uuids) > 0:
		// put into correct order
		for _, uuid := range uuids {
			c, ok := foundCharacteristics[uuid]
			if !ok {
				return nil, errCharacteristicNotFound
			}
			characteristics = append(characteristics, c)
		}
	default:
		for _, c := range foundCharacteristics {
			characteristics = append(characteristics, c)
		}

	}

	return characteristics, nil
}

// Write replaces the characteristic value with a new value.
//
// Not yet implemented on HCI.
func (c DeviceCharacteristic) Write(p []byte) (n int, err error) {
	return 0, errNotYetImplemented
}

// WriteWithoutResponse replaces the characteristic value with a new value. The
// call will return before all data has been written. A limited number of such
// writes can be in flight at any given time. This call is also known as a
// "write command" (as opposed to a write request).
func (c DeviceCharacteristic) WriteWithoutResponse(p []byte) (n int, err error) {
	if !c.permissions.WriteWithoutResponse() {
		return 0, errNoWriteWithoutResponse
	}

	err = c.service.device.adapter.att.WriteCmd(c.service.device.handle, c.handle, p)
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
func (c DeviceCharacteristic) EnableNotifications(callback func(buf []byte)) error {
	if !c.permissions.Notify() {
		return errNoNotify
	}

	switch {
	case callback == nil:
		// disable notifications
		if debug {
			println("disabling notifications")
		}

		err := c.service.device.adapter.att.WriteReq(c.service.device.handle, c.handle+1, []byte{0x00, 0x00})
		if err != nil {
			return err
		}
	default:
		// enable notifications
		if debug {
			println("enabling notifications")
		}

		err := c.service.device.adapter.att.WriteReq(c.service.device.handle, c.handle+1, []byte{0x01, 0x00})
		if err != nil {
			return err
		}
	}

	c.callback = callback

	c.service.device.startNotifications()
	c.service.device.addNotificationRegistration(c.handle, c.callback)

	return nil
}

// GetMTU returns the MTU for the characteristic.
func (c DeviceCharacteristic) GetMTU() (uint16, error) {
	err := c.service.device.adapter.att.MTUReq(c.service.device.handle)
	if err != nil {
		return 0, err
	}

	c.service.device.mtu = c.service.device.adapter.att.MTU()

	return c.service.device.mtu, nil
}

// Read reads the current characteristic value.
func (c DeviceCharacteristic) Read(data []byte) (int, error) {
	if !c.permissions.Read() {
		return 0, errNoRead
	}

	err := c.service.device.adapter.att.ReadReq(c.service.device.handle, c.handle)
	if err != nil {
		return 0, err
	}

	cd, err := c.service.device.adapter.att.ConnectionData(c.service.device.handle)
	if err != nil {
		return 0, err
	}

	if len(cd.Value) == 0 {
		return 0, errReadFailed
	}

	copy(data, cd.Value)

	return len(cd.Value), nil
}
