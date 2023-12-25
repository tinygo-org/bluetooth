//go:build softdevice

package bluetooth

/*
#include "ble_gap.h"
*/
import "C"

// Device is a connection to a remote peripheral or central.
type Device struct {
	Address Address

	connectionHandle C.uint16_t
}
