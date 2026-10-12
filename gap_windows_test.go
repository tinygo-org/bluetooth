package bluetooth

import (
	"reflect"
	"syscall"
	"testing"
	"time"
	"unsafe"

	"github.com/saltosystems/winrt-go/windows/foundation/collections"
)

// guidVector returns an IVector<GUID> whose GetAt writes whole GUIDs like WinRT does.
func guidVector(guids []syscall.GUID) *collections.IVector {
	const eBounds = 0x8000000B
	vtbl := &collections.IVectorVtbl{
		GetAt: syscall.NewCallback(func(this, index uintptr, out *syscall.GUID) uintptr {
			if index >= uintptr(len(guids)) {
				return eBounds
			}
			*out = guids[index]
			return 0
		}),
	}
	v := &collections.IVector{}
	v.RawVTable = (*interface{})(unsafe.Pointer(vtbl))
	return v
}

func TestGUIDVectorAt(t *testing.T) {
	want := syscall.GUID{
		Data1: 0x0000cdab,
		Data2: 0x0000,
		Data3: 0x1000,
		Data4: [8]byte{0x80, 0x00, 0x00, 0x80, 0x5f, 0x9b, 0x34, 0xfb},
	}
	v := guidVector([]syscall.GUID{{}, want})

	got, err := guidVectorAt(v, 1)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("guidVectorAt = %+v, want %+v", got, want)
	}
	if uuid, want := GUIDToUUID(got).String(), "0000cdab-0000-1000-8000-00805f9b34fb"; uuid != want {
		t.Errorf("UUID = %s, want %s", uuid, want)
	}

	if _, err := guidVectorAt(v, 2); err == nil {
		t.Error("guidVectorAt out of range: want an error")
	}
}

func TestMergeAdvertisementFields(t *testing.T) {
	service := New16BitUUID(0xcdab)
	adv := AdvertisementFields{ServiceUUIDs: []UUID{service}}
	scanResponse := AdvertisementFields{LocalName: "Frame"}

	got := mergeAdvertisementFields(adv, scanResponse)
	if got.LocalName != "Frame" || len(got.ServiceUUIDs) != 1 || got.ServiceUUIDs[0] != service {
		t.Errorf("scan response after advertisement = %+v, want the name and the service", got)
	}
	got = mergeAdvertisementFields(got, AdvertisementFields{ServiceUUIDs: []UUID{service}})
	if got.LocalName != "Frame" {
		t.Errorf("advertisement after scan response = %+v, want the name kept", got)
	}
	got = mergeAdvertisementFields(got, AdvertisementFields{LocalName: "Renamed"})
	if got.LocalName != "Renamed" {
		t.Errorf("new name = %q, want Renamed", got.LocalName)
	}
}

func TestMergeManufacturerDataByCompanyID(t *testing.T) {
	adv := AdvertisementFields{ManufacturerData: []ManufacturerDataElement{
		{CompanyID: 0x004c, Data: []byte{1}},
		{CompanyID: 0xffff, Data: []byte{2}},
	}}
	scanResponse := AdvertisementFields{ManufacturerData: []ManufacturerDataElement{
		{CompanyID: 0xffff, Data: []byte{3}},
	}}

	got := mergeAdvertisementFields(adv, scanResponse).ManufacturerData
	want := []ManufacturerDataElement{
		{CompanyID: 0x004c, Data: []byte{1}},
		{CompanyID: 0xffff, Data: []byte{3}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("manufacturer data = %+v, want %+v", got, want)
	}
}

func TestMergeKeepsOrder(t *testing.T) {
	a, b := New16BitUUID(0xaaaa), New16BitUUID(0xbbbb)
	adv := AdvertisementFields{ServiceUUIDs: []UUID{a}}
	scanResponse := AdvertisementFields{ServiceUUIDs: []UUID{b}}

	got := mergeAdvertisementFields(AdvertisementFields{}, adv)
	got = mergeAdvertisementFields(got, scanResponse)
	got = mergeAdvertisementFields(got, adv)
	if want := []UUID{a, b}; !reflect.DeepEqual(got.ServiceUUIDs, want) {
		t.Errorf("services = %v, want %v", got.ServiceUUIDs, want)
	}
}

func testAddress(i int) Address {
	return Address{MACAddress{MAC: MAC{0, 0, 0, 0, byte(i >> 8), byte(i)}}}
}

func TestScanCacheLimit(t *testing.T) {
	var c scanCache
	for i := 0; i < scanCacheSize; i++ {
		c.merge(testAddress(i), AdvertisementFields{LocalName: "Device"})
	}
	// See device 0 again, so device 1 is now the one seen longest ago.
	c.merge(testAddress(0), AdvertisementFields{})
	c.merge(testAddress(scanCacheSize), AdvertisementFields{LocalName: "New"})

	if len(c.devices) != scanCacheSize {
		t.Errorf("cache has %d devices, want %d", len(c.devices), scanCacheSize)
	}
	if _, ok := c.devices[testAddress(1)]; ok {
		t.Error("device seen longest ago was kept")
	}
	if _, ok := c.devices[testAddress(0)]; !ok {
		t.Error("device seen recently was dropped")
	}
}

func TestScanCacheReturnsCopies(t *testing.T) {
	var c scanCache
	addr := testAddress(0)
	service := New16BitUUID(0xcdab)
	first := c.merge(addr, AdvertisementFields{
		ServiceUUIDs:     []UUID{service},
		ManufacturerData: []ManufacturerDataElement{{CompanyID: 0xffff, Data: []byte{1}}},
	})
	first.ServiceUUIDs[0] = New16BitUUID(0x1234)
	first.ManufacturerData[0].Data[0] = 9

	got := c.merge(addr, AdvertisementFields{LocalName: "Frame"})
	if got.ServiceUUIDs[0] != service {
		t.Errorf("service = %s after changing an earlier result, want %s", got.ServiceUUIDs[0], service)
	}
	if got.ManufacturerData[0].Data[0] != 1 {
		t.Errorf("manufacturer data = %v after changing an earlier result, want [1]", got.ManufacturerData[0].Data)
	}
}

func TestWaitConnected(t *testing.T) {
	never := make(chan struct{})
	connected := func(ok bool) func() (bool, error) {
		return func() (bool, error) { return ok, nil }
	}

	if err := waitConnected(never, connected(true), time.Second); err != nil {
		t.Errorf("already connected: %v", err)
	}

	ch := make(chan struct{}, 1)
	go func() {
		time.Sleep(20 * time.Millisecond)
		ch <- struct{}{}
	}()
	if err := waitConnected(ch, connected(false), time.Second); err != nil {
		t.Errorf("connected later: %v", err)
	}

	start := time.Now()
	if err := waitConnected(never, connected(false), 50*time.Millisecond); err == nil {
		t.Error("never connected: want an error")
	}
	if d := time.Since(start); d < 50*time.Millisecond {
		t.Errorf("never connected returned after %s, want the timeout", d)
	}
}
