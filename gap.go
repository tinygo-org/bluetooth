package bluetooth

// Advertisement encapsulates a single advertisement instance.
type Advertisement struct {
	handle uint8
}

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
