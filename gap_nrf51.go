//go:build softdevice && s110v8

package bluetooth

/*
#include "ble_gap.h"

// Workaround wrapper function to avoid pointer arguments escaping to heap
static inline uint32_t sd_ble_gap_adv_start_noescape(ble_gap_adv_params_t const p_adv_params) {
	return sd_ble_gap_adv_start(&p_adv_params);
}
*/
import "C"

import (
	"runtime/volatile"
	"time"
	"unsafe"
)

// Address contains a Bluetooth MAC address.
type Address struct {
	MACAddress
}

// Advertisement encapsulates a single advertisement instance.
type Advertisement struct {
	interval      Duration
	isAdvertising volatile.Register8
	typ           uint8
}

var defaultAdvertisement Advertisement

// DefaultAdvertisement returns the default advertisement instance but does not
// configure it.
func (a *Adapter) DefaultAdvertisement() *Advertisement {
	return &defaultAdvertisement
}

// Configure this advertisement.
func (a *Advertisement) Configure(options AdvertisementOptions) error {
	// Fill empty options with reasonable defaults.
	if options.Interval == 0 {
		// Pick an advertisement interval recommended by Apple (section 35.5
		// Advertising Interval):
		// https://developer.apple.com/accessories/Accessory-Design-Guidelines.pdf
		options.Interval = NewDuration(152500 * time.Microsecond) // 152.5ms
	}

	// Construct payload.
	var payload rawAdvertisementPayload
	if !payload.addFromOptions(options) {
		return errAdvertisementPacketTooBig
	}

	switch options.AdvertisementType {
	case AdvertisingTypeInd:
		a.typ = C.BLE_GAP_ADV_TYPE_ADV_IND
	case AdvertisingTypeDirectInd:
		a.typ = C.BLE_GAP_ADV_TYPE_ADV_DIRECT_IND
	case AdvertisingTypeScanInd:
		a.typ = C.BLE_GAP_ADV_TYPE_ADV_SCAN_IND
	case AdvertisingTypeNonConnInd:
		a.typ = C.BLE_GAP_ADV_TYPE_ADV_NONCONN_IND
	}

	errCode := C.sd_ble_gap_adv_data_set((*C.uint8_t)(unsafe.Pointer(&payload.data[0])), C.uint8_t(payload.len), nil, 0)
	a.interval = options.Interval
	return makeError(errCode)
}

// Start advertisement. May only be called after it has been configured.
func (a *Advertisement) Start() error {
	a.isAdvertising.Set(1)
	errCode := a.start()
	return makeError(errCode)
}

// Stop advertisement.
func (a *Advertisement) Stop() error {
	a.isAdvertising.Set(0)
	errCode := C.sd_ble_gap_adv_stop()
	return makeError(errCode)
}

// Low-level version of Start. Used to restart advertisement when a connection
// is lost.
func (a *Advertisement) start() C.uint32_t {
	params := C.ble_gap_adv_params_t{
		_type:    a.typ,
		fp:       C.BLE_GAP_ADV_FP_ANY,
		interval: C.uint16_t(a.interval),
		timeout:  0, // no timeout
	}
	return C.sd_ble_gap_adv_start_noescape(params)
}

// SetRandomAddress sets the random address to be used for advertising.
func (a *Adapter) SetRandomAddress(mac MAC) error {
	var addr C.ble_gap_addr_t
	addr.addr = makeSDAddress(mac)
	addr.addr_type = C.BLE_GAP_ADDR_TYPE_RANDOM_STATIC

	errCode := C.sd_ble_gap_address_set(C.BLE_GAP_ADDR_CYCLE_MODE_NONE, &addr)
	if errCode != 0 {
		return Error(errCode)
	}
	return nil
}
