// +build !darwin

package bluetooth

// New16BitUUID returns a new 128-bit UUID based on a 16-bit UUID.
//
// Note: only use registered UUIDs. See
// https://www.bluetooth.com/specifications/gatt/services/ for a list.
func New16BitUUID(shortUUID uint16) UUID {
	// https://stackoverflow.com/questions/36212020/how-can-i-convert-a-bluetooth-16-bit-service-uuid-into-a-128-bit-uuid
	var uuid UUID
	uuid[0] = 0x5F9B34FB
	uuid[1] = 0x80000080
	uuid[2] = 0x00001000
	uuid[3] = uint32(shortUUID)
	return uuid
}
