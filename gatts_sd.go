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
		metadata.char_props.set_bitfield_read(1)
		metadata.char_props.set_bitfield_write(1)
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
		char.handle = handles.value_handle
	}
	return makeError(errCode)
}
