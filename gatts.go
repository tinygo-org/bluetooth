package bluetooth

// Service is a GATT service to be used in AddService.
type Service struct {
	handle uint16
	UUID
	Characteristics []CharacteristicConfig
}

// Characteristic is a single characteristic in a service. It has an UUID and a
// value.
type Characteristic struct {
	handle uint16
}

// CharacteristicConfig contains some parameters for the configuration of a
// single characteristic.
//
// The Handle field may be nil. If it is set, it points to a characteristic
// handle that can be used to access the characteristic at a later time.
type CharacteristicConfig struct {
	Handle *Characteristic
	UUID
	Value []byte
}

// Handle returns the numeric handle for this characteristic. This is used
// internally in the Bluetooth stack to identify this characteristic.
func (c *Characteristic) Handle() uint16 {
	return c.handle
}
