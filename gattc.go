//go:build !baremetal || !(s110v8 || s113v7)

package bluetooth

import (
	"errors"
	"io"
)

var (
	// ErrServiceNotFound is returned by DiscoverServices when a requested service is not on the device.
	ErrServiceNotFound = errors.New("bluetooth: service not found")

	// ErrCharacteristicNotFound is returned by DiscoverCharacteristics when a requested characteristic is not in the service.
	ErrCharacteristicNotFound = errors.New("bluetooth: characteristic not found")
)

// GATTCService is the common interface that all platform-specific
// DeviceService types must implement.
type GATTCService interface {
	// UUID returns the UUID for this DeviceService.
	UUID() UUID

	// DiscoverCharacteristics discovers characteristics in this service.
	// Pass a list of characteristic UUIDs you are interested in to this
	// function. Either all requested characteristics are returned, or
	// ErrCharacteristicNotFound if one of them is not in the service.
	DiscoverCharacteristics(uuids []UUID) ([]DeviceCharacteristic, error)
}

// GATTCCharacteristic is the common interface that all platform-specific
// DeviceCharacteristic types must implement.
type GATTCCharacteristic interface {
	// UUID returns the UUID for this DeviceCharacteristic.
	UUID() UUID

	// Read reads the current characteristic value into data. It returns the number of
	// bytes copied, and io.ErrShortBuffer when the value is longer than data.
	Read(data []byte) (int, error)

	// Write replaces the characteristic value with a new value.
	Write(p []byte) (n int, err error)

	// WriteWithoutResponse replaces the characteristic value with a new
	// value. The call will return before all data has been written.
	WriteWithoutResponse(p []byte) (n int, err error)

	// EnableNotifications enables notifications for this characteristic,
	// calling the provided callback with the new value when the
	// characteristic value is changed by the remote peripheral.
	EnableNotifications(callback func(buf []byte)) error

	// GetMTU returns the MTU for this characteristic.
	GetMTU() (uint16, error)
}

// copyValue copies a characteristic value into data and reports truncation.
func copyValue(data, value []byte) (int, error) {
	n := copy(data, value)
	if n < len(value) {
		return n, io.ErrShortBuffer
	}
	return n, nil
}
