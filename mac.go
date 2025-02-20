package bluetooth

import "errors"

// MAC represents a MAC address, in little endian format.
type MAC [6]byte

var errInvalidMAC = errors.New("bluetooth: failed to parse MAC address")

// ParseMAC parses the given MAC address, which must be in 11:22:33:AA:BB:CC
// format. If it cannot be parsed, an error is returned.
func ParseMAC(s string) (mac MAC, err error) {
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
			err = errInvalidMAC
			return
		}
		if macIndex < 0 {
			err = errInvalidMAC
			return
		}
		if macIndex%2 == 0 {
			mac[macIndex/2] |= nibble
		} else {
			mac[macIndex/2] |= nibble << 4
		}
		macIndex--
	}
	if macIndex != -1 {
		err = errInvalidMAC
	}
	return
}

// String returns a human-readable version of this MAC address, such as
// 11:22:33:AA:BB:CC.
func (mac MAC) String() string {
	buf, _ := mac.MarshalText()
	return string(buf)
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

func (mac MAC) MarshalText() (text []byte, err error) {
	return mac.AppendText(make([]byte, 0, 17))
}

func (mac MAC) MarshalBinary() (data []byte, err error) {
	return mac.AppendBinary(make([]byte, 0, 6))
}

// AppendBinary appends the binary representation of itself to the end of b
// (allocating a larger slice if necessary) and returns the updated slice.
func (mac MAC) AppendBinary(b []byte) ([]byte, error) {
	return append(b, mac[0], mac[1], mac[2], mac[3], mac[4], mac[5]), nil
}
