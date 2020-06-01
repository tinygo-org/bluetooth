// +build softdevice,s110v8

package bluetooth

/*
// Define SoftDevice functions as regular function declarations (not inline
// static functions).
#define SVCALL_AS_NORMAL_FUNCTION

#include "ble_gap.h"
*/
import "C"

// Advertisement encapsulates a single advertisement instance.
type Advertisement struct {
	interval AdvertisementInterval
}

var globalAdvertisement Advertisement

// NewAdvertisement creates a new advertisement instance but does not configure
// it. It can be called before the SoftDevice has been initialized.
//
// On the nrf51 only one advertisement is allowed at a given time, therefore
// this is a singleton.
func (a *Adapter) NewAdvertisement() *Advertisement {
	return &globalAdvertisement
}

// Configure this advertisement. Must be called after SoftDevice initialization.
func (a *Advertisement) Configure(options AdvertisementOptions) error {
	var payload rawAdvertisementPayload
	payload.addFlags(0x06)
	if options.LocalName != "" {
		if !payload.addCompleteLocalName(options.LocalName) {
			return errAdvertisementPacketTooBig
		}
	}
	errCode := C.sd_ble_gap_adv_data_set(&payload.data[0], payload.len, nil, 0)
	a.interval = options.Interval
	return makeError(errCode)
}

// Start advertisement. May only be called after it has been configured.
func (a *Advertisement) Start() error {
	params := C.ble_gap_adv_params_t{
		_type:    C.BLE_GAP_ADV_TYPE_ADV_IND,
		fp:       C.BLE_GAP_ADV_FP_ANY,
		interval: uint16(a.interval),
		timeout:  0, // no timeout
	}
	errCode := C.sd_ble_gap_adv_start(&params)
	return makeError(errCode)
}
