package bluetooth

import (
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
