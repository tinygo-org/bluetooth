package hci

// Transport carries HCI packets to and from the controller. A board provides
// an implementation for its own hardware.
//
// StartRead and EndRead bracket a read burst, so that a transport with flow
// control can assert it for the whole burst. A packet oriented transport hands
// back a whole packet at a time and fails a read outright when the packet does
// not fit, so Read is always given room for the rounded up size that Buffered
// reports.
type Transport interface {
	StartRead()
	EndRead()
	Buffered() int
	Read(buf []byte) (int, error)
	Write(buf []byte) (int, error)
}

// MaxPacketSize is the largest packet that can legitimately be received. An
// HCI event carries a single byte parameter length, so the largest one is the
// packet type byte, the event code, the length byte and 255 parameter bytes.
// This also covers the largest ACL packet that can arrive given maximumMTU.
const MaxPacketSize = hciEvtLenPos + 255 + 1

// TransportOverhead is the extra room a transport may need in the
// destination buffer on top of the bytes it hands back. The CYW43439 copies a
// whole entry out of its ring buffer before stripping the 3 byte SDIO header,
// and rounds the copy up to a 4 byte boundary.
const TransportOverhead = 3

// ReadBufferSize is the size of the packet read buffer. It must be large enough
// for the largest packet that can legitimately be received plus any transport
// overhead, otherwise such a packet can never be assembled.
const ReadBufferSize = (MaxPacketSize + TransportOverhead + 3) &^ 3

// alignUp4 rounds n up to a multiple of 4. Packet oriented transports read in
// 4 byte units, so the destination has to have room for the rounded up size.
func alignUp4(n int) int {
	return (n + 3) &^ 3
}
