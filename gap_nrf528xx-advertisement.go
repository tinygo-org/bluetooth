//go:build (softdevice && s113v7) || (softdevice && s132v6) || (softdevice && s140v6) || (softdevice && s140v7)

package bluetooth

import (
	"runtime/volatile"
	"time"
	"unsafe"
)

/*
#include "ble_gap.h"
*/
import "C"

// Address contains a Bluetooth MAC address.
type Address struct {
	MACAddress
}

// Advertisement encapsulates a single advertisement instance.
type Advertisement struct {
	handle        C.uint8_t
	isAdvertising volatile.Register8
	payload       rawAdvertisementPayload
}

// The nrf528xx devices only seem to support one advertisement instance. The way
// multiple advertisements are implemented is by changing the packet data
// frequently.
var defaultAdvertisement = Advertisement{
	handle: C.BLE_GAP_ADV_SET_HANDLE_NOT_SET,
}

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
	// Note that the payload needs to be part of the Advertisement object as the
	// memory is still used after sd_ble_gap_adv_set_configure returns.
	a.payload.reset()
	if !a.payload.addFromOptions(options) {
		return errAdvertisementPacketTooBig
	}

	var typ uint8
	switch options.AdvertisementType {
	case AdvertisingTypeInd:
		typ = C.BLE_GAP_ADV_TYPE_CONNECTABLE_SCANNABLE_UNDIRECTED
	case AdvertisingTypeDirectInd:
		typ = C.BLE_GAP_ADV_TYPE_CONNECTABLE_NONSCANNABLE_DIRECTED
	case AdvertisingTypeScanInd:
		typ = C.BLE_GAP_ADV_TYPE_NONCONNECTABLE_SCANNABLE_UNDIRECTED
	case AdvertisingTypeNonConnInd:
		typ = C.BLE_GAP_ADV_TYPE_NONCONNECTABLE_NONSCANNABLE_UNDIRECTED
	}

	data := C.ble_gap_adv_data_t{}
	data.adv_data = C.ble_data_t{
		p_data: (*C.uint8_t)(unsafe.Pointer(&a.payload.data[0])),
		len:    C.uint16_t(a.payload.len),
	}
	params := C.ble_gap_adv_params_t{
		properties: C.ble_gap_adv_properties_t{
			_type: typ,
		},
		interval: C.uint32_t(options.Interval),
	}
	errCode := C.sd_ble_gap_adv_set_configure(&a.handle, &data, &params)
	return makeError(errCode)
}

// Start advertisement. May only be called after it has been configured.
func (a *Advertisement) Start() error {
	a.isAdvertising.Set(1)
	errCode := C.sd_ble_gap_adv_start(a.handle, C.BLE_CONN_CFG_TAG_DEFAULT)
	return makeError(errCode)
}

// Stop advertisement.
func (a *Advertisement) Stop() error {
	a.isAdvertising.Set(0)
	errCode := C.sd_ble_gap_adv_stop(a.handle)
	return makeError(errCode)
}

// SetRandomAddress sets the random address to be used for advertising.
func (a *Adapter) SetRandomAddress(mac MAC) error {
	var addr C.ble_gap_addr_t
	addr.addr = makeSDAddress(mac)
	addr.set_bitfield_addr_type(C.BLE_GAP_ADDR_TYPE_RANDOM_STATIC)

	errCode := C.sd_ble_gap_addr_set(&addr)
	if errCode != 0 {
		return Error(errCode)
	}
	return nil
}
