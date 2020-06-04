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
	if macIndex != 0 {
		err = errInvalidMAC
	}
	return
}

// String returns a human-readable version of this MAC address, such as
// 11:22:33:AA:BB:CC.
func (mac MAC) String() string {
	// TODO: make this more efficient.
	s := ""
	for i := 5; i >= 0; i-- {
		c := mac[i]
		// Insert a hyphen at the correct locations.
		if i != 5 {
			s += ":"
		}

		// First nibble.
		nibble := c >> 4
		if nibble <= 9 {
			s += string(nibble + '0')
		} else {
			s += string(nibble + 'A' - 10)
		}

		// Second nibble.
		nibble = c & 0x0f
		if nibble <= 9 {
			s += string(nibble + '0')
		} else {
			s += string(nibble + 'A' - 10)
		}
	}

	return s
}
