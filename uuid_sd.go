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

type shortUUID C.ble_uuid_t

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

// UUID returns the full length UUID for this short UUID.
func (s shortUUID) UUID() UUID {
	if s._type == C.BLE_UUID_TYPE_BLE {
		return New16BitUUID(s.uuid)
	}
	var outLen C.uint8_t
	var outUUID UUID
	C.sd_ble_uuid_encode(((*C.ble_uuid_t)(unsafe.Pointer(&s))), &outLen, ((*C.uint8_t)(unsafe.Pointer(&outUUID))))
	return outUUID
}

// IsIn checks the passed in slice of short UUIDs to see if this uuid is in it.
func (s shortUUID) IsIn(uuids []C.ble_uuid_t) bool {
	for _, u := range uuids {
		if u == s {
			return true
		}
	}
	return false
}
