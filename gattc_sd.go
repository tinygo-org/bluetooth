//go:build (softdevice && s132v6) || (softdevice && s140v6) || (softdevice && s140v7)

package bluetooth

/*
#include "ble_gattc.h"
*/
import "C"

import (
	"device/arm"
	"errors"
	"runtime/volatile"
	"unsafe"
)

const (
	maxDefaultServicesToDiscover        = 6
	maxDefaultCharacteristicsToDiscover = 8
)

var (
	errAlreadyDiscovering = errors.New("bluetooth: already discovering a service or characteristic")
	errNotFound           = errors.New("bluetooth: not found")
	errNoNotify           = errors.New("bluetooth: no notify permission")
)

// A global used while discovering services, to communicate between the main
// program and the event handler.
var discoveringService struct {
	state       volatile.Register8 // 0 means nothing happening, 1 means in progress, 2 means found something
	startHandle volatileHandle
	endHandle   volatileHandle
	uuid        C.ble_uuid_t
}

// DeviceService is a BLE service on a connected peripheral device. It is only
// valid as long as the device remains connected.
type DeviceService struct {
	uuid shortUUID

	connectionHandle C.uint16_t
	startHandle      C.uint16_t
	endHandle        C.uint16_t
}

// UUID returns the UUID for this DeviceService.
func (s DeviceService) UUID() UUID {
	return s.uuid.UUID()
}

// DiscoverServices starts a service discovery procedure. Pass a list of service
// UUIDs you are interested in to this function. Either a slice of all services
// is returned (of the same length as the requested UUIDs and in the same
// order), or if some services could not be discovered an error is returned.
//
// Passing a nil slice of UUIDs will return a complete list of
// services.
//
// On the Nordic SoftDevice, only one service discovery procedure may be done at
// a time.
func (d Device) DiscoverServices(uuids []UUID) ([]DeviceService, error) {
	if discoveringService.state.Get() != 0 {
		// Not concurrency safe, but should catch most concurrency misuses.
		return nil, errAlreadyDiscovering
	}

	sz := maxDefaultServicesToDiscover
	if len(uuids) > 0 {
		sz = len(uuids)
	}
	services := make([]DeviceService, 0, sz)

	var shortUUIDs []C.ble_uuid_t

	// Make a map of UUIDs in SoftDevice short form, for easier comparing.
	if len(uuids) > 0 {
		shortUUIDs = make([]C.ble_uuid_t, sz)
		for i, uuid := range uuids {
			var errCode C.uint32_t
			shortUUIDs[i], errCode = uuid.shortUUID()
			if errCode != 0 {
				return nil, Error(errCode)
			}
		}
	}

	numFound := 0

	var startHandle C.uint16_t = 1

	for i := 0; i < sz; i++ {
		var suuid C.ble_uuid_t
		if len(uuids) > 0 {
			suuid = shortUUIDs[i]
		}

		// Start discovery of this service.
		discoveringService.state.Set(1)
		var errCode C.uint32_t
		if len(uuids) > 0 {
			errCode = C.sd_ble_gattc_primary_services_discover(d.connectionHandle, startHandle, &suuid)
		} else {
			// calling with nil searches for all primary services.
			// TODO: need a way to set suuid from the returned data
			errCode = C.sd_ble_gattc_primary_services_discover(d.connectionHandle, startHandle, nil)
		}
		if errCode != 0 {
			discoveringService.state.Set(0)
			return nil, Error(errCode)
		}

		// Wait until it is discovered.
		// TODO: use some sort of condition variable once the scheduler supports
		// them.
		for discoveringService.state.Get() == 1 {
			// still waiting...
			arm.Asm("wfe")
		}
		// Retrieve values, and mark the global as unused.
		startHandle = discoveringService.startHandle.Get()
		endHandle := discoveringService.endHandle.Get()
		suuid = discoveringService.uuid
		discoveringService.state.Set(0)

		if startHandle == 0 {
			// The event handler will set the start handle to zero if the
			// service was not found.
			return nil, errNotFound
		}

		// Store the discovered service.
		svc := DeviceService{
			uuid:             shortUUID(suuid),
			connectionHandle: d.connectionHandle,
			startHandle:      startHandle,
			endHandle:        endHandle,
		}
		services = append(services, svc)

		numFound++
		if numFound >= sz {
			break
		}

		// last entry
		if endHandle == 0xffff {
			break
		}

		// start with the next handle
		startHandle = endHandle + 1
	}

	return services, nil
}

// DeviceCharacteristic is a BLE characteristic on a connected peripheral
// device. It is only valid as long as the device remains connected.
type DeviceCharacteristic struct {
	uuid shortUUID

	connectionHandle C.uint16_t
	valueHandle      C.uint16_t
	cccdHandle       C.uint16_t
	permissions      CharacteristicPermissions
}

// UUID returns the UUID for this DeviceCharacteristic.
func (c DeviceCharacteristic) UUID() UUID {
	return c.uuid.UUID()
}

// A global used to pass information from the event handler back to the
// DiscoverCharacteristics function below.
var discoveringCharacteristic struct {
	uuid         C.ble_uuid_t
	char_props   C.ble_gatt_char_props_t
	handle_value volatileHandle
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
	if discoveringCharacteristic.handle_value.Get() != 0 {
		return nil, errAlreadyDiscovering
	}

	sz := maxDefaultCharacteristicsToDiscover
	if len(uuids) > 0 {
		sz = len(uuids)
	}
	characteristics := make([]DeviceCharacteristic, 0, sz)

	var shortUUIDs []C.ble_uuid_t

	// Make a map of UUIDs in SoftDevice short form, for easier comparing.
	if len(uuids) > 0 {
		shortUUIDs = make([]C.ble_uuid_t, sz)
		for i, uuid := range uuids {
			var errCode C.uint32_t
			shortUUIDs[i], errCode = uuid.shortUUID()
			if errCode != 0 {
				return nil, Error(errCode)
			}
		}
	}

	// Request characteristics one by one, until all are found.
	numFound := 0
	startHandle := s.startHandle

	for startHandle < s.endHandle {
		// Discover the next characteristic in this service.
		handles := C.ble_gattc_handle_range_t{
			start_handle: startHandle,
			end_handle:   startHandle + 1,
		}

		errCode := C.sd_ble_gattc_characteristics_discover(s.connectionHandle, &handles)
		if errCode != 0 {
			return nil, Error(errCode)
		}

		// Wait until it is discovered.
		// TODO: use some sort of condition variable once the scheduler supports
		// them.
		for discoveringCharacteristic.handle_value.Get() == 0 {
			arm.Asm("wfe")
		}
		foundCharacteristicHandle := discoveringCharacteristic.handle_value.Get()
		discoveringCharacteristic.handle_value.Set(0)

		// was it last characteristic?
		if foundCharacteristicHandle == 0xffff {
			break
		}

		// Start the next request from the handle right after this one.
		startHandle = foundCharacteristicHandle + 1

		// not one of the characteristics we are looking for
		if len(shortUUIDs) > 0 && !shortUUID(discoveringCharacteristic.uuid).IsIn(shortUUIDs) {
			continue
		}

		// Found a characteristic.
		permissions := CharacteristicPermissions(0)
		rawPermissions := discoveringCharacteristic.char_props
		if rawPermissions.bitfield_broadcast() != 0 {
			permissions |= CharacteristicBroadcastPermission
		}
		if rawPermissions.bitfield_read() != 0 {
			permissions |= CharacteristicReadPermission
		}
		if rawPermissions.bitfield_write_wo_resp() != 0 {
			permissions |= CharacteristicWriteWithoutResponsePermission
		}
		if rawPermissions.bitfield_write() != 0 {
			permissions |= CharacteristicWritePermission
		}
		if rawPermissions.bitfield_notify() != 0 {
			permissions |= CharacteristicNotifyPermission
		}
		if rawPermissions.bitfield_indicate() != 0 {
			permissions |= CharacteristicIndicatePermission
		}

		dc := DeviceCharacteristic{uuid: shortUUID(discoveringCharacteristic.uuid)}
		dc.permissions = permissions
		dc.valueHandle = foundCharacteristicHandle

		if permissions&CharacteristicNotifyPermission != 0 {
			// This characteristic has the notify permission, so most
			// likely it also supports notifications.
			errCode := C.sd_ble_gattc_descriptors_discover(s.connectionHandle, &C.ble_gattc_handle_range_t{
				start_handle: startHandle,
				end_handle:   startHandle + 1,
			})
			if errCode != 0 {
				return nil, Error(errCode)
			}

			// Wait until the descriptor handle is found.
			for discoveringCharacteristic.handle_value.Get() == 0 {
				arm.Asm("wfe")
			}
			foundDescriptorHandle := discoveringCharacteristic.handle_value.Get()
			discoveringCharacteristic.handle_value.Set(0)

			dc.cccdHandle = foundDescriptorHandle
		}

		characteristics = append(characteristics, dc)
		numFound++
		if numFound >= sz {
			break
		}
	}

	if len(uuids) > 0 && numFound != len(uuids) {
		return nil, errNotFound
	}

	return characteristics, nil
}

// WriteWithoutResponse replaces the characteristic value with a new value. The
// call will return before all data has been written. A limited number of such
// writes can be in flight at any given time. This call is also known as a
// "write command" (as opposed to a write request).
func (c DeviceCharacteristic) WriteWithoutResponse(p []byte) (n int, err error) {
	if len(p) == 0 {
		return 0, nil
	}

	errCode := C.sd_ble_gattc_write(c.connectionHandle, &C.ble_gattc_write_params_t{
		write_op: C.BLE_GATT_OP_WRITE_CMD,
		handle:   c.valueHandle,
		offset:   0,
		len:      C.uint16_t(len(p)),
		p_value:  (*C.uint8_t)(unsafe.Pointer(&p[0])),
	})
	if errCode != 0 {
		return 0, Error(errCode)
	}
	return len(p), nil
}

type gattcNotificationCallback struct {
	connectionHandle C.uint16_t
	valueHandle      C.uint16_t // may be 0 if the slot is empty
	callback         func([]byte)
}

// List of notification callbacks for the current connection. Some slots may be
// empty, they are indicated with a zero valueHandle. They can be reused for new
// notification callbacks.
var gattcNotificationCallbacks []gattcNotificationCallback

// EnableNotifications enables notifications in the Client Characteristic
// Configuration Descriptor (CCCD). This means that most peripherals will send a
// notification with a new value every time the value of the characteristic
// changes.
//
// Warning: when using the SoftDevice, the callback is called from an interrupt
// which means there are various limitations (such as not being able to allocate
// heap memory).
func (c DeviceCharacteristic) EnableNotifications(callback func(buf []byte)) error {
	if c.permissions&CharacteristicNotifyPermission == 0 {
		return errNoNotify
	}

	// Try to insert the callback in the list.
	updatedCallback := false
	mask := DisableInterrupts()
	for i, callbackInfo := range gattcNotificationCallbacks {
		// Check for re-enabling an already enabled notification.
		if callbackInfo.valueHandle == c.valueHandle {
			gattcNotificationCallbacks[i].callback = callback
			updatedCallback = true
			break
		}
	}
	if !updatedCallback {
		for i, callbackInfo := range gattcNotificationCallbacks {
			// Check for empty slots.
			if callbackInfo.valueHandle == 0 {
				gattcNotificationCallbacks[i] = gattcNotificationCallback{
					connectionHandle: c.connectionHandle,
					valueHandle:      c.valueHandle,
					callback:         callback,
				}
				updatedCallback = true
				break
			}
		}
	}
	RestoreInterrupts(mask)

	// Add this callback to the list of callbacks, if it couldn't be inserted
	// into the list.
	if !updatedCallback {
		// The append call is done outside of a cricital section to avoid GC in
		// a critical section.
		callbackList := append(gattcNotificationCallbacks, gattcNotificationCallback{
			connectionHandle: c.connectionHandle,
			valueHandle:      c.valueHandle,
			callback:         callback,
		})
		mask := DisableInterrupts()
		gattcNotificationCallbacks = callbackList
		RestoreInterrupts(mask)
	}

	// Write to the CCCD to enable notifications. Don't wait for a response.
	value := [2]C.uint8_t{0x01, 0x00} // 0x0001 enables notifications (and disables indications)
	errCode := C.sd_ble_gattc_write(c.connectionHandle, &C.ble_gattc_write_params_t{
		write_op: C.BLE_GATT_OP_WRITE_CMD,
		handle:   c.cccdHandle,
		offset:   0,
		len:      2,
		p_value:  &value[0],
	})
	return makeError(errCode)
}

// A global used to pass information from the event handler back to the
// Read function below.
var readingCharacteristic struct {
	handle_value volatileHandle
	offset       C.uint16_t
	length       C.uint16_t
	value        []byte
}

// Read reads the current characteristic value up to MTU length.
// A future enhancement would be to be able to retrieve a longer
// value by making multiple calls.
func (c DeviceCharacteristic) Read(data []byte) (n int, err error) {
	// global will copy bytes from read operation into data slice
	readingCharacteristic.value = data

	errCode := C.sd_ble_gattc_read(c.connectionHandle, c.valueHandle, 0)
	if errCode != 0 {
		return 0, Error(errCode)
	}

	// wait for response with data
	for readingCharacteristic.handle_value.Get() == 0 {
		arm.Asm("wfe")
	}

	// how much data was read into buffer
	n = int(readingCharacteristic.length)

	// prepare for next read
	readingCharacteristic.handle_value.Set(0)
	readingCharacteristic.length = 0

	return
}

// GetMTU returns the MTU for the characteristic.
func (c DeviceCharacteristic) GetMTU() (uint16, error) {
	return uint16(C.BLE_GATT_ATT_MTU_DEFAULT), nil
}
