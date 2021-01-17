package bluetooth

// New16BitUUID returns a new 128-bit UUID based on a 16-bit UUID.
//
// Note: only use registered UUIDs. See
// https://www.bluetooth.com/specifications/gatt/services/ for a list.
func New16BitUUID(shortUUID uint16) UUID {
	// mac OS uses a unique format for UUID.
	var uuid UUID
	uuid[0] = 0x00000000
	uuid[1] = 0x00000000
	uuid[2] = 0x00000000
	uuid[3] = uint32(shortUUID) << 16
	return uuid
}
