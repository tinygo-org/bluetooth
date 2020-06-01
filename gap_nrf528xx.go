// +build softdevice,!s110v8

package bluetooth

import (
	"device/arm"
	"runtime/volatile"
)

/*
// Define SoftDevice functions as regular function declarations (not inline
// static functions).
#define SVCALL_AS_NORMAL_FUNCTION

#include "ble_gap.h"
*/
import "C"

// Memory buffers needed by sd_ble_gap_scan_start.
var (
	scanReportBuffer rawAdvertisementPayload
	gotScanReport    volatile.Register8
	globalScanResult ScanResult
)

// Advertisement encapsulates a single advertisement instance.
type Advertisement struct {
	handle        uint8
	isAdvertising volatile.Register8
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
	data := C.ble_gap_adv_data_t{}
	var payload rawAdvertisementPayload
	payload.addFlags(0x06)
	if options.LocalName != "" {
		if !payload.addCompleteLocalName(options.LocalName) {
			return errAdvertisementPacketTooBig
		}
	}
	data.adv_data = C.ble_data_t{
		p_data: &payload.data[0],
		len:    uint16(payload.len),
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
	a.isAdvertising.Set(1)
	errCode := C.sd_ble_gap_adv_start(a.handle, C.BLE_CONN_CFG_TAG_DEFAULT)
	return makeError(errCode)
}

// Scan starts a BLE scan. It is stopped by a call to StopScan. A common pattern
// is to cancel the scan when a particular device has been found.
func (a *Adapter) Scan(callback func(*Adapter, ScanResult)) error {
	if a.scanning {
		// There is a possible race condition here if Scan() is called from a
		// different goroutine, but that is not allowed (and will likely result
		// in an error below anyway).
		return errScanning
	}
	a.scanning = true

	scanParams := C.ble_gap_scan_params_t{}
	scanParams.set_bitfield_extended(0)
	scanParams.set_bitfield_active(0)
	scanParams.interval = 100 * 1000 / 625 // 100ms in 625µs units
	scanParams.window = 100 * 1000 / 625   // 100ms in 625µs units
	scanParams.timeout = C.BLE_GAP_SCAN_TIMEOUT_UNLIMITED
	scanReportBufferInfo := C.ble_data_t{
		p_data: &scanReportBuffer.data[0],
		len:    uint16(len(scanReportBuffer.data)),
	}
	errCode := C.sd_ble_gap_scan_start(&scanParams, &scanReportBufferInfo)
	if errCode != 0 {
		return Error(errCode)
	}

	// Wait for received scan reports.
	for a.scanning {
		// Wait for the next advertisement packet to arrive.
		// TODO: use some sort of condition variable once the scheduler supports
		// them.
		arm.Asm("wfe")
		if gotScanReport.Get() == 0 {
			// Spurious event. Continue waiting.
			continue
		}
		gotScanReport.Set(0)

		// Call the callback with the scan result.
		callback(a, globalScanResult)

		// Restart the advertisement. This is needed, because advertisements are
		// automatically stopped when the first packet arrives.
		errCode := C.sd_ble_gap_scan_start(nil, &scanReportBufferInfo)
		if errCode != 0 {
			return Error(errCode)
		}
	}
	return nil
}

// StopScan stops any in-progress scan. It can be called from within a Scan
// callback to stop the current scan. If no scan is in progress, an error will
// be returned.
func (a *Adapter) StopScan() error {
	if !a.scanning {
		return errNotScanning
	}
	a.scanning = false
	return nil
}
