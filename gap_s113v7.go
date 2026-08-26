//go:build softdevice && s113v7

package bluetooth

/*
// Add the correct SoftDevice include path to CFLAGS, so #include will work as
// expected.
#cgo CFLAGS: -Is113_nrf52_7.0.1/s113_nrf52_7.0.1_API/include
*/
import "C"

import "context"

// Disconnect from the BLE device.
//
// s113v7 is a peripheral-only device, so this is not supported.
func (d Device) Disconnect() error {
	return errNotSupported
}

// DiscoverServicesWithContext starts a service discovery procedure, abandoning
// it if ctx is cancelled.
//
// s113v7 is a peripheral-only device, so this is not supported.
func (d Device) DiscoverServicesWithContext(ctx context.Context, uuids []UUID) ([]DeviceService, error) {
	return nil, errNotSupported
}

// DiscoverServices starts a service discovery procedure.
//
// s113v7 is a peripheral-only device, so this is not supported.
func (d Device) DiscoverServices(uuids []UUID) ([]DeviceService, error) {
	return nil, errNotSupported
}

// RequestConnectionParams requests a different connection latency and timeout
// of the given device connection.
//
// s113v7 is a peripheral-only device, so this is not supported.
func (d Device) RequestConnectionParams(params ConnectionParams) error {
	return errNotSupported
}

// Connected returns whether the device is currently connected.
//
// s113v7 is a peripheral-only device, so this is not supported.
func (d Device) Connected() (bool, error) {
	return false, errNotSupported
}

type DeviceService struct{}
