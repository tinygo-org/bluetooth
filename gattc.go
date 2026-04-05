//go:build !baremetal || !(s110v8 || s113v7)

package bluetooth

// GATTCService is the common interface that all platform-specific
// DeviceService types must implement.
type GATTCService interface {
	// UUID returns the UUID for this DeviceService.
	UUID() UUID

	// DiscoverCharacteristics discovers characteristics in this service.
	// Pass a list of characteristic UUIDs you are interested in to this
	// function. Either a list of all requested services is returned, or if
	// some services could not be discovered an error is returned.
	DiscoverCharacteristics(uuids []UUID) ([]DeviceCharacteristic, error)
}

// GATTCCharacteristic is the common interface that all platform-specific
// DeviceCharacteristic types must implement.
type GATTCCharacteristic interface {
	// UUID returns the UUID for this DeviceCharacteristic.
	UUID() UUID

	// Read reads the current characteristic value.
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
