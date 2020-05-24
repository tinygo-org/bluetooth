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
	interval AdvertiseInterval
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
func (a *Advertisement) Configure(broadcastData, scanResponseData []byte, options *AdvertiseOptions) error {
	var (
		p_data    *byte
		dlen      byte
		p_sr_data *byte
		srdlen    byte
	)
	if broadcastData != nil {
		p_data = &broadcastData[0]
		dlen = uint8(len(broadcastData))
	}
	if scanResponseData != nil {
		p_sr_data = &scanResponseData[0]
		srdlen = uint8(len(scanResponseData))
	}
	errCode := C.sd_ble_gap_adv_data_set(p_data, dlen, p_sr_data, srdlen)
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
