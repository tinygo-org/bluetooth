package bluetooth

import (
	"reflect"
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
		{
			raw: "\x02\x01\x06" + // flags
				"\a\xff\x34\x12asdf", // manufacturer data
			parsed: AdvertisementOptions{
				ManufacturerData: []ManufacturerDataElement{
					{0x1234, []byte("asdf")},
				},
			},
		},
		{
			raw: "\x02\x01\x06" + // flags
				"\x04\xff\x34\x12\x05" + // manufacturer data 1
				"\x05\xff\xff\xff\x03\x07" + // manufacturer data 2
				"\x03\xff\x11\x00", // manufacturer data 3
			parsed: AdvertisementOptions{
				ManufacturerData: []ManufacturerDataElement{
					{0x1234, []byte{5}},
					{0xffff, []byte{3, 7}},
					{0x0011, []byte{}},
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
		mdata := raw.ManufacturerData()
		if !reflect.DeepEqual(mdata, tc.parsed.ManufacturerData) {
			t.Errorf("ManufacturerData was not parsed as expected:\nexpected: %#v\nactual:   %#v", tc.parsed.ManufacturerData, mdata)
		}
	}
}
