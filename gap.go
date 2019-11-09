package bluetooth

// AdvertiseOptions configures everything related to BLE advertisements.
type AdvertiseOptions struct {
	Interval AdvertiseInterval
}

// AdvertiseInterval is the advertisement interval in 0.625µs units.
type AdvertiseInterval uint32

// NewAdvertiseInterval returns a new advertisement interval, based on an
// interval in milliseconds.
func NewAdvertiseInterval(intervalMillis uint32) AdvertiseInterval {
	// Convert an interval to units of
	return AdvertiseInterval(intervalMillis * 8 / 5)
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
