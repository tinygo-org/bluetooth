//go:build softdevice && (s140v6 || s140v7)

package bluetooth

/*
#include "nrf_soc.h"
*/
import "C"

// A package variable, because &usbregstatus escapes through the cgo call and a
// local variable would allocate on each call.
var usbregstatus C.uint32_t

// USBVBusPresent reports whether USB bus power (VBUS) is present, after Enable.
// Use it to stop serial output when the USB cable is removed, because a write
// to machine.Serial then blocks when its buffer is full.
// See nRF52840 Product Specification v1.11 section 5.3.7, USBREGSTATUS.
func (a *Adapter) USBVBusPresent() (bool, error) {
	if err := makeError(C.sd_power_usbregstatus_get(&usbregstatus)); err != nil {
		return false, err
	}
	return usbregstatus&C.POWER_USBREGSTATUS_VBUSDETECT_Msk != 0, nil
}
