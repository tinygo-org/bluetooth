//go:build softdevice

package bluetooth

/*
#include "ble_gap.h"
#include "ble_gatts.h"

// Workaround wrapper functions which prevent pointer arguments escaping to heap
static inline uint32_t sd_ble_gatts_hvx_noescape(uint16_t conn_handle, uint16_t handle, uint8_t type, uint16_t offset, uint16_t len, uint8_t *p_data) {
	ble_gatts_hvx_params_t p_hvx_params = {handle, type, offset, &len, p_data};
	return sd_ble_gatts_hvx(conn_handle, &p_hvx_params);
}

static inline uint32_t sd_ble_gatts_value_set_noescape(uint16_t conn_handle, uint16_t handle, uint16_t len, uint8_t *value) {
	ble_gatts_value_t p_value = {
		.len     = len,
		.offset  = 0,
		.p_value = value,
	};
	return sd_ble_gatts_value_set(conn_handle, handle, &p_value);
}
*/
import "C"
import "unsafe"

// Characteristic is a single characteristic in a service. It has an UUID and a
// value.
type Characteristic struct {
	handle      C.uint16_t
	permissions CharacteristicPermissions
}

// AddService creates a new service with the characteristics listed in the
// Service struct.
func (a *Adapter) AddService(service *Service) error {
	uuid, errCode := service.UUID.shortUUID()
	if errCode != 0 {
		return Error(errCode)
	}
	errCode = C.sd_ble_gatts_service_add(C.BLE_GATTS_SRVC_TYPE_PRIMARY, &uuid, (*C.uint16_t)(unsafe.Pointer(&service.handle)))
	if errCode != 0 {
		return Error(errCode)
	}
	for _, char := range service.Characteristics {
		metadata := C.ble_gatts_char_md_t{}
		metadata.char_props.set_bitfield_broadcast(C.uint8_t(char.Flags>>0) & 1)
		metadata.char_props.set_bitfield_read(C.uint8_t(char.Flags>>1) & 1)
		metadata.char_props.set_bitfield_write_wo_resp(C.uint8_t(char.Flags>>2) & 1)
		metadata.char_props.set_bitfield_write(C.uint8_t(char.Flags>>3) & 1)
		metadata.char_props.set_bitfield_notify(C.uint8_t(char.Flags>>4) & 1)
		metadata.char_props.set_bitfield_indicate(C.uint8_t(char.Flags>>5) & 1)
		handles := C.ble_gatts_char_handles_t{}
		charUUID, errCode := char.UUID.shortUUID()
		if errCode != 0 {
			return Error(errCode)
		}
		value := C.ble_gatts_attr_t{
			p_uuid: &charUUID,
			p_attr_md: &C.ble_gatts_attr_md_t{
				read_perm:  secModeOpen,
				write_perm: secModeOpen,
			},
			init_len:  C.uint16_t(len(char.Value)),
			init_offs: 0,
			max_len:   20, // This is a conservative maximum length.
		}
		if len(char.Value) != 0 {
			value.p_value = (*C.uint8_t)(unsafe.Pointer(&char.Value[0]))
		}
		value.p_attr_md.set_bitfield_vloc(C.BLE_GATTS_VLOC_STACK)
		value.p_attr_md.set_bitfield_vlen(1)
		errCode = C.sd_ble_gatts_characteristic_add(C.uint16_t(service.handle), &metadata, &value, &handles)
		if errCode != 0 {
			return Error(errCode)
		}
		if char.Handle != nil {
			char.Handle.handle = handles.value_handle
			char.Handle.permissions = char.Flags
		}
		if char.Flags.Write() && char.WriteEvent != nil {
			handlers := append(a.charWriteHandlers, charWriteHandler{
				handle:   handles.value_handle,
				callback: char.WriteEvent,
			})
			mask := DisableInterrupts()
			a.charWriteHandlers = handlers
			RestoreInterrupts(mask)
		}
	}
	return makeError(errCode)
}

// charWriteHandler contains a handler->callback mapping for characteristic
// writes.
type charWriteHandler struct {
	handle   C.uint16_t
	callback func(connection Connection, offset int, value []byte)
}

// getCharWriteHandler returns a characteristic write handler if one matches the
// handle, or nil otherwise.
func (a *Adapter) getCharWriteHandler(handle C.uint16_t) *charWriteHandler {
	// Look through all handlers for a match.
	// There is probably a way to do this more efficiently (with a hashmap for
	// example) but the number of event handlers is likely low and improving
	// this does not need an API change.
	for i := range a.charWriteHandlers {
		h := &a.charWriteHandlers[i]
		if h.handle == handle {
			// Handler found.
			return h
		}
	}
	return nil // not found
}

// Write replaces the characteristic value with a new value.
func (c *Characteristic) Write(p []byte) (n int, err error) {
	if len(p) == 0 {
		// Nothing to write.
		return 0, nil
	}

	connHandle := currentConnection.Get()
	if connHandle != C.BLE_CONN_HANDLE_INVALID && c.permissions&(CharacteristicNotifyPermission|CharacteristicIndicatePermission) != 0 {
		// There is a connected central.
		p_len := uint16(len(p))
		errCode := C.sd_ble_gatts_hvx_noescape(connHandle,
			c.handle,
			C.BLE_GATT_HVX_NOTIFICATION,
			0,
			C.uint16_t(p_len),
			(*C.uint8_t)(unsafe.Pointer(&p[0])),
		)

		// Check for some expected errors. Don't report them as errors, but
		// instead fall through and do a normal characteristic value update.
		// Only return (and possibly report an error) in other cases.
		//
		// TODO: improve CGo so that the C constant can be used.
		if errCode == 0x0008 { // C.NRF_ERROR_INVALID_STATE
			// May happen when the central has unsubscribed from the
			// characteristic.
		} else if errCode == 0x3401 { // C.BLE_ERROR_GATTS_SYS_ATTR_MISSING
			// May happen when the central is not subscribed to this
			// characteristic.
		} else {
			return int(p_len), makeError(errCode)
		}
	}

	errCode := C.sd_ble_gatts_value_set_noescape(C.BLE_CONN_HANDLE_INVALID, c.handle, C.uint16_t(len(p)), (*C.uint8_t)(unsafe.Pointer(&p[0])))
	if errCode != 0 {
		return 0, Error(errCode)
	}

	return len(p), nil
}
