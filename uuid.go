package bluetooth

// This file implements 16-bit and 128-bit UUIDs as defined in the Bluetooth
// specification.

// UUID is a single UUID as used in the Bluetooth stack. It is represented as a
// [4]uint32 instead of a [16]byte for efficiency.
type UUID [4]uint32

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

// Is16Bit returns whether this UUID is a 16-bit BLE UUID.
func (uuid UUID) Is16Bit() bool {
	return uuid.Is32Bit() && uuid[3] == uint32(uint16(uuid[3]))
}

// Is32Bit returns whether this UUID is a 32-bit BLE UUID.
func (uuid UUID) Is32Bit() bool {
	return uuid[0] == 0x5F9B34FB && uuid[1] == 0x80000080 && uuid[2] == 0x00001000
}
