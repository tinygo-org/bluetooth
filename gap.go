package bluetooth

import (
	"errors"
	"time"
)

var (
	errScanning                  = errors.New("bluetooth: a scan is already in progress")
	errNotScanning               = errors.New("bluetooth: there is no scan in progress")
	errAdvertisementPacketTooBig = errors.New("bluetooth: advertisement packet overflows")
)

// MACAddress contains a Bluetooth address which is a MAC address.
type MACAddress struct {
	// MAC address of the Bluetooth device.
	MAC

	isRandom bool
}

// IsRandom if the address is randomly created.
func (mac MACAddress) IsRandom() bool {
	return mac.isRandom
}

// SetRandom if is a random address.
func (mac *MACAddress) SetRandom(val bool) {
	mac.isRandom = val
}

// Set the address
func (mac *MACAddress) Set(val string) {
	m, err := ParseMAC(val)
	if err != nil {
		return
	}

	mac.MAC = m
}

// AdvertisementOptions configures an advertisement instance. More options may
// be added over time.
type AdvertisementOptions struct {
	// The (complete) local name that will be advertised. Optional, omitted if
	// this is a zero-length string.
	LocalName string

	// ServiceUUIDs are the services (16-bit or 128-bit) that are broadcast as
	// part of the advertisement packet, in data types such as "complete list of
	// 128-bit UUIDs".
	ServiceUUIDs []UUID

	// Interval in BLE-specific units. Create an interval by using NewDuration.
	Interval Duration
}

// Duration is the unit of time used in BLE, in 0.625µs units. This unit of time
// is used throughout the BLE stack.
type Duration uint16

// NewDuration returns a new Duration, in units of 0.625µs. It is used both for
// advertisement intervals and for connection parameters.
func NewDuration(interval time.Duration) Duration {
	// Convert an interval to units of 0.625µs.
	return Duration(uint64(interval / (625 * time.Microsecond)))
}

// Connection is a numeric identifier that indicates a connection handle.
type Connection uint16

// ScanResult contains information from when an advertisement packet was
// received. It is passed as a parameter to the callback of the Scan method.
type ScanResult struct {
	// Bluetooth address of the scanned device.
	Address Address

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

	// HasServiceUUID returns true whether the given UUID is present in the
	// advertisement payload as a Service Class UUID. It checks both 16-bit
	// UUIDs and 128-bit UUIDs.
	HasServiceUUID(UUID) bool

	// Bytes returns the raw advertisement packet, if available. It returns nil
	// if this data is not available.
	Bytes() []byte

	// ManufacturerData returns a map with all the manufacturer data present in the
	//advertising. IT may be empty.
	ManufacturerData() map[uint16][]byte
}

// AdvertisementFields contains advertisement fields in structured form.
type AdvertisementFields struct {
	// The LocalName part of the advertisement (either the complete local name
	// or the shortened local name).
	LocalName string

	// ServiceUUIDs are the services (16-bit or 128-bit) that are broadcast as
	// part of the advertisement packet, in data types such as "complete list of
	// 128-bit UUIDs".
	ServiceUUIDs []UUID

	// ManufacturerData is the manufacturer data of the advertisement.
	ManufacturerData map[uint16][]byte
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

// HasServiceUUID returns true whether the given UUID is present in the
// advertisement payload as a Service Class UUID.
func (p *advertisementFields) HasServiceUUID(uuid UUID) bool {
	for _, u := range p.AdvertisementFields.ServiceUUIDs {
		if u == uuid {
			return true
		}
	}
	return false
}

// Bytes returns nil, as structured advertisement data does not have the
// original raw advertisement data available.
func (p *advertisementFields) Bytes() []byte {
	return nil
}

// ManufacturerData returns the underlying ManufacturerData field.
func (p *advertisementFields) ManufacturerData() map[uint16][]byte {
	return p.AdvertisementFields.ManufacturerData
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
//
// See this list of field types:
// https://www.bluetooth.com/specifications/assigned-numbers/generic-access-profile/
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
		return string(b)
	}
	b = buf.findField(8) // Shortened Local Name
	if len(b) != 0 {
		return string(b)
	}
	return ""
}

// HasServiceUUID returns true whether the given UUID is present in the
// advertisement payload as a Service Class UUID. It checks both 16-bit UUIDs
// and 128-bit UUIDs.
func (buf *rawAdvertisementPayload) HasServiceUUID(uuid UUID) bool {
	if uuid.Is16Bit() {
		b := buf.findField(0x03) // Complete List of 16-bit Service Class UUIDs
		if len(b) == 0 {
			b = buf.findField(0x02) // Incomplete List of 16-bit Service Class UUIDs
		}
		uuid := uuid.Get16Bit()
		for i := 0; i < len(b)/2; i++ {
			foundUUID := uint16(b[i*2]) | (uint16(b[i*2+1]) << 8)
			if uuid == foundUUID {
				return true
			}
		}
		return false
	} else {
		b := buf.findField(0x07) // Complete List of 128-bit Service Class UUIDs
		if len(b) == 0 {
			b = buf.findField(0x06) // Incomplete List of 128-bit Service Class UUIDs
		}
		uuidBuf1 := uuid.Bytes()
		for i := 0; i < len(b)/16; i++ {
			uuidBuf2 := b[i*16 : i*16+16]
			match := true
			for i, c := range uuidBuf1 {
				if c != uuidBuf2[i] {
					match = false
					break
				}
			}
			if match {
				return true
			}
		}
		return false
	}
}

// ManufacturerData returns the manufacturer data in the advertisement payload.
func (buf *rawAdvertisementPayload) ManufacturerData() map[uint16][]byte {
	mData := make(map[uint16][]byte)
	data := buf.Bytes()
	for len(data) >= 2 {
		fieldLength := data[0]
		if int(fieldLength)+1 > len(data) {
			// Invalid field length.
			return nil
		}
		// If this is the manufacturer data
		if byte(0xFF) == data[1] {
			mData[uint16(data[2])+(uint16(data[3])<<8)] = data[4 : fieldLength+1]
		}
		data = data[fieldLength+1:]
	}
	return mData
}

// reset restores this buffer to the original state.
func (buf *rawAdvertisementPayload) reset() {
	// The data is not reset (only the length), because with a zero length the
	// data is undefined.
	buf.len = 0
}

// addFromOptions constructs a new advertisement payload (assumed to be empty
// before the call) from the advertisement options. It returns true if it fits,
// false otherwise.
func (buf *rawAdvertisementPayload) addFromOptions(options AdvertisementOptions) (ok bool) {
	buf.addFlags(0x06)
	if options.LocalName != "" {
		if !buf.addCompleteLocalName(options.LocalName) {
			return false
		}
	}
	// TODO: if there are multiple 16-bit UUIDs, they should be listed in
	// one field.
	// This is not possible for 128-bit service UUIDs (at least not in
	// legacy advertising) because of the 31-byte advertisement packet
	// limit.
	for _, uuid := range options.ServiceUUIDs {
		if !buf.addServiceUUID(uuid) {
			return false
		}
	}
	return true
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

// addCompleteLocalName adds the Complete Local Name field to the advertisement
// buffer. It returns true on success (the name fits) and false on failure.
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

// addServiceUUID adds a Service Class UUID (16-bit or 128-bit). It has
// currently only been designed for adding single UUIDs: multiple UUIDs are
// stored in separate fields without joining them together in one field.
func (buf *rawAdvertisementPayload) addServiceUUID(uuid UUID) (ok bool) {
	// Don't bother with 32-bit UUID support, it doesn't seem to be used in
	// practice.
	if uuid.Is16Bit() {
		if int(buf.len)+4 > len(buf.data) {
			return false // UUID doesn't fit.
		}
		shortUUID := uuid.Get16Bit()
		buf.data[buf.len+0] = 3    // length of field, including type
		buf.data[buf.len+1] = 0x03 // type, 0x03 means "Complete List of 16-bit Service Class UUIDs"
		buf.data[buf.len+2] = byte(shortUUID)
		buf.data[buf.len+3] = byte(shortUUID >> 8)
		buf.len += 4
		return true
	} else {
		if int(buf.len)+18 > len(buf.data) {
			return false // UUID doesn't fit.
		}
		buf.data[buf.len+0] = 17   // length of field, including type
		buf.data[buf.len+1] = 0x07 // type, 0x07 means "Complete List of 128-bit Service Class UUIDs"
		rawUUID := uuid.Bytes()
		copy(buf.data[buf.len+2:], rawUUID[:])
		buf.len += 18
		return true
	}
}

// ConnectionParams are used when connecting to a peripherals.
type ConnectionParams struct {
	// The timeout for the connection attempt. Not used during the rest of the
	// connection. If no duration is specified, a default timeout will be used.
	ConnectionTimeout Duration

	// Minimum and maximum connection interval. The shorter the interval, the
	// faster data can travel between both devices but also the more power they
	// will draw. If no intervals are specified, a default connection interval
	// will be used.
	MinInterval Duration
	MaxInterval Duration
}
