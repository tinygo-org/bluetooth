package bluetooth

import (
	"errors"
	"unsafe"
)

// MAC represents a MAC address, in little endian format.
type MAC [6]byte

// UnmarshalText unmarshals the text into itself.
// The given MAC address byte array must be of the format 11:22:33:AA:BB:CC.
// If it cannot be unmarshaled, an error is returned.
func (mac *MAC) UnmarshalText(s []byte) error {
	macIndex := 11
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == ':' {
			continue
		}
		var nibble byte
		if c >= '0' && c <= '9' {
			nibble = c - '0' + 0x0
		} else if c >= 'A' && c <= 'F' {
			nibble = c - 'A' + 0xA
		} else {
			return ErrInvalidMAC
		}
		if macIndex < 0 {
			return ErrInvalidMAC
		}
		if macIndex%2 == 0 {
			mac[macIndex/2] |= nibble
		} else {
			mac[macIndex/2] |= nibble << 4
		}
		macIndex--
	}
	if macIndex != -1 {
		return ErrInvalidMAC
	}
	return nil
}

// ParseMAC parses the given MAC address, which must be in 11:22:33:AA:BB:CC
// format. If it cannot be parsed, an error is returned.
func ParseMAC(s string) (mac MAC, err error) {
	err = (&mac).UnmarshalText([]byte(s))
	return
}

// String returns a human-readable version of this MAC address, such as
// 11:22:33:AA:BB:CC.
func (mac MAC) String() string {
	buf, _ := mac.MarshalText()
	return unsafe.String(unsafe.SliceData(buf), 17)
}

const hexDigit = "0123456789ABCDEF"

// AppendText appends the textual representation of itself to the end of b
// (allocating a larger slice if necessary) and returns the updated slice.
func (mac MAC) AppendText(buf []byte) ([]byte, error) {
	for i := 5; i >= 0; i-- {
		if i != 5 {
			buf = append(buf, ':')
		}
		buf = append(buf, hexDigit[mac[i]>>4])
		buf = append(buf, hexDigit[mac[i]&0xF])
	}
	return buf, nil
}

// MarshalText marshals itself into a string of format 11:22:33:AA:BB:CC.
// It is a simple wrapper of the AppentText method.
func (mac MAC) MarshalText() (text []byte, err error) {
	return mac.AppendText(make([]byte, 0, 17))
}

var ErrInvalidMAC = errors.New("bluetooth: failed to parse MAC address")

// MarshalBinary marshals itself into a binary format.
// This is a simple wrapper of the AppendBinary method
func (mac MAC) MarshalBinary() (data []byte, err error) {
	return mac.AppendBinary(make([]byte, 0, 6))
}

var ErrInvalidBinaryMac = errors.New("bluetooth: failed to unmarshal the binary MAC address")

// UnmarshalBinary unmarshals the mac byte slice into itself.
// It will return the ErrInvalidBinaryMac error if the given slice is not exactually 6 in length.
func (mac *MAC) UnmarshalBinary(data []byte) error {
	if len(data) != 6 {
		return ErrInvalidBinaryMac
	}
	copy(mac[:], data)
	return nil
}

// AppendBinary appends the binary representation of itself to the end of b
// (allocating a larger slice if necessary) and returns the updated slice.
func (mac MAC) AppendBinary(b []byte) ([]byte, error) {
	return append(b, mac[0], mac[1], mac[2], mac[3], mac[4], mac[5]), nil
}
