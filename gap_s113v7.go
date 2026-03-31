//go:build softdevice && s113v7

package bluetooth

/*
// Add the correct SoftDevice include path to CFLAGS, so #include will work as
// expected.
#cgo CFLAGS: -Is113_nrf52_7.0.1/s113_nrf52_7.0.1_API/include
*/
import "C"

// Disconnect from the BLE device.
//
// Not yet implemented on the s113v7.
func (d Device) Disconnect() error {
	return errNotYetImplmented
}

// DiscoverServices starts a service discovery procedure.
//
// Not yet implemented on the s113v7.
func (d Device) DiscoverServices(uuids []UUID) ([]DeviceService, error) {
	return nil, errNotYetImplmented
}

// RequestConnectionParams requests a different connection latency and timeout
// of the given device connection.
//
// Not yet implemented on the s113v7.
func (d Device) RequestConnectionParams(params ConnectionParams) error {
	return errNotYetImplmented
}

// Connected returns whether the device is currently connected.
//
// Not yet implemented on the s113v7.
func (d Device) Connected() (bool, error) {
	return false, errNotYetImplmented
}

type DeviceService struct{}
