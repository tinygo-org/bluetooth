// +build !linux

package bluetooth

// Characteristic is a single characteristic in a service. It has an UUID and a
// value.
type Characteristic struct {
	permissions CharacteristicPermissions
}
