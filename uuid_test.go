package bluetooth

import "testing"

func TestUUIDString(t *testing.T) {
	checkUUID(t, New16BitUUID(0x1234), "00001234-0000-1000-8000-00805F9B34FB")
}

func checkUUID(t *testing.T, uuid UUID, check string) {
	if uuid.String() != check {
		t.Errorf("expected UUID %s but got %s", check, uuid.String())
	}
}
