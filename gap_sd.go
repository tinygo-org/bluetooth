//go:build softdevice

package bluetooth

/*
#include "ble_gap.h"
*/
import "C"

var _ DeviceInterface = Device{}

// Device is a connection to a remote peripheral or central.
type Device struct {
	Address Address

	connectionHandle C.uint16_t
}

// Disconnect from the BLE device.
func (d Device) Disconnect() error {
	return errNotYetImplmented
}

// DiscoverServices starts a service discovery procedure.
func (d Device) DiscoverServices(uuids []UUID) ([]DeviceService, error) {
	return nil, errNotYetImplmented
}

// RequestConnectionParams requests a different connection latency and timeout
// of the given device connection.
func (d Device) RequestConnectionParams(params ConnectionParams) error {
	return errNotYetImplmented
}
