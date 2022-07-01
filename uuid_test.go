package bluetooth

import (
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

func BenchmarkUUIDToString(b *testing.B) {
	uuid, e := ParseUUID("00001234-0000-1000-8000-00805f9b34fb")
	if e != nil {
		b.Errorf("expected nil but got %v", e)
	}
	for i := 0; i < b.N; i++ {
		_ = uuid.String()
	}
}
