//go:build (softdevice && s113v7) || (softdevice && s132v6) || (softdevice && s140v6) || (softdevice && s140v7)

package bluetooth

// This file defines the SoftDevice adapter for all nrf52-series chips.

/*
#include "nrf_sdm.h"
#include "nrf_nvic.h"
#include "ble.h"
#include "ble_gap.h"

void assertHandler(void);
*/
import "C"

import (
	"machine"
	"unsafe"
)

//export assertHandler
func assertHandler() {
	println("SoftDevice assert")
}

var clockConfigXtal C.nrf_clock_lf_cfg_t = C.nrf_clock_lf_cfg_t{
	source:       C.NRF_CLOCK_LF_SRC_XTAL,
	rc_ctiv:      0,
	rc_temp_ctiv: 0,
	accuracy:     C.NRF_CLOCK_LF_ACCURACY_250_PPM,
}

//go:extern __app_ram_base
var appRAMBase [0]uint32

func (a *Adapter) enable() error {
	// Enable the SoftDevice.
	var clockConfig *C.nrf_clock_lf_cfg_t
	if machine.HasLowFrequencyCrystal {
		clockConfig = &clockConfigXtal
	}
	errCode := C.sd_softdevice_enable(clockConfig, C.nrf_fault_handler_t(C.assertHandler))
	if errCode != 0 {
		return Error(errCode)
	}

	// Enable the BLE stack.
	appRAMBase := C.uint32_t(uintptr(unsafe.Pointer(&appRAMBase)))
	errCode = C.sd_ble_enable(&appRAMBase)
	return makeError(errCode)
}

func (a *Adapter) Address() (MACAddress, error) {
	var addr C.ble_gap_addr_t
	errCode := C.sd_ble_gap_addr_get(&addr)
	if errCode != 0 {
		return MACAddress{}, Error(errCode)
	}
	return MACAddress{MAC: makeAddress(addr.addr)}, nil
}

// Convert a C.ble_gap_addr_t to a MACAddress struct.
func makeMACAddress(addr C.ble_gap_addr_t) MACAddress {
	return MACAddress{
		MAC:      makeAddress(addr.addr),
		isRandom: addr.bitfield_addr_type() != 0,
	}
}
