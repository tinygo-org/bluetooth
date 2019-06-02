// +build softdevice,s132v6

package bluetooth

/*
// Define SoftDevice functions as regular function declarations (not inline
// static functions).
#define SVCALL_AS_NORMAL_FUNCTION

#include "s132_nrf52_6.1.1/s132_nrf52_6.1.1_API/include/nrf_sdm.h"
#include "s132_nrf52_6.1.1/s132_nrf52_6.1.1_API/include/ble.h"
#include "s132_nrf52_6.1.1/s132_nrf52_6.1.1_API/include/ble_gap.h"

void assertHandler(void);
*/
import "C"

import (
	"device/arm"
	"device/nrf"
	"unsafe"
)

//export assertHandler
func assertHandler() {
	println("SoftDevice assert")
}

var clockConfig C.nrf_clock_lf_cfg_t = C.nrf_clock_lf_cfg_t{
	source:       C.NRF_CLOCK_LF_SRC_SYNTH,
	rc_ctiv:      0,
	rc_temp_ctiv: 0,
	accuracy:     0,
}

var (
	secModeOpen       C.ble_gap_conn_sec_mode_t
	defaultDeviceName = [6]byte{'T', 'i', 'n', 'y', 'G', 'o'}
)

// Globally allocated buffer for incoming SoftDevice events.
var eventBuf struct {
	C.ble_evt_t
	buf [23]byte
}

func init() {
	secModeOpen.set_bitfield_sm(1)
	secModeOpen.set_bitfield_lv(1)
}

// Adapter is a dummy adapter: it represents the connection to the (only)
// SoftDevice on the chip.
type Adapter struct {
}

// DefaultAdapter is an adapter to the default Bluetooth stack on a given
// target.
var DefaultAdapter = &Adapter{}

// Enable configures the BLE stack. It must be called before any
// Bluetooth-related calls (unless otherwise indicated).
func (a *Adapter) Enable() error {
	// Enable the IRQ that handles all events.
	arm.EnableIRQ(nrf.IRQ_SWI2)
	arm.SetPriority(nrf.IRQ_SWI2, 192)

	errCode := C.sd_softdevice_enable(&clockConfig, C.nrf_fault_handler_t(C.assertHandler))
	if errCode != 0 {
		return Error(errCode)
	}

	appRAMBase := uint32(0x200039c0)
	errCode = C.sd_ble_enable(&appRAMBase)
	if errCode != 0 {
		return Error(errCode)
	}

	errCode = C.sd_ble_gap_device_name_set(&secModeOpen, &defaultDeviceName[0], uint16(len(defaultDeviceName)))
	if errCode != 0 {
		return Error(errCode)
	}

	var gapConnParams C.ble_gap_conn_params_t
	gapConnParams.min_conn_interval = C.BLE_GAP_CP_MIN_CONN_INTVL_MIN
	gapConnParams.max_conn_interval = C.BLE_GAP_CP_MIN_CONN_INTVL_MAX
	gapConnParams.slave_latency = 0
	gapConnParams.conn_sup_timeout = C.BLE_GAP_CP_CONN_SUP_TIMEOUT_NONE

	errCode = C.sd_ble_gap_ppcp_set(&gapConnParams)
	if errCode != 0 {
		return Error(errCode)
	}

	return nil
}

func handleEvent() {
	// TODO: do something with the events.
}

//go:export SWI2_EGU2_IRQHandler
func handleInterrupt() {
	for {
		eventBufLen := uint16(unsafe.Sizeof(eventBuf))
		errCode := C.sd_ble_evt_get((*uint8)(unsafe.Pointer(&eventBuf)), &eventBufLen)
		if errCode != 0 {
			// Possible error conditions:
			//  * NRF_ERROR_NOT_FOUND: no events left, break
			//  * NRF_ERROR_DATA_SIZE: retry with a bigger data buffer
			//    (currently not handled, TODO)
			//  * NRF_ERROR_INVALID_ADDR: pointer is not aligned, should
			//    not happen.
			// In all cases, it's best to simply stop now.
			break
		}
		handleEvent()
	}
}
