package bluetooth

import (
	"testing"
	"time"
)

func TestCreateAdvertisementPayload(t *testing.T) {
	type testCase struct {
		raw    string
		parsed AdvertisementOptions
	}
	tests := []testCase{
		{
			raw:    "\x02\x01\x06", // flags
			parsed: AdvertisementOptions{},
		},
		{
			raw: "\x02\x01\x06", // flags
			parsed: AdvertisementOptions{
				// Interval doesn't affect the advertisement payload.
				Interval: NewDuration(100 * time.Millisecond),
			},
		},
		{
			raw: "\x02\x01\x06" + // flags
				"\x07\x09foobar", // local name
			parsed: AdvertisementOptions{
				LocalName: "foobar",
			},
		},
		{
			raw: "\x02\x01\x06" + // flags
				"\x0b\x09Heart rate" + // local name
				"\x03\x03\x0d\x18", // service UUID
			parsed: AdvertisementOptions{
				LocalName: "Heart rate",
				ServiceUUIDs: []UUID{
					ServiceUUIDHeartRate,
				},
			},
		},
		{
			// Note: the two service UUIDs should really be merged into one to
			// save space.
			raw: "\x02\x01\x06" + // flags
				"\x0b\x09Heart rate" + // local name
				"\x03\x03\x0d\x18" + // heart rate service UUID
				"\x03\x03\x0f\x18", // battery service UUID
			parsed: AdvertisementOptions{
				LocalName: "Heart rate",
				ServiceUUIDs: []UUID{
					ServiceUUIDHeartRate,
					ServiceUUIDBattery,
				},
			},
		},
	}
	for _, tc := range tests {
		var expectedRaw rawAdvertisementPayload
		expectedRaw.len = uint8(len(tc.raw))
		copy(expectedRaw.data[:], tc.raw)

		var raw rawAdvertisementPayload
		raw.addFromOptions(tc.parsed)
		if raw != expectedRaw {
			t.Errorf("error when serializing options: %#v\nexpected: %#v\nactual:   %#v\n", tc.parsed, tc.raw, string(raw.data[:raw.len]))
		}
	}
}
