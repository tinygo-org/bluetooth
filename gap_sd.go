// +build softdevice,s132v6

package bluetooth

/*
// Define SoftDevice functions as regular function declarations (not inline
// static functions).
#define SVCALL_AS_NORMAL_FUNCTION

#include "s132_nrf52_6.1.1/s132_nrf52_6.1.1_API/include/ble_gap.h"
*/
import "C"

// NewAdvertisement creates a new advertisement instance but does not configure
// it. It can be called before the SoftDevice has been initialized.
func (a *Adapter) NewAdvertisement() *Advertisement {
	return &Advertisement{
		handle: C.BLE_GAP_ADV_SET_HANDLE_NOT_SET,
	}
}

// Configure this advertisement. Must be called after SoftDevice initialization.
func (a *Advertisement) Configure(broadcastData, scanResponseData []byte, options *AdvertiseOptions) error {
	data := C.ble_gap_adv_data_t{}
	if broadcastData != nil {
		data.adv_data = C.ble_data_t{
			p_data: &broadcastData[0],
			len:    uint16(len(broadcastData)),
		}
	}
	if scanResponseData != nil {
		data.scan_rsp_data = C.ble_data_t{
			p_data: &scanResponseData[0],
			len:    uint16(len(scanResponseData)),
		}
	}
	params := C.ble_gap_adv_params_t{
		properties: C.ble_gap_adv_properties_t{
			_type: C.BLE_GAP_ADV_TYPE_CONNECTABLE_SCANNABLE_UNDIRECTED,
		},
		interval: uint32(options.Interval),
	}
	errCode := C.sd_ble_gap_adv_set_configure(&a.handle, &data, &params)
	return makeError(errCode)
}

// Start advertisement. May only be called after it has been configured.
func (a *Advertisement) Start() error {
	errCode := C.sd_ble_gap_adv_start(a.handle, C.BLE_CONN_CFG_TAG_DEFAULT)
	return makeError(errCode)
}
