package bluetooth

// This file implements 16-bit and 128-bit UUIDs as defined in the Bluetooth
// specification.

import (
	"errors"
	"unsafe"
)

// UUID is a single UUID as used in the Bluetooth stack. It is represented as a
// [4]uint32 instead of a [16]byte for efficiency.
type UUID struct {
	id [4]uint32
}

var errInvalidUUID = errors.New("bluetooth: failed to parse UUID")

// NewUUID returns a new UUID based on the 128-bit (or 16-byte) input.
func NewUUID(uuid [16]byte) UUID {
	uu := UUID{}
	uu.id[0] = uint32(uuid[15]) | uint32(uuid[14])<<8 | uint32(uuid[13])<<16 | uint32(uuid[12])<<24
	uu.id[1] = uint32(uuid[11]) | uint32(uuid[10])<<8 | uint32(uuid[9])<<16 | uint32(uuid[8])<<24
	uu.id[2] = uint32(uuid[7]) | uint32(uuid[6])<<8 | uint32(uuid[5])<<16 | uint32(uuid[4])<<24
	uu.id[3] = uint32(uuid[3]) | uint32(uuid[2])<<8 | uint32(uuid[1])<<16 | uint32(uuid[0])<<24
	return uu
}

// New16BitUUID returns a new 128-bit UUID based on a 16-bit UUID.
//
// Note: only use registered UUIDs. See
// https://www.bluetooth.com/specifications/gatt/services/ for a list.
func New16BitUUID(shortUUID uint16) UUID {
	// https://stackoverflow.com/questions/36212020/how-can-i-convert-a-bluetooth-16-bit-service-uu-into-a-128-bit-uu
	var uu UUID
	uu.id[0] = 0x5F9B34FB
	uu.id[1] = 0x80000080
	uu.id[2] = 0x00001000
	uu.id[3] = uint32(shortUUID)
	return uu
}

// New32BitUUID returns a new 128-bit UUID based on a 32-bit UUID.
//
// Note: only use registered UUIDs. See
// https://www.bluetooth.com/specifications/gatt/services/ for a list.
func New32BitUUID(shortUUID uint32) UUID {
	// https://stackoverflow.com/questions/36212020/how-can-i-convert-a-bluetooth-16-bit-service-uuid-into-a-128-bit-uuid
	var uu UUID
	uu.id[0] = 0x5F9B34FB
	uu.id[1] = 0x80000080
	uu.id[2] = 0x00001000
	uu.id[3] = shortUUID
	return uu
}

// Replace16BitComponent returns a new UUID where bits 16..32 have been replaced
// with the bits given in the argument. These bits are the same bits that vary
// in the 16-bit compressed UUID form.
//
// This is especially useful for the Nordic SoftDevice, because it is able to
// store custom UUIDs more efficiently when only these bits vary between them.
func (uuid UUID) Replace16BitComponent(component uint16) UUID {
	uuid.id[3] &^= 0x0000ffff       // clear the new component bits
	uuid.id[3] |= uint32(component) // set the component bits
	return uuid
}

// Is16Bit returns whether this UUID is a 16-bit BLE UUID.
func (uuid UUID) Is16Bit() bool {
	return uuid.Is32Bit() && uuid.id[3] == uint32(uint16(uuid.id[3]))
}

// Is32Bit returns whether this UUID is a 32-bit or 16-bit BLE UUID.
func (uuid UUID) Is32Bit() bool {
	return uuid.id[0] == 0x5F9B34FB && uuid.id[1] == 0x80000080 && uuid.id[2] == 0x00001000
}

// Get16Bit returns the 16-bit version of this UUID. This is only valid if it
// actually is a 16-bit UUID, see Is16Bit.
func (uuid UUID) Get16Bit() uint16 {
	// Note: using a Get* function as a getter because method names can't start
	// with a number.
	return uint16(uuid.id[3])
}

// Get32Bit returns the 32-bit version of this UUID. This is only valid if it
// actually is a 32-bit UUID, see Is32Bit.
func (uuid UUID) Get32Bit() uint32 {
	// Note: using a Get* function as a getter because method names can't start
	// with a number.
	return uuid.id[3]
}

// BytesBytesBigEndian returns a 16-byte array containing the raw UUID in
// network order. The result from BytesBigEndian may be used as the input for
// NewUUID.
func (uuid UUID) BytesBigEndian() [16]byte {
	return [16]byte{
		0:  byte(uuid.id[3] >> 24),
		1:  byte(uuid.id[3] >> 16),
		2:  byte(uuid.id[3] >> 8),
		3:  byte(uuid.id[3]),
		4:  byte(uuid.id[2] >> 24),
		5:  byte(uuid.id[2] >> 16),
		6:  byte(uuid.id[2] >> 8),
		7:  byte(uuid.id[2]),
		8:  byte(uuid.id[1] >> 24),
		9:  byte(uuid.id[1] >> 16),
		10: byte(uuid.id[1] >> 8),
		11: byte(uuid.id[1]),
		12: byte(uuid.id[0] >> 24),
		13: byte(uuid.id[0] >> 16),
		14: byte(uuid.id[0] >> 8),
		15: byte(uuid.id[0]),
	}
}

// bytes returns a 16-byte array containing the raw UUID in little-endian
// order.
func (uuid UUID) bytes() [16]byte {
	return [16]byte{
		0:  byte(uuid.id[0]),
		1:  byte(uuid.id[0] >> 8),
		2:  byte(uuid.id[0] >> 16),
		3:  byte(uuid.id[0] >> 24),
		4:  byte(uuid.id[1]),
		5:  byte(uuid.id[1] >> 8),
		6:  byte(uuid.id[1] >> 16),
		7:  byte(uuid.id[1] >> 24),
		8:  byte(uuid.id[2]),
		9:  byte(uuid.id[2] >> 8),
		10: byte(uuid.id[2] >> 16),
		11: byte(uuid.id[2] >> 24),
		12: byte(uuid.id[3]),
		13: byte(uuid.id[3] >> 8),
		14: byte(uuid.id[3] >> 16),
		15: byte(uuid.id[3] >> 24),
	}
}

// AppendBinary appends the bytes of the uuid in little-endian order to the
// given byte slice b.
func (uuid UUID) AppendBinary(b []byte) ([]byte, error) {
	id := uuid.bytes()
	return append(b, id[:]...), nil
}

// MarshalBinary marshals the uuid into and byte slice in little-endian order
// and returns the slice. It will not return an error.
func (uuid UUID) MarshalBinary() (data []byte, err error) {
	return uuid.AppendBinary(make([]byte, 0, 16))
}

// ParseUUID parses the given UUID
//
// Expected formats:
//
//	00001234-0000-1000-8000-00805f9b34fb
//	00001234
//	1234
//
// If the UUID cannot be parsed, an error is returned.
// It will always successfully parse UUIDs generated by UUID.String().
func ParseUUID(s string) (uuid UUID, err error) {
	err = (&uuid).UnmarshalText([]byte(s))
	return
}

// UnmarshalText unmarshals a text representation of a UUID.
//
// Expected formats:
//
//	00001234-0000-1000-8000-00805f9b34fb
//	00001234
//	1234
//
// If the UUID cannot be parsed, an error is returned.
// It will always successfully parse UUIDs generated by UUID.String().
// This method is an adaptation of hex.Decode idea of using a reverse hex table.
func (u *UUID) UnmarshalText(s []byte) error {
	switch len(s) {
	case 36:
		return u.unmarshalText128(s)
	case 8:
		return u.unmarshalText32(s)
	case 4:
		return u.unmarshalText16(s)
	default:
		return errInvalidUUID
	}
}

// Using the reverseHexTable rebuild the UUID from the string s represented in bytes
// This implementation is the inverse of MarshalText and reaches performance parity
func (u *UUID) unmarshalText128(s []byte) error {
	var j uint8
	for i := 3; i >= 0; i-- {
		// Skip hyphens
		if s[j] == '-' {
			j++
		}

		if reverseHexTable[s[j]] == 255 {
			return errInvalidUUID
		}
		u.id[i] |= uint32(reverseHexTable[s[j]]) << 28
		j++

		if reverseHexTable[s[j]] == 255 {
			return errInvalidUUID
		}
		u.id[i] |= uint32(reverseHexTable[s[j]]) << 24
		j++

		if reverseHexTable[s[j]] == 255 {
			return errInvalidUUID
		}
		u.id[i] |= uint32(reverseHexTable[s[j]]) << 20
		j++

		if reverseHexTable[s[j]] == 255 {
			return errInvalidUUID
		}
		u.id[i] |= uint32(reverseHexTable[s[j]]) << 16
		j++

		// skip hypens
		if s[j] == '-' {
			j++
		}

		if reverseHexTable[s[j]] == 255 {
			return errInvalidUUID
		}
		u.id[i] |= uint32(reverseHexTable[s[j]]) << 12
		j++

		if reverseHexTable[s[j]] == 255 {
			return errInvalidUUID
		}
		u.id[i] |= uint32(reverseHexTable[s[j]]) << 8
		j++

		if reverseHexTable[s[j]] == 255 {
			return errInvalidUUID
		}
		u.id[i] |= uint32(reverseHexTable[s[j]]) << 4
		j++

		if reverseHexTable[s[j]] == 255 {
			return errInvalidUUID
		}
		u.id[i] |= uint32(reverseHexTable[s[j]])
		j++
	}

	return nil
}

// Using the reverseHexTable rebuild the UUID from the string s represented in bytes
// This implementation is the inverse of MarshalText and reaches performance pairity
func (u *UUID) unmarshalText32(s []byte) error {
	u.id[0] = 0x5F9B34FB
	u.id[1] = 0x80000080
	u.id[2] = 0x00001000

	var j uint8 = 0

	if reverseHexTable[s[j]] == 255 {
		return errInvalidUUID
	}
	u.id[3] |= uint32(reverseHexTable[s[j]]) << 28
	j++

	if reverseHexTable[s[j]] == 255 {
		return errInvalidUUID
	}
	u.id[3] |= uint32(reverseHexTable[s[j]]) << 24
	j++

	if reverseHexTable[s[j]] == 255 {
		return errInvalidUUID
	}
	u.id[3] |= uint32(reverseHexTable[s[j]]) << 20
	j++

	if reverseHexTable[s[j]] == 255 {
		return errInvalidUUID
	}
	u.id[3] |= uint32(reverseHexTable[s[j]]) << 16
	j++

	if reverseHexTable[s[j]] == 255 {
		return errInvalidUUID
	}
	u.id[3] |= uint32(reverseHexTable[s[j]]) << 12
	j++

	if reverseHexTable[s[j]] == 255 {
		return errInvalidUUID
	}
	u.id[3] |= uint32(reverseHexTable[s[j]]) << 8
	j++

	if reverseHexTable[s[j]] == 255 {
		return errInvalidUUID
	}
	u.id[3] |= uint32(reverseHexTable[s[j]]) << 4
	j++

	if reverseHexTable[s[j]] == 255 {
		return errInvalidUUID
	}
	u.id[3] |= uint32(reverseHexTable[s[j]])
	j++

	return nil
}

// Using the reverseHexTable rebuild the UUID from the string s represented in bytes
// This implementation is the inverse of MarshalText and reaches performance pairity
func (u *UUID) unmarshalText16(s []byte) error {
	u.id[0] = 0x5F9B34FB
	u.id[1] = 0x80000080
	u.id[2] = 0x00001000

	var j uint8 = 0
	if reverseHexTable[s[j]] == 255 {
		return errInvalidUUID
	}
	u.id[3] |= uint32(reverseHexTable[s[j]]) << 12
	j++

	if reverseHexTable[s[j]] == 255 {
		return errInvalidUUID
	}
	u.id[3] |= uint32(reverseHexTable[s[j]]) << 8
	j++

	if reverseHexTable[s[j]] == 255 {
		return errInvalidUUID
	}
	u.id[3] |= uint32(reverseHexTable[s[j]]) << 4
	j++

	if reverseHexTable[s[j]] == 255 {
		return errInvalidUUID
	}
	u.id[3] |= uint32(reverseHexTable[s[j]])
	j++

	return nil
}

// This table is structured such that an ascii byte can be directly used as the array key
// to directly look up the decoded uint8 value.  This is a similar solution to
// the reverseHexTable found in the hex package. 255 is a sentinel value
// indicating an invalid hex byte.
var reverseHexTable = [256]uint8{
	255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255,
	255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255,
	255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255,
	0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 255, 255, 255, 255, 255, 255,
	255, 10, 11, 12, 13, 14, 15, 255, 255, 255, 255, 255, 255, 255, 255, 255,
	255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255,
	255, 10, 11, 12, 13, 14, 15, 255, 255, 255, 255, 255, 255, 255, 255, 255,
	255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255,
}

// String returns a human-readable version of this UUID, such as
// 00001234-0000-1000-8000-00805f9b34fb.
func (u UUID) String() string {
	buf, _ := u.AppendText(make([]byte, 0, 36))

	// pulled from the guts of string builder
	return unsafe.String(unsafe.SliceData(buf), 36)
}

const hexDigitLower = "0123456789abcdef"

// AppendText converts and appends the uuid onto the given byte slice
// representing a human-readable version of this UUID, such as
// 00001234-0000-1000-8000-00805f9b34fb.
func (u UUID) AppendText(buf []byte) ([]byte, error) {
	for i := 3; i >= 0; i-- {
		// Insert a hyphen at the correct locations.
		// position 4 and 8
		if i != 3 && i != 0 {
			buf = append(buf, '-')
		}

		buf = append(buf, hexDigitLower[byte(u.id[i]>>24)>>4])
		buf = append(buf, hexDigitLower[byte(u.id[i]>>24)&0xF])

		buf = append(buf, hexDigitLower[byte(u.id[i]>>16)>>4])
		buf = append(buf, hexDigitLower[byte(u.id[i]>>16)&0xF])

		// Insert a hyphen at the correct locations.
		// position 6 and 10
		if i == 2 || i == 1 {
			buf = append(buf, '-')
		}

		buf = append(buf, hexDigitLower[byte(u.id[i]>>8)>>4])
		buf = append(buf, hexDigitLower[byte(u.id[i]>>8)&0xF])

		buf = append(buf, hexDigitLower[byte(u.id[i])>>4])
		buf = append(buf, hexDigitLower[byte(u.id[i])&0xF])
	}

	return buf, nil
}

// MarshalText returns the converted uuid as a bytle slice
// representing a human-readable version, such as
// 00001234-0000-1000-8000-00805f9b34fb.
func (u UUID) MarshalText() ([]byte, error) {
	return u.AppendText(make([]byte, 0, 36))
}

var ErrInvalidBinaryUUID = errors.New("bluetooth: failed to unmarshal the given binary UUID")

// UnmarshalBinary copies the given uuid bytes in little-endian order onto itself.
func (u *UUID) UnmarshalBinary(uuid []byte) error {
	if len(uuid) != 16 {
		return ErrInvalidBinaryUUID
	}

	u.id[0] = uint32(uuid[0]) | uint32(uuid[1])<<8 | uint32(uuid[2])<<16 | uint32(uuid[3])<<24
	u.id[1] = uint32(uuid[4]) | uint32(uuid[5])<<8 | uint32(uuid[6])<<16 | uint32(uuid[7])<<24
	u.id[2] = uint32(uuid[8]) | uint32(uuid[9])<<8 | uint32(uuid[10])<<16 | uint32(uuid[11])<<24
	u.id[3] = uint32(uuid[12]) | uint32(uuid[13])<<8 | uint32(uuid[14])<<16 | uint32(uuid[15])<<24
	return nil
}
