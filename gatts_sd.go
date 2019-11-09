// +build softdevice,s132v6

package bluetooth

/*
// Define SoftDevice functions as regular function declarations (not inline
// static functions).
#define SVCALL_AS_NORMAL_FUNCTION

#include "s132_nrf52_6.1.1/s132_nrf52_6.1.1_API/include/ble_gap.h"
*/
import "C"

func (a *Adapter) AddService(service *Service) error {
	uuid, errCode := service.UUID.shortUUID()
	if errCode != 0 {
		return Error(errCode)
	}
	errCode = C.sd_ble_gatts_service_add(C.BLE_GATTS_SRVC_TYPE_PRIMARY, &uuid, &service.handle)
	if errCode != 0 {
		return Error(errCode)
	}
	for _, char := range service.Characteristics {
		metadata := C.ble_gatts_char_md_t{}
		metadata.char_props.set_bitfield_broadcast(uint8(char.Flags>>0) & 1)
		metadata.char_props.set_bitfield_read(uint8(char.Flags>>1) & 1)
		metadata.char_props.set_bitfield_write_wo_resp(uint8(char.Flags>>2) & 1)
		metadata.char_props.set_bitfield_write(uint8(char.Flags>>3) & 1)
		metadata.char_props.set_bitfield_notify(uint8(char.Flags>>4) & 1)
		metadata.char_props.set_bitfield_indicate(uint8(char.Flags>>5) & 1)
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
			init_len:  uint16(len(char.Value)),
			init_offs: 0,
			max_len:   uint16(len(char.Value)),
		}
		if len(char.Value) != 0 {
			value.p_value = &char.Value[0]
		}
		value.p_attr_md.set_bitfield_vloc(C.BLE_GATTS_VLOC_STACK)
		errCode = C.sd_ble_gatts_characteristic_add(service.handle, &metadata, &value, &handles)
		if errCode != 0 {
			return Error(errCode)
		}
		if char.Handle != nil {
			char.Handle.handle = handles.value_handle
			char.Handle.permissions = char.Flags
		}
		if char.Flags.Write() && char.WriteEvent != nil {
			a.charWriteHandlers = append(a.charWriteHandlers, charWriteHandler{
				handle:   handles.value_handle,
				callback: char.WriteEvent,
			})
		}
	}
	return makeError(errCode)
}

// charWriteHandler contains a handler->callback mapping for characteristic
// writes.
type charWriteHandler struct {
	handle   uint16
	callback func(connection Connection, offset int, value []byte)
}

// getCharWriteHandler returns a characteristic write handler if one matches the
// handle, or nil otherwise.
func (a *Adapter) getCharWriteHandler(handle uint16) *charWriteHandler {
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
