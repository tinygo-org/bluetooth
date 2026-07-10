//go:build (softdevice && s140v6) || (softdevice && s140v7)

package bluetooth

/*
#include "nrf_soc.h"
*/
import "C"

// USBVBusPresent reports whether USB bus power (VBUS) is currently present.
// It uses sd_power_usbregstatus_get because the POWER peripheral is
// restricted while the SoftDevice is enabled. Before Adapter.Enable it
// returns false.
//
// TinyGo's machine layer never notices the cable being unplugged (the USB
// power SOC events are drained unhandled in adapter_sd.go), so with a serial
// monitor attached the USB CDC console keeps accepting output after unplug
// and machine.Serial.Write blocks forever once its TX ring fills up. Guard
// debug output on battery-powered devices with this function.
// usbregstatus is a package-level variable to avoid a heap allocation: &x
// escapes through the CGo call, so a local variable would allocate on every
// call. USBVBusPresent is assumed to be called from a single goroutine.
var usbregstatus C.uint32_t

func USBVBusPresent() bool {
	if C.sd_power_usbregstatus_get(&usbregstatus) != 0 {
		return false
	}
	return usbregstatus&1 != 0 // VBUSDETECT
}
