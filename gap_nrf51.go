// +build softdevice,s110v8

package bluetooth

/*
// Define SoftDevice functions as regular function declarations (not inline
// static functions).
#define SVCALL_AS_NORMAL_FUNCTION

#include "ble_gap.h"
*/
import "C"

import (
	"runtime/volatile"
	"time"
)

// Address contains a Bluetooth MAC address.
type Address struct {
	MACAddress
}

// Advertisement encapsulates a single advertisement instance.
type Advertisement struct {
	interval      Duration
	isAdvertising volatile.Register8
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

	errCode := C.sd_ble_gap_adv_data_set(&payload.data[0], payload.len, nil, 0)
	a.interval = options.Interval
	return makeError(errCode)
}

// Start advertisement. May only be called after it has been configured.
func (a *Advertisement) Start() error {
	a.isAdvertising.Set(1)
	errCode := a.start()
	return makeError(errCode)
}

// Low-level version of Start. Used to restart advertisement when a connection
// is lost.
func (a *Advertisement) start() uint32 {
	params := C.ble_gap_adv_params_t{
		_type:    C.BLE_GAP_ADV_TYPE_ADV_IND,
		fp:       C.BLE_GAP_ADV_FP_ANY,
		interval: uint16(a.interval),
		timeout:  0, // no timeout
	}
	return C.sd_ble_gap_adv_start(&params)
}
