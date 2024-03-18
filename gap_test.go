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
		{
			raw: "\x02\x01\x06" + // flags
				"\x05\x16\xD2\xFC\x40\x02" + // service data 16-Bit UUID
				"\x06\x20\xD2\xFC\x40\x02\xC4", // service data 32-Bit UUID
			parsed: AdvertisementOptions{
				ServiceData: []ServiceDataElement{
					{UUID: New16BitUUID(0xFCD2), Data: []byte{0x40, 0x02}},
					{UUID: New32BitUUID(0x0240FCD2), Data: []byte{0xC4}},
				},
			},
		},
		{
			raw: "\x02\x01\x06" + // flags
				"\x05\x16\xD2\xFC\x40\x02" + // service data 16-Bit UUID
				"\x05\x16\xD3\xFC\x40\x02", // service data 16-Bit UUID
			parsed: AdvertisementOptions{
				ServiceData: []ServiceDataElement{
					{UUID: New16BitUUID(0xFCD2), Data: []byte{0x40, 0x02}},
					{UUID: New16BitUUID(0xFCD3), Data: []byte{0x40, 0x02}},
				},
			},
		},
		{
			raw: "\x02\x01\x06" + // flags
				"\x04\x16\xD2\xFC\x40" + // service data 16-Bit UUID
				"\x12\x21\xB8\x6C\x75\x05\xE9\x25\xBD\x93\xA8\x42\x32\xC3\x00\x01\xAF\xAD\x09", // service data 128-Bit UUID
			parsed: AdvertisementOptions{
				ServiceData: []ServiceDataElement{
					{UUID: New16BitUUID(0xFCD2), Data: []byte{0x40}},
					{
						UUID: NewUUID([16]byte{0xad, 0xaf, 0x01, 0x00, 0xc3, 0x32, 0x42, 0xa8, 0x93, 0xbd, 0x25, 0xe9, 0x05, 0x75, 0x6c, 0xb8}),
						Data: []byte{0x09},
					},
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
