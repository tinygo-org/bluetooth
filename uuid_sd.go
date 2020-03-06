// +build softdevice

package bluetooth

/*
// Define SoftDevice functions as regular function declarations (not inline
// static functions).
#define SVCALL_AS_NORMAL_FUNCTION

#include "ble.h"
*/
import "C"
import "unsafe"

func (uuid UUID) shortUUID() (C.ble_uuid_t, uint32) {
	var short C.ble_uuid_t
	short.uuid = uint16(uuid[3])
	if uuid.Is16Bit() {
		short._type = C.BLE_UUID_TYPE_BLE
		return short, 0
	}
	errCode := C.sd_ble_uuid_vs_add((*C.ble_uuid128_t)(unsafe.Pointer(&uuid[0])), &short._type)
	return short, errCode
}
