//go:build hci || ninafw || cyw43439 || espradio

package bluetooth

import "tinygo.org/x/bluetooth/hci"

// The protocol errors are defined in the hci subpackage. They are aliased here
// so that a comparison against the old name still holds.
var (
	ErrHCITimeout       = hci.ErrTimeout
	ErrHCIUnknownEvent  = hci.ErrUnknownEvent
	ErrHCIUnknown       = hci.ErrUnknown
	ErrHCIInvalidPacket = hci.ErrInvalidPacket
	ErrHCIHardware      = hci.ErrHardware

	ErrATTTimeout           = hci.ErrATTTimeout
	ErrATTUnknownEvent      = hci.ErrATTUnknownEvent
	ErrATTUnknown           = hci.ErrATTUnknown
	ErrATTOp                = hci.ErrATTOp
	ErrATTUnknownConnection = hci.ErrATTUnknownConnection
	ErrATTAttributeNotFound = hci.ErrATTAttributeNotFound
)
