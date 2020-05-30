package winbt

import (
	"syscall"
	"unsafe"

	"github.com/go-ole/go-ole"
)

type IVector struct {
	ole.IInspectable
}

type IVectorVtbl struct {
	ole.IInspectableVtbl

	// These methods have been obtained from windows.foundation.collections.h
	// in the WinRT API.

	// read methods
	GetAt   uintptr // (_In_opt_ unsigned index, _Out_ T_abi *item)
	GetSize uintptr // (_Out_ unsigned *size)
	GetView uintptr // (_Outptr_result_maybenull_ IVectorView<T_logical> **view)
	IndexOf uintptr // (_In_opt_ T_abi value, _Out_ unsigned *index, _Out_ boolean *found)

	// write methods
	SetAt       uintptr // (_In_ unsigned index, _In_opt_ T_abi item)
	InsertAt    uintptr // (_In_ unsigned index, _In_opt_ T_abi item)
	RemoveAt    uintptr // (_In_ unsigned index)
	Append      uintptr // (_In_opt_ T_abi item)
	RemoveAtEnd uintptr // ()
	Clear       uintptr // ()

	// bulk transfer methods
	GetMany    uintptr // (_In_  unsigned startIndex, _In_ unsigned capacity, _Out_writes_to_(capacity,*actual) T_abi *value, _Out_ unsigned *actual)
	ReplaceAll uintptr // (_In_ unsigned count, _In_reads_(count) T_abi *value)
}

func (v *IVector) VTable() *IVectorVtbl {
	return (*IVectorVtbl)(unsafe.Pointer(v.RawVTable))
}

func (v *IVector) At(index int) (element unsafe.Pointer) {
	// The caller will need to cast the element to the correct type (for
	// example, *IBluetoothLEAdvertisementDataSection).
	hr, _, _ := syscall.Syscall(
		v.VTable().GetAt,
		3,
		uintptr(unsafe.Pointer(v)),
		uintptr(index),
		uintptr(unsafe.Pointer(&element)),
	)
	mustSucceed(hr)
	return
}

func (v *IVector) Size() int {
	// Note that because the size is defined as `unsigned`, and `unsigned`
	// means 32-bit in Windows (even 64-bit windows), the size is always a
	// uint32.
	// Casting to int because that is the common data type for sizes in Go. It
	// should practically always fit on 32-bit Windows and definitely always
	// fit on 64-bit Windows (with a 64-bit Go int).
	var size uint32
	hr, _, _ := syscall.Syscall(
		v.VTable().GetSize,
		2,
		uintptr(unsafe.Pointer(v)),
		uintptr(unsafe.Pointer(&size)),
		0)
	mustSucceed(hr)
	return int(size)
}
