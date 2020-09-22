// +build softdevice,!s110v8

package bluetooth

import (
	"device/arm"
	"errors"
	"runtime/volatile"
	"time"
)

/*
// Define SoftDevice functions as regular function declarations (not inline
// static functions).
#define SVCALL_AS_NORMAL_FUNCTION

#include "ble_gap.h"
*/
import "C"

var errAlreadyConnecting = errors.New("bluetooth: already in a connection attempt")

// Memory buffers needed by sd_ble_gap_scan_start.
var (
	scanReportBuffer rawAdvertisementPayload
	gotScanReport    volatile.Register8
	globalScanResult ScanResult
)

// Address contains a Bluetooth MAC address.
type Address struct {
	MACAddress
}

// Advertisement encapsulates a single advertisement instance.
type Advertisement struct {
	handle        uint8
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

	data := C.ble_gap_adv_data_t{}
	data.adv_data = C.ble_data_t{
		p_data: &a.payload.data[0],
		len:    uint16(a.payload.len),
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
//
// The callback is run on the same goroutine as the Scan function when using a
// SoftDevice.
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
	scanParams.interval = uint16(NewDuration(40 * time.Millisecond))
	scanParams.window = uint16(NewDuration(30 * time.Millisecond))
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

	// TODO: stop immediately, not when the next scan result arrives.

	return nil
}

// Device is a connection to a remote peripheral.
type Device struct {
	connectionHandle uint16
}

// In-progress connection attempt.
var connectionAttempt struct {
	state            volatile.Register8 // 0 means unused, 1 means connecting, 2 means ready (connected or timeout)
	connectionHandle uint16
}

// Connect starts a connection attempt to the given peripheral device address.
//
// Limitations on Nordic SoftDevices inclue that you cannot do more than one
// connection attempt at once and that the address parameter must have the
// IsRandom bit set correctly. This bit is set correctly for scan results, so
// you can reuse that address directly.
func (a *Adapter) Connect(address Addresser, params ConnectionParams) (*Device, error) {
	adr := address.(Address)
	// Construct an address object as used in the SoftDevice.
	var addr C.ble_gap_addr_t
	addr.addr = adr.MAC
	if address.IsRandom() {
		switch adr.MAC[5] >> 6 {
		case 0b11:
			addr.set_bitfield_addr_type(C.BLE_GAP_ADDR_TYPE_RANDOM_STATIC)
		case 0b01:
			addr.set_bitfield_addr_type(C.BLE_GAP_ADDR_TYPE_RANDOM_PRIVATE_RESOLVABLE)
		case 0b00:
			addr.set_bitfield_addr_type(C.BLE_GAP_ADDR_TYPE_RANDOM_PRIVATE_NON_RESOLVABLE)
		}
	}

	// Pick default values if some parameters aren't specified.
	if params.ConnectionTimeout == 0 {
		params.ConnectionTimeout = NewDuration(4 * time.Second)
	}
	if params.MinInterval == 0 && params.MaxInterval == 0 {
		// Pick some semi-arbitrary range if these values haven't been
		// configured. The values have been picked to be compliant with the
		// guidelines from Apple (section 35.6 Connection Parameters):
		// https://developer.apple.com/accessories/Accessory-Design-Guidelines.pdf
		params.MinInterval = NewDuration(15 * time.Millisecond)
		params.MaxInterval = NewDuration(150 * time.Millisecond)
	}

	// Set scan params, presumably these parameters are used to re-scan for the
	// device to connect to because only right after an advertisement has been
	// received is the device connectable.
	scanParams := C.ble_gap_scan_params_t{}
	scanParams.set_bitfield_extended(0)
	scanParams.set_bitfield_active(0)
	scanParams.interval = uint16(NewDuration(40 * time.Millisecond))
	scanParams.window = uint16(NewDuration(30 * time.Millisecond))
	scanParams.timeout = uint16(params.ConnectionTimeout)

	connectionParams := C.ble_gap_conn_params_t{
		min_conn_interval: uint16(params.MinInterval) / 2,
		max_conn_interval: uint16(params.MaxInterval) / 2,
		slave_latency:     0,   // mostly relevant to connected keyboards etc
		conn_sup_timeout:  200, // 2 seconds (in 10ms units), the minimum recommended by Apple
	}

	// Flag to the event handler that we are waiting for incoming connections.
	// This should be safe as long as Connect is not called concurrently. And
	// even then, it should catch most such race conditions.
	if connectionAttempt.state.Get() != 0 {
		return nil, errAlreadyConnecting
	}
	connectionAttempt.state.Set(1)

	// Start the connection attempt. We'll get a signal in the event handler.
	errCode := C.sd_ble_gap_connect(&addr, &scanParams, &connectionParams, C.BLE_CONN_CFG_TAG_DEFAULT)
	if errCode != 0 {
		connectionAttempt.state.Set(0)
		return nil, Error(errCode)
	}

	// Wait until the connection is established.
	// TODO: use some sort of condition variable once the scheduler supports
	// them.
	for connectionAttempt.state.Get() != 2 {
		arm.Asm("wfe")
	}
	connectionHandle := connectionAttempt.connectionHandle
	connectionAttempt.state.Set(0)

	// Connection has been established.
	return &Device{
		connectionHandle: connectionHandle,
	}, nil
}

// Disconnect from the BLE device.
func (d *Device) Disconnect() error {
	errCode := C.sd_ble_gap_disconnect(d.connectionHandle, C.BLE_HCI_REMOTE_USER_TERMINATED_CONNECTION)
	if errCode != 0 {
		return Error(errCode)
	}

	return nil
}
