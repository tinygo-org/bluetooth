package winbt

import (
	"syscall"
	"unsafe"

	"github.com/go-ole/go-ole"
)

// IBuffer Represents a referenced array of bytes used by
// byte stream read and write interfaces. Buffer is the class
// implementation of this interface.
type IBuffer struct {
	ole.IInspectable
}

type IBufferVtbl struct {
	ole.IInspectableVtbl

	// These methods have been obtained from windows.storage.streams.h in the WinRT API.

	// read methods
	GetCapacity uintptr // ([out] [retval] UINT32* value)
	GetLength   uintptr // ([out] [retval] UINT32* value)

	// write methods
	SetLength uintptr // ([in] UINT32 value);
}

func (v *IBuffer) VTable() *IBufferVtbl {
	return (*IBufferVtbl)(unsafe.Pointer(v.RawVTable))
}

func (v *IBuffer) Length() int {
	var n int
	hr, _, _ := syscall.SyscallN(
		v.VTable().GetLength,
		uintptr(unsafe.Pointer(v)),
		uintptr(unsafe.Pointer(&n)),
	)
	mustSucceed(hr)
	return n
}

func (v *IBuffer) Bytes() ([]byte, error) {
	// Get DataReaderStatics: we need to pass the class name, and the iid of the interface
	// GUID: https://github.com/tpn/winsdk-10/blob/9b69fd26ac0c7d0b83d378dba01080e93349c2ed/Include/10.0.14393.0/winrt/windows.storage.streams.idl#L311
	inspectable, err := ole.RoGetActivationFactory("Windows.Storage.Streams.DataReader", ole.NewGUID("11FCBFC8-F93A-471B-B121-F379E349313C"))
	if err != nil {
		return nil, err
	}

	drStatics := (*IDataReaderStatics)(unsafe.Pointer(inspectable))

	// Call FromBuffer to create new DataReader
	var dr *IDataReader
	hr, _, _ := syscall.SyscallN(
		drStatics.VTable().FromBuffer,
		0,                            // this is a static func, so there's no this
		uintptr(unsafe.Pointer(v)),   // in buffer
		uintptr(unsafe.Pointer(&dr)), // out DataReader
	)
	err = makeError(hr)
	if err != nil {
		return nil, err
	}

	data := make([]byte, v.Length())
	err = dr.Bytes(data)
	return data, err
}

type IDataReaderStatics struct {
	ole.IInspectable
}

type IDataReaderStaticsVtbl struct {
	ole.IInspectableVtbl

	FromBuffer uintptr // ([in] Windows.Storage.Streams.IBuffer* buffer, [out] [retval] Windows.Storage.Streams.DataReader** dataReader);
}

func (v *IDataReaderStatics) VTable() *IDataReaderStaticsVtbl {
	return (*IDataReaderStaticsVtbl)(unsafe.Pointer(v.RawVTable))
}

type IDataReader struct {
	ole.IInspectable
}

type IDataReaderVtbl struct {
	ole.IInspectableVtbl

	GetUnconsumedBufferLength uintptr // ([out] [retval] UINT32* value);
	GetUnicodeEncoding        uintptr // ([out] [retval] Windows.Storage.Streams.UnicodeEncoding* value);
	PutUnicodeEncoding        uintptr // ([in] Windows.Storage.Streams.UnicodeEncoding value);
	GetByteOrder              uintptr // ([out] [retval] Windows.Storage.Streams.ByteOrder* value);
	PutByteOrder              uintptr // ([in] Windows.Storage.Streams.ByteOrder value);
	GetInputStreamOptions     uintptr // ([out] [retval] Windows.Storage.Streams.InputStreamOptions* value);
	PutInputStreamOptions     uintptr // ([in] Windows.Storage.Streams.InputStreamOptions value);
	ReadByte                  uintptr // ([out] [retval] BYTE* value);
	ReadBytes                 uintptr // ([in] UINT32 __valueSize, [out] [size_is(__valueSize)] BYTE* value);
	ReadBuffer                uintptr // ([in] UINT32 length, [out] [retval] Windows.Storage.Streams.IBuffer** buffer);
	ReadBoolean               uintptr // ([out] [retval] boolean* value);
	ReadGuid                  uintptr // ([out] [retval] GUID* value);
	ReadInt16                 uintptr // ([out] [retval] INT16* value);
	ReadInt32                 uintptr // ([out] [retval] INT32* value);
	ReadInt64                 uintptr // ([out] [retval] INT64* value);
	ReadUInt16                uintptr // ([out] [retval] UINT16* value);
	ReadUInt32                uintptr // ([out] [retval] UINT32* value);
	ReadUInt64                uintptr // ([out] [retval] UINT64* value);
	ReadSingle                uintptr // ([out] [retval] FLOAT* value);
	ReadDouble                uintptr // ([out] [retval] DOUBLE* value);
	ReadString                uintptr // ([in] UINT32 codeUnitCount, [out] [retval] HSTRING* value);
	ReadDateTime              uintptr // ([out] [retval] Windows.Foundation.DateTime* value);
	ReadTimeSpan              uintptr // ([out] [retval] Windows.Foundation.TimeSpan* value);
	LoadAsync                 uintptr // ([in] UINT32 count, [out] [retval] Windows.Storage.Streams.DataReaderLoadOperation** operation);
	DetachBuffer              uintptr // ([out] [retval] Windows.Storage.Streams.IBuffer** buffer);
	DetachStream              uintptr // ([out] [retval] Windows.Storage.Streams.IInputStream** stream);*/
}

func (v *IDataReader) VTable() *IDataReaderVtbl {
	return (*IDataReaderVtbl)(unsafe.Pointer(v.RawVTable))
}

// Bytes fills the incoming array with the data from the buffer
func (v *IDataReader) Bytes(b []byte) error {
	// ([in] UINT32 __valueSize, [out] [size_is(__valueSize)] BYTE* value);
	size := len(b)
	hr, _, _ := syscall.SyscallN(
		v.VTable().ReadBytes,
		uintptr(unsafe.Pointer(v)),
		uintptr(size),
		uintptr(unsafe.Pointer(&b[0])),
	)
	return makeError(hr)
}
