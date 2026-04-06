//go:build softdevice && s113v7

package bluetooth

/*
// Add the correct SoftDevice include path to CFLAGS, so #include will work as
// expected.
#cgo CFLAGS: -Is113_nrf52_7.0.1/s113_nrf52_7.0.1_API/include

#include "nrf_nvic.h"
nrf_nvic_state_t nrf_nvic_state = {0};
*/
import "C"

// Connect starts a connection attempt to the given peripheral device address.
//
// s113v7 is a peripheral-only device, so this is not supported.
func (a *Adapter) Connect(address Address, params ConnectionParams) (Device, error) {
	return Device{}, errNotSupported
}

// Scan starts a BLE scan. It is stopped by a call to StopScan.
//
// s113v7 is a peripheral-only device, so this is not supported.
func (a *Adapter) Scan(callback func(*Adapter, ScanResult)) (err error) {
	return errNotSupported
}

// StopScan stops any in-progress scan.
//
// s113v7 is a peripheral-only device, so this is not supported.
func (a *Adapter) StopScan() error {
	return errNotSupported
}
