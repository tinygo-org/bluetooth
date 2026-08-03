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
				"\x03\x19\xc1\x03", // appearance (961, HID Keyboard, little-endian)
			parsed: AdvertisementOptions{
				Appearance: 961,
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

func TestServiceUUIDs(t *testing.T) {
	type testCase struct {
		raw      string
		expected []UUID
	}
	uuidBytes := ServiceUUIDAdafruitSound.bytes()
	tests := []testCase{
		{},
		{
			raw:      "\x03\x03\x0d\x18", // service UUID
			expected: []UUID{ServiceUUIDHeartRate},
		},
		{
			raw:      "\x03\x02\x0f\x18", // Service UUID
			expected: []UUID{ServiceUUIDBattery},
		},
		{
			raw:      "\x11\x07" + string(uuidBytes[:]),
			expected: []UUID{ServiceUUIDAdafruitSound},
		},
		{
			raw:      "\x11\x06" + string(uuidBytes[:]),
			expected: []UUID{ServiceUUIDAdafruitSound},
		},
		{
			raw: "\x11\x06" + string(uuidBytes[:15]), // data was cut off
		},
	}
	for _, tc := range tests {
		raw := rawAdvertisementPayload{len: uint8(len(tc.raw))}
		copy(raw.data[:], []byte(tc.raw))
		actual := raw.ServiceUUIDs()
		if !reflect.DeepEqual(actual, tc.expected) {
			t.Errorf("unexpected raw service UUIDs: %#v\nexpected: %#v\nactual:   %#v\n",
				tc.raw, tc.expected, actual)
		}
		for _, uuid := range actual {
			if !raw.HasServiceUUID(uuid) {
				t.Errorf("raw payload does not have UUID %#v\nhas: %#v", uuid, raw.ServiceUUIDs())
			}
		}
		fields := advertisementFields{AdvertisementFields: AdvertisementFields{ServiceUUIDs: tc.expected}}
		actual = fields.ServiceUUIDs()
		if !reflect.DeepEqual(actual, tc.expected) {
			t.Errorf("unexpected structured service UUIDs: %#v\nexpected: %#v\nactual:   %#v\n",
				tc.raw, tc.expected, actual)
		}
		for _, uuid := range actual {
			if !fields.HasServiceUUID(uuid) {
				t.Errorf("structured payload does not have UUID %#v\nhas: %#v", uuid, fields.ServiceUUIDs())
			}
		}
	}
}

// Advertisement payloads arrive straight off the air and may be truncated or
// otherwise malformed. Parsing one must never panic, no matter what it claims.
func TestMalformedAdvertisementPayload(t *testing.T) {
	tests := []struct {
		name string
		raw  string
	}{
		{
			name: "manufacturer data length past end of payload",
			raw:  "\x02\x01\x06" + "\x0a\xff\x4c\x00\x01",
		},
		{
			name: "manufacturer data field claiming the maximum length",
			raw:  "\x02\x01\x06" + "\xff\xff\x4c\x00",
		},
		{
			name: "16-bit service data length past end of payload",
			raw:  "\x02\x01\x06" + "\x0a\x16\xd2\xfc\x40",
		},
		{
			name: "32-bit service data truncated inside the UUID",
			raw:  "\x02\x01\x06" + "\x04\x20\xd2\xfc",
		},
		{
			name: "128-bit service data truncated inside the UUID",
			raw:  "\x05\x21\xb8\x6c\x75\x05",
		},
		{
			name: "zero length field followed by a local name type byte",
			raw:  "\x00\x09foobar",
		},
		{
			name: "trailing zero padding after a valid field",
			raw:  "\x07\x09foobar" + "\x00\x00\x00",
		},
		{
			name: "truncated local name",
			raw:  "\x02\x01\x06" + "\x0c\x09foo",
		},
		{
			name: "single dangling length byte",
			raw:  "\x02\x01\x06" + "\x05",
		},
	}

	check := func(t *testing.T, raw string) {
		t.Helper()
		var buf rawAdvertisementPayload
		buf.len = uint8(copy(buf.data[:], raw))

		// None of these may panic. The returned values are not checked: the
		// input is garbage, so any output is acceptable as long as parsing
		// stays inside the buffer.
		buf.LocalName()
		buf.ManufacturerData()
		buf.ServiceData()
		buf.ServiceUUIDs()
		buf.HasServiceUUID(ServiceUUIDHeartRate)
		buf.HasServiceUUID(ServiceUUIDAdafruitSound)
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			check(t, tc.raw)
		})
	}

	// Exhaustive sweep: place a field of every possible declared length at
	// every offset of a full-size payload, for each type this code parses.
	t.Run("every field length at every offset", func(t *testing.T) {
		for _, fieldType := range []byte{0x08, 0x09, 0x02, 0x03, 0x06, 0x07, 0x16, 0x20, 0x21, 0xff} {
			for offset := 0; offset < 31; offset++ {
				for fieldLength := 0; fieldLength <= 255; fieldLength++ {
					for _, payloadLen := range []int{offset + 2, 31} {
						if payloadLen > 31 {
							continue
						}
						raw := make([]byte, payloadLen)
						if offset < len(raw) {
							raw[offset] = byte(fieldLength)
						}
						if offset+1 < len(raw) {
							raw[offset+1] = fieldType
						}
						check(t, string(raw))
					}
				}
			}
		}
	})
}
