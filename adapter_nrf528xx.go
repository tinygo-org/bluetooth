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

// TODO: Probably it should be in adapter_sd, but as it's usage is added only for nrf528xx-full.go
// as well as i do not have other machines to test, adding it here for now

type GapIOCapability uint8

const (
	DisplayOnlyGapIOCapability     = C.BLE_GAP_IO_CAPS_DISPLAY_ONLY
	DisplayYesNoGapIOCapability    = C.BLE_GAP_IO_CAPS_DISPLAY_YESNO
	KeyboardOnlyGapIOCapability    = C.BLE_GAP_IO_CAPS_KEYBOARD_ONLY
	NoneGapIOCapability            = C.BLE_GAP_IO_CAPS_NONE
	KeyboardDisplayGapIOCapability = C.BLE_GAP_IO_CAPS_KEYBOARD_DISPLAY
)

var (
	secParams = C.ble_gap_sec_params_t{
		min_key_size: 7, // not sure if those are the best default length
		max_key_size: 16,
	}

	secKeySet C.ble_gap_sec_keyset_t = C.ble_gap_sec_keyset_t{
		keys_peer: C.ble_gap_sec_keys_t{
			p_enc_key:  &C.ble_gap_enc_key_t{},   /**< Encryption Key, or NULL. */
			p_id_key:   &C.ble_gap_id_key_t{},    /**< Identity Key, or NULL. */
			p_sign_key: &C.ble_gap_sign_info_t{}, /**< Signing Key, or NULL. */
			p_pk:       &C.ble_gap_lesc_p256_pk_t{},
		},
		keys_own: C.ble_gap_sec_keys_t{
			p_enc_key:  &C.ble_gap_enc_key_t{},   /**< Encryption Key, or NULL. */
			p_id_key:   &C.ble_gap_id_key_t{},    /**< Identity Key, or NULL. */
			p_sign_key: &C.ble_gap_sign_info_t{}, /**< Signing Key, or NULL. */
			p_pk:       &C.ble_gap_lesc_p256_pk_t{},
		},
	}
)

// are those should be methods for adapter as they are relevant for sd only
func SetSecParamsBonding() {
	secParams.set_bitfield_bond(1)
}

func SetSecCapabilities(cap GapIOCapability) {
	secParams.set_bitfield_io_caps(uint8(cap))
}

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
	appRAMBase := uint32(uintptr(unsafe.Pointer(&appRAMBase)))
	errCode = C.sd_ble_enable(&appRAMBase)
	return makeError(errCode)
}

func (a *Adapter) Address() (MACAddress, error) {
	var addr C.ble_gap_addr_t
	errCode := C.sd_ble_gap_addr_get(&addr)
	if errCode != 0 {
		return MACAddress{}, Error(errCode)
	}
	return MACAddress{MAC: addr.addr}, nil
}
