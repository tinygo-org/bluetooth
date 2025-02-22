package bluetooth

import (
	"reflect"
	"strings"
	"testing"
)

func TestUUIDString(t *testing.T) {
	checkUUID(t, New16BitUUID(0x1234), "00001234-0000-1000-8000-00805f9b34fb")
}

func checkUUID(t *testing.T, uuid UUID, check string) {
	if uuid.String() != check {
		t.Errorf("expected UUID %s but got %s", check, uuid.String())
	}
}

func TestParseUUIDTooSmall(t *testing.T) {
	_, e := ParseUUID("00001234-0000-1000-8000-00805f9b34f")
	if e != errInvalidUUID {
		t.Errorf("expected errInvalidUUID but got %v", e)
	}
}

func TestParseUUIDTooLarge(t *testing.T) {
	_, e := ParseUUID("00001234-0000-1000-8000-00805F9B34FB0")
	if e != errInvalidUUID {
		t.Errorf("expected errInvalidUUID but got %v", e)
	}
}

func TestStringUUID(t *testing.T) {
	uuidString := "00001234-0000-1000-8000-00805f9b34fb"
	u, e := ParseUUID(uuidString)
	if e != nil {
		t.Errorf("expected nil but got %v", e)
	}
	if u.String() != uuidString {
		t.Errorf("expected %s but got %s", uuidString, u.String())
	}
}

func TestStringUUIDUpperCase(t *testing.T) {
	uuidString := strings.ToUpper("00001234-0000-1000-8000-00805f9b34fb")
	u, e := ParseUUID(uuidString)
	if e != nil {
		t.Errorf("expected nil but got %v", e)
	}
	if !strings.EqualFold(u.String(), uuidString) {
		t.Errorf("%s does not match %s ignoring case", uuidString, u.String())
	}
}

func TestStringUUIDLowerCase(t *testing.T) {
	uuidString := strings.ToLower("00001234-0000-1000-8000-00805f9b34fb")
	u, e := ParseUUID(uuidString)
	if e != nil {
		t.Errorf("expected nil but got %v", e)
	}
	if !strings.EqualFold(u.String(), uuidString) {
		t.Errorf("%s does not match %s ignoring case", uuidString, u.String())
	}
}

func TestParse16(t *testing.T) {
	uuidString := "00001234-0000-1000-8000-00805f9b34fb"
	u, e := ParseUUID("1234")
	if e != nil {
		t.Errorf("expected nil but got %v", e)
	}
	if !strings.EqualFold(u.String(), uuidString) {
		t.Errorf("%s does not match %s ignoring case", uuidString, u.String())
	}
}

func TestParse32(t *testing.T) {
	uuidString := "abcd1234-0000-1000-8000-00805f9b34fb"
	u, e := ParseUUID("abcd1234")
	if e != nil {
		t.Errorf("expected nil but got %v", e)
	}
	if !strings.EqualFold(u.String(), uuidString) {
		t.Errorf("%s does not match %s ignoring case", uuidString, u.String())
	}
}

func TestNewUUID(t *testing.T) {
	uuidString := "abcd1234-0000-1000-8000-00805f9b34fb"
	uI, e := ParseUUID("abcd1234")
	if e != nil {
		t.Errorf("expected nil but got %v", e)
	}

	b, _ := uI.MarshalBinary()
	u := &UUID{}
	u.UnmarshalBinary(b)
	if !strings.EqualFold(u.String(), uuidString) {
		t.Errorf("%s does not match %s ignoring case", uuidString, u.String())
	}
}

func BenchmarkUUIDToString(b *testing.B) {
	uuid, e := ParseUUID("00001234-0000-1000-8000-00805f9b34fb")
	if e != nil {
		b.Errorf("expected nil but got %v", e)
	}
	for i := 0; i < b.N; i++ {
		_ = uuid.String()
	}
}

func BenchmarkParseUUID(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, e := ParseUUID("00001234-0000-1000-8000-00805f9b34fb")
		if e != nil {
			b.Errorf("expected nil but got %v", e)
		}
	}
}

func TestUUIDUnmarshalText2(t *testing.T) {
	uExpected := &UUID{}
	_ = uExpected.UnmarshalText([]byte("00001234-0000-1000-8000-00805f9b34fb"))
	u := &UUID{}
	_ = u.UnmarshalText([]byte("00001234-0000-1000-8000-00805f9b34fb"))
	if uExpected.String() != u.String() {
		t.Errorf("%s does not match %s ignoring case", uExpected.String(), u.String())
	}
}

func TestInvalidStringUUIDs(t *testing.T) {
	tests := []struct {
		name    string
		uuidStr string
	}{
		{
			name:    "short",
			uuidStr: "00001234-0000-10",
		},
		{
			name:    "even shorter",
			uuidStr: "000012",
		},
		{
			name:    "almost gone",
			uuidStr: "00",
		},
		{
			name:    "empty",
			uuidStr: "",
		},
		{
			name:    "invalid char",
			uuidStr: "1234abcg",
		},
		{
			name:    "invalid char",
			uuidStr: "g234",
		},
	}

	u := &UUID{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := u.UnmarshalText([]byte(tt.uuidStr)); err == nil {
				t.Errorf("UUID.UnmarshalText() did not return an error with invalid uuid: %s", tt.uuidStr)
			}
		})
	}
}

func TestInvalidUUID16(t *testing.T) {
	u := &UUID{}
	for i := 0; i < 4; i++ {
		b := []byte("1234")
		b[i] = byte('g')
		if err := u.UnmarshalText(b); err != errInvalidUUID {
			t.Errorf("UUID.UnmarshalText() did not return an error with invalid hex char at pos: %d", i)
		}
	}
}

func TestInvalidUUID32(t *testing.T) {
	u := &UUID{}
	for i := 0; i < 8; i++ {
		b := []byte("12345678")
		b[i] = byte('g')
		if err := u.UnmarshalText(b); err != errInvalidUUID {
			t.Errorf("UUID.UnmarshalText() did not return an error with invalid hex char at pos: %d", i)
		}
	}
}

func TestInvalidUUID128(t *testing.T) {
	u := &UUID{}
	for i := 0; i < 36; i++ {
		b := []byte("ffffffff-ffff-ffff-ffff-ffffffffffff")
		b[8] = byte('-')
		b[13] = byte('-')
		b[18] = byte('-')
		b[23] = byte('-')
		b[i] = byte('g')
		if err := u.UnmarshalText(b); err != errInvalidUUID {
			t.Errorf("UUID.UnmarshalText() did not return an error with invalid hex char at pos: %d", i)
		}
	}
}

func TestUUID_MarshalText(t *testing.T) {
	tests := []struct {
		name    string
		u       UUID
		want    []byte
		wantErr bool
	}{
		{
			name:    "16bin",
			u:       New16BitUUID(0x1234),
			want:    []byte("00001234-0000-1000-8000-00805f9b34fb"),
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.u.MarshalText()
			if (err != nil) != tt.wantErr {
				t.Errorf("UUID.MarshalText() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("UUID.MarshalText() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUUID_UnmarshalBinary(t *testing.T) {
	tests := []struct {
		name    string
		uuid    []byte
		wantErr bool
	}{
		{
			name:    "Empty",
			uuid:    []byte{},
			wantErr: true,
		},
		{
			name:    "Too small",
			uuid:    []byte{2, 3, 4},
			wantErr: true,
		},
		{
			name:    "Too long",
			uuid:    []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17},
			wantErr: true,
		},
	}

	u := &UUID{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := u.UnmarshalBinary(tt.uuid); (err != nil) != tt.wantErr {
				t.Errorf("UUID.UnmarshalBinary() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestUUID_Replace16BitComponent(t *testing.T) {
	type args struct {
		component uint16
	}
	tests := []struct {
		name      string
		uuid      UUID
		component uint16
		want      UUID
	}{
		{
			name:      "replace",
			uuid:      New16BitUUID(0x1234),
			component: 0x4567,
			want:      New16BitUUID(0x4567),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.uuid.Replace16BitComponent(tt.component); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("UUID.Replace16BitComponent() = %v, want %v", got, tt.want)
			}
		})
	}
}
