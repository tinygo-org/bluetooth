// +build wioterminal

package bluetooth

// Characteristic is a single characteristic in a service. It has an UUID and a
// value.
type Characteristic struct {
	handle      uint16
	permissions CharacteristicPermissions
}

// charWriteHandler contains a handler->callback mapping for characteristic
// writes.
type charWriteHandler struct {
	handle   uint16
	callback func(connection Connection, offset int, value []byte)
}
