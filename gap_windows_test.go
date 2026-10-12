package bluetooth

import (
	"syscall"
	"testing"
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
