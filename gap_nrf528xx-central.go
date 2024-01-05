//go:build (softdevice && s132v6) || (softdevice && s140v6) || (softdevice && s140v7)

package bluetooth

import (
	"device/arm"
	"errors"
	"runtime/volatile"
	"time"
	"unsafe"
)

/*
#include "ble_gap.h"
*/
import "C"

var errAlreadyConnecting = errors.New("bluetooth: already in a connection attempt")
var errConnectionTimeout = errors.New("bluetooth: timeout while connecting")

// Memory buffers needed by sd_ble_gap_scan_start.
var (
	scanReportBuffer rawAdvertisementPayload
	gotScanReport    volatile.Register8
	globalScanResult ScanResult
)

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
	scanParams.interval = C.uint16_t(NewDuration(40 * time.Millisecond))
	scanParams.window = C.uint16_t(NewDuration(30 * time.Millisecond))
	scanParams.timeout = C.BLE_GAP_SCAN_TIMEOUT_UNLIMITED
	scanReportBufferInfo := C.ble_data_t{
		p_data: (*C.uint8_t)(unsafe.Pointer(&scanReportBuffer.data[0])),
		len:    C.uint16_t(len(scanReportBuffer.data)),
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

// In-progress connection attempt.
var connectionAttempt struct {
	state            volatile.Register8 // 0 means unused, 1 means connecting, 2 means connected, 3 means timeout
	connectionHandle C.uint16_t
}

// Connect starts a connection attempt to the given peripheral device address.
//
// Limitations on Nordic SoftDevices inclue that you cannot do more than one
// connection attempt at once and that the address parameter must have the
// IsRandom bit set correctly. This bit is set correctly for scan results, so
// you can reuse that address directly.
func (a *Adapter) Connect(address Address, params ConnectionParams) (Device, error) {
	// Construct an address object as used in the SoftDevice.
	var addr C.ble_gap_addr_t
	addr.addr = makeSDAddress(address.MAC)
	if address.IsRandom() {
		switch address.MAC[5] >> 6 {
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
	scanParams.interval = C.uint16_t(NewDuration(40 * time.Millisecond))
	scanParams.window = C.uint16_t(NewDuration(30 * time.Millisecond))
	scanParams.timeout = C.uint16_t(params.ConnectionTimeout / 16) // timeout in 10ms units

	connectionParams := C.ble_gap_conn_params_t{
		min_conn_interval: C.uint16_t(params.MinInterval) / 2,
		max_conn_interval: C.uint16_t(params.MaxInterval) / 2,
		slave_latency:     0,   // mostly relevant to connected keyboards etc
		conn_sup_timeout:  200, // 2 seconds (in 10ms units), the minimum recommended by Apple
	}
	if params.Timeout != 0 {
		connectionParams.conn_sup_timeout = uint16(params.Timeout / 16)
	}

	// Flag to the event handler that we are waiting for incoming connections.
	// This should be safe as long as Connect is not called concurrently. And
	// even then, it should catch most such race conditions.
	if connectionAttempt.state.Get() != 0 {
		return Device{}, errAlreadyConnecting
	}
	connectionAttempt.state.Set(1)

	// Start the connection attempt. We'll get a signal in the event handler.
	errCode := C.sd_ble_gap_connect(&addr, &scanParams, &connectionParams, C.BLE_CONN_CFG_TAG_DEFAULT)
	if errCode != 0 {
		connectionAttempt.state.Set(0)
		return Device{}, Error(errCode)
	}

	// Wait until the connection is established.
	for {
		state := connectionAttempt.state.Get()
		if state == 2 {
			// Successfully connected.
			connectionAttempt.state.Set(0)
			connectionHandle := connectionAttempt.connectionHandle
			return Device{
				connectionHandle: connectionHandle,
			}, nil
		} else if state == 3 {
			// Timeout while connecting.
			connectionAttempt.state.Set(0)
			return Device{}, errConnectionTimeout
		} else {
			// TODO: use some sort of condition variable once the scheduler
			// supports them.
			arm.Asm("wfe")
		}
	}
}

// Disconnect from the BLE device.
func (d Device) Disconnect() error {
	errCode := C.sd_ble_gap_disconnect(d.connectionHandle, C.BLE_HCI_REMOTE_USER_TERMINATED_CONNECTION)
	if errCode != 0 {
		return Error(errCode)
	}

	return nil
}

// RequestConnectionParams requests a different connection latency and timeout
// of the given device connection. Fields that are unset will be left alone.
// Whether or not the device will actually honor this, depends on the device and
// on the specific parameters.
//
// On the Nordic SoftDevice, this call will also set the slave latency to 0.
func (d Device) RequestConnectionParams(params ConnectionParams) error {
	// The default parameters if no specific parameters are picked.
	connParams := C.ble_gap_conn_params_t{
		min_conn_interval: C.BLE_GAP_CP_MIN_CONN_INTVL_NONE,
		max_conn_interval: C.BLE_GAP_CP_MAX_CONN_INTVL_NONE,
		slave_latency:     0,
		conn_sup_timeout:  C.BLE_GAP_CP_CONN_SUP_TIMEOUT_NONE,
	}

	// Use specified parameters if available.
	if params.MinInterval != 0 {
		connParams.min_conn_interval = C.uint16_t(params.MinInterval) / 2
	}
	if params.MaxInterval != 0 {
		connParams.max_conn_interval = C.uint16_t(params.MaxInterval) / 2
	}
	if params.Timeout != 0 {
		connParams.conn_sup_timeout = C.uint16_t(params.Timeout) / 16
	}

	// Send them to peer device.
	errCode := C.sd_ble_gap_conn_param_update(d.connectionHandle, &connParams)
	return makeError(errCode)
}
