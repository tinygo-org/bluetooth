package bluetooth

import (
	"testing"

	"github.com/saltosystems/winrt-go/windows/devices/bluetooth/genericattributeprofile"
)

func TestOpenServicesAddOnce(t *testing.T) {
	var s openServices
	a := &genericattributeprofile.GattDeviceService{}
	b := &genericattributeprofile.GattDeviceService{}
	s.add(a)
	s.add(b)
	s.add(a)
	if len(s.list) != 2 {
		t.Fatalf("got %d services, want 2", len(s.list))
	}
}
