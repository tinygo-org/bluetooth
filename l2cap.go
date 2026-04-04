package bluetooth

import "io"

// L2CAPPSM represents an L2CAP Protocol/Service Multiplexer identifier.
type L2CAPPSM = uint16

// L2CAPChannel is the common interface implemented by L2CAPConn on all platforms.
type L2CAPChannel interface {
	io.ReadWriteCloser
	// PSM returns the Protocol/Service Multiplexer identifier for this channel.
	PSM() L2CAPPSM
}
