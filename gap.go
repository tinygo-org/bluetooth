package bluetooth

import "errors"

var (
	errScanning                  = errors.New("bluetooth: a scan is already in progress")
	errNotScanning               = errors.New("bluetooth: there is no scan in progress")
	errAdvertisementPacketTooBig = errors.New("bluetooth: advertisement packet overflows")
)

// AdvertisementOptions configures an advertisement instance. More options may
// be added over time.
type AdvertisementOptions struct {
	// The (complete) local name that will be advertised. Optional, omitted if
	// this is a zero-length string.
	LocalName string

	// Interval in BLE-specific units. Create an interval by using
	// NewAdvertiseInterval.
	Interval AdvertisementInterval
}

// AdvertisementInterval is the advertisement interval in 0.625µs units.
type AdvertisementInterval uint32

// NewAdvertisementInterval returns a new advertisement interval, based on an
// interval in milliseconds.
func NewAdvertisementInterval(intervalMillis uint32) AdvertisementInterval {
	// Convert an interval to units of
	return AdvertisementInterval(intervalMillis * 8 / 5)
}

// Connection is a numeric identifier that indicates a connection handle.
type Connection uint16

// GAPEvent is a base (embeddable) event for all GAP events.
type GAPEvent struct {
	Connection Connection
}

// ConnectEvent occurs when a remote device connects to this device.
type ConnectEvent struct {
	GAPEvent
}

// DisconnectEvent occurs when a remote device disconnects from this device.
type DisconnectEvent struct {
	GAPEvent
}

// ScanResult contains information from when an advertisement packet was
// received. It is passed as a parameter to the callback of the Scan method.
type ScanResult struct {
	// MAC address of the scanned device.
	Address MAC

	// RSSI the last time a packet from this device has been received.
	RSSI int16

	// The data obtained from the advertisement data, which may contain many
	// different properties.
	// Warning: this data may only stay valid until the next event arrives. If
	// you need any of the fields to stay alive until after the callback
	// returns, copy them.
	AdvertisementPayload
}

// AdvertisementPayload contains information obtained during a scan (see
// ScanResult). It is provided as an interface as there are two possible
// implementations: an implementation that works with raw data (usually on
// low-level BLE stacks) and an implementation that works with structured data.
type AdvertisementPayload interface {
	// LocalName is the (complete or shortened) local name of the device.
	// Please note that many devices do not broadcast a local name, but may
	// broadcast other data (e.g. manufacturer data or service UUIDs) with which
	// they may be identified.
	LocalName() string

	// Bytes returns the raw advertisement packet, if available. It returns nil
	// if this data is not available.
	Bytes() []byte
}

// AdvertisementFields contains advertisement fields in structured form.
type AdvertisementFields struct {
	// The LocalName part of the advertisement (either the complete local name
	// or the shortened local name).
	LocalName string
}

// advertisementFields wraps AdvertisementFields to implement the
// AdvertisementPayload interface. The methods to implement the interface (such
// as LocalName) cannot be implemented on AdvertisementFields because they would
// conflict with field names.
type advertisementFields struct {
	AdvertisementFields
}

// LocalName returns the underlying LocalName field.
func (p *advertisementFields) LocalName() string {
	return p.AdvertisementFields.LocalName
}

// Bytes returns nil, as structured advertisement data does not have the
// original raw advertisement data available.
func (p *advertisementFields) Bytes() []byte {
	return nil
}

// rawAdvertisementPayload encapsulates a raw advertisement packet. Methods to
// get the data (such as LocalName()) will parse just the needed field. Scanning
// the data should be fast as most advertisement packets only have a very small
// (3 or so) amount of fields.
type rawAdvertisementPayload struct {
	data [31]byte
	len  uint8
}

// Bytes returns the raw advertisement packet as a byte slice.
func (buf *rawAdvertisementPayload) Bytes() []byte {
	return buf.data[:buf.len]
}

// findField returns the data of a specific field in the advertisement packet.
func (buf *rawAdvertisementPayload) findField(fieldType byte) []byte {
	data := buf.Bytes()
	for len(data) >= 2 {
		fieldLength := data[0]
		if int(fieldLength)+1 > len(data) {
			// Invalid field length.
			return nil
		}
		if fieldType == data[1] {
			return data[2 : fieldLength+1]
		}
		data = data[fieldLength+1:]
	}
	return nil
}

// LocalName returns the local name (complete or shortened) in the advertisement
// payload.
func (buf *rawAdvertisementPayload) LocalName() string {
	b := buf.findField(9) // Complete Local Name
	if len(b) != 0 {
		println("complete")
		return string(b)
	}
	b = buf.findField(8) // Shortened Local Name
	if len(b) != 0 {
		println("shortened")
		return string(b)
	}
	return ""
}

// addFlags adds a flags field to the advertisement buffer. It returns true on
// success (the flags can be added) and false on failure.
func (buf *rawAdvertisementPayload) addFlags(flags byte) (ok bool) {
	if int(buf.len)+3 > len(buf.data) {
		return false // flags don't fit
	}

	buf.data[buf.len] = 2       // length of field (including type)
	buf.data[buf.len+1] = 0x01  // type, 0x01 means Flags
	buf.data[buf.len+2] = flags // the flags
	buf.len += 3
	return true
}

// addFlags adds the Complete Local Name field to the advertisement buffer. It
// returns true on success (the name fits) and false on failure.
func (buf *rawAdvertisementPayload) addCompleteLocalName(name string) (ok bool) {
	if int(buf.len)+len(name)+2 > len(buf.data) {
		return false // name doesn't fit
	}

	buf.data[buf.len] = byte(len(name) + 1) // length of field (including type)
	buf.data[buf.len+1] = 9                 // type, 0x09 means Complete Local name
	copy(buf.data[buf.len+2:], name)        // copy the name into the buffer
	buf.len += byte(len(name) + 2)
	return true
}
