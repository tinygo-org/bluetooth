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
	handle      uint16
	permissions CharacteristicPermissions
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
	Flags CharacteristicPermissions
}

// Handle returns the numeric handle for this characteristic. This is used
// internally in the Bluetooth stack to identify this characteristic.
func (c *Characteristic) Handle() uint16 {
	return c.handle
}

// CharacteristicPermissions lists a number of basic permissions/capabilities
// that clients have regarding this characteristic. For example, if you want to
// allow clients to read the value of this characteristic (a common scenario),
// set the Read permission.
type CharacteristicPermissions uint8

// Characteristic permission bitfields.
const (
	CharacteristicBroadcastPermission CharacteristicPermissions = 1 << iota
	CharacteristicReadPermission
	CharacteristicWriteWithoutResponsePermission
	CharacteristicWritePermission
	CharacteristicNotifyPermission
	CharacteristicIndicatePermission
)

// Broadcast returns whether broadcasting of the value is permitted.
func (p CharacteristicPermissions) Broadcast() bool {
	return p&CharacteristicBroadcastPermission != 0
}

// Read returns whether reading of the value is permitted.
func (p CharacteristicPermissions) Read() bool {
	return p&CharacteristicReadPermission != 0
}

// Write returns whether writing of the value with Write Request is permitted.
func (p CharacteristicPermissions) Write() bool {
	return p&CharacteristicWritePermission != 0
}

// WriteWithoutResponse returns whether writing of the value with Write Command
// is permitted.
func (p CharacteristicPermissions) WriteWithoutResponse() bool {
	return p&CharacteristicWriteWithoutResponsePermission != 0
}
