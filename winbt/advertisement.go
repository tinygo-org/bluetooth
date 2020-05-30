package winbt

import (
	"syscall"
	"unsafe"

	"github.com/go-ole/go-ole"
)

type WatcherStatus uint32

const (
	WatcherStatusCreated  WatcherStatus = 0
	WatcherStatusStarted  WatcherStatus = 1
	WatcherStatusStopping WatcherStatus = 2
	WatcherStatusStopped  WatcherStatus = 3
	WatcherStatusAborted  WatcherStatus = 4
)

type IBluetoothLEAdvertisementWatcher struct {
	ole.IInspectable
}

type IBluetoothLEAdvertisementWatcherVtbl struct {
	ole.IInspectableVtbl
	GetMinSamplingInterval  uintptr // ([out] [retval] Windows.Foundation.TimeSpan* value);
	GetMaxSamplingInterval  uintptr // ([out] [retval] Windows.Foundation.TimeSpan* value);
	GetMinOutOfRangeTimeout uintptr // ([out] [retval] Windows.Foundation.TimeSpan* value);
	GetMaxOutOfRangeTimeout uintptr // ([out] [retval] Windows.Foundation.TimeSpan* value);
	GetStatus               uintptr // ([out] [retval] Windows.Devices.Bluetooth.Advertisement.BluetoothLEAdvertisementWatcherStatus* value);
	GetScanningMode         uintptr // ([out] [retval] Windows.Devices.Bluetooth.Advertisement.BluetoothLEScanningMode* value);
	SetScanningMode         uintptr // ([in] Windows.Devices.Bluetooth.Advertisement.BluetoothLEScanningMode value);
	GetSignalStrengthFilter uintptr // ([out] [retval] Windows.Devices.Bluetooth.BluetoothSignalStrengthFilter** value);
	SetSignalStrengthFilter uintptr // ([in] Windows.Devices.Bluetooth.BluetoothSignalStrengthFilter* value);
	GetAdvertisementFilter  uintptr // ([out] [retval] Windows.Devices.Bluetooth.Advertisement.BluetoothLEAdvertisementFilter** value);
	SetAdvertisementFilter  uintptr // ([in] Windows.Devices.Bluetooth.Advertisement.BluetoothLEAdvertisementFilter* value);
	Start                   uintptr // ();
	Stop                    uintptr // ();
	AddReceivedEvent        uintptr // ([in] Windows.Foundation.TypedEventHandler<Windows.Devices.Bluetooth.Advertisement.BluetoothLEAdvertisementWatcher*, Windows.Devices.Bluetooth.Advertisement.BluetoothLEAdvertisementReceivedEventArgs*>* handler, [out] [retval] EventRegistrationToken* token);
	RemoveReceivedEvent     uintptr // ([in] EventRegistrationToken token);
	AddStoppedEvent         uintptr // ([in] Windows.Foundation.TypedEventHandler<Windows.Devices.Bluetooth.Advertisement.BluetoothLEAdvertisementWatcher*, Windows.Devices.Bluetooth.Advertisement.BluetoothLEAdvertisementWatcherStoppedEventArgs*>* handler, [out] [retval] EventRegistrationToken* token);
	RemoveStoppedEvent      uintptr // ([in] EventRegistrationToken token);
}

func (v *IBluetoothLEAdvertisementWatcher) VTable() *IBluetoothLEAdvertisementWatcherVtbl {
	return (*IBluetoothLEAdvertisementWatcherVtbl)(unsafe.Pointer(v.RawVTable))
}

func NewBluetoothLEAdvertisementWatcher() (*IBluetoothLEAdvertisementWatcher, error) {
	inspectable, err := ole.RoActivateInstance("Windows.Devices.Bluetooth.Advertisement.BluetoothLEAdvertisementWatcher")
	if err != nil {
		return nil, err
	}
	watcherItf := inspectable.MustQueryInterface(ole.NewGUID("A6AC336F-F3D3-4297-8D6C-C81EA6623F40"))
	return (*IBluetoothLEAdvertisementWatcher)(unsafe.Pointer(watcherItf)), nil
}

func (v *IBluetoothLEAdvertisementWatcher) AddReceivedEvent(handler func(*IBluetoothLEAdvertisementWatcher, *IBluetoothLEAdvertisementReceivedEventArgs)) (err error) {
	event := NewEvent(ole.NewGUID("{90EB4ECA-D465-5EA0-A61C-033C8C5ECEF2}"), func(event *Event, argsInspectable *ole.IInspectable) {
		args := (*IBluetoothLEAdvertisementReceivedEventArgs)(unsafe.Pointer(argsInspectable.MustQueryInterface(IID_IBluetoothLEAdvertisementReceivedEventArgs)))
		defer args.Release()
		handler(v, args)
	})
	hr, _, _ := syscall.Syscall(
		v.VTable().AddReceivedEvent,
		3,
		uintptr(unsafe.Pointer(v)),
		uintptr(unsafe.Pointer(event)),
		uintptr(unsafe.Pointer(&event.token)),
	)
	return makeError(hr)
}

func (v *IBluetoothLEAdvertisementWatcher) AddStoppedEvent(handler func(*IBluetoothLEAdvertisementWatcher, *IBluetoothLEAdvertisementWatcherStoppedEventArgs)) (err error) {
	event := NewEvent(ole.NewGUID("{9936A4DB-DC99-55C3-9E9B-BF4854BD9EAB}"), func(event *Event, argsInspectable *ole.IInspectable) {
		args := (*IBluetoothLEAdvertisementWatcherStoppedEventArgs)(unsafe.Pointer(argsInspectable.MustQueryInterface(IID_IBluetoothLEAdvertisementWatcherStoppedEventArgs)))
		defer args.Release()
		handler(v, args)
	})
	hr, _, _ := syscall.Syscall(
		v.VTable().AddStoppedEvent,
		3,
		uintptr(unsafe.Pointer(v)),
		uintptr(unsafe.Pointer(event)),
		uintptr(unsafe.Pointer(&event.token)),
	)
	return makeError(hr)
}

func (v *IBluetoothLEAdvertisementWatcher) Start() error {
	hr, _, _ := syscall.Syscall(
		v.VTable().Start,
		1,
		uintptr(unsafe.Pointer(v)),
		0,
		0)
	return makeError(hr)
}

func (v *IBluetoothLEAdvertisementWatcher) Stop() error {
	hr, _, _ := syscall.Syscall(
		v.VTable().Stop,
		1,
		uintptr(unsafe.Pointer(v)),
		0,
		0)
	return makeError(hr)
}

func (v *IBluetoothLEAdvertisementWatcher) Status() WatcherStatus {
	var status WatcherStatus
	hr, _, _ := syscall.Syscall(
		v.VTable().GetStatus,
		2,
		uintptr(unsafe.Pointer(v)),
		uintptr(unsafe.Pointer(&status)),
		0)
	mustSucceed(hr)
	return status
}

type IBluetoothLEAdvertisementReceivedEventArgs struct {
	ole.IInspectable
}

type IBluetoothLEAdvertisementReceivedEventArgsVtbl struct {
	ole.IInspectableVtbl
	RawSignalStrengthInDBm uintptr // ([out] [retval] INT16* value);
	BluetoothAddress       uintptr // ([out] [retval] UINT64* value);
	AdvertisementType      uintptr // ([out] [retval] Windows.Devices.Bluetooth.Advertisement.BluetoothLEAdvertisementType* value);
	Timestamp              uintptr // ([out] [retval] Windows.Foundation.DateTime* value);
	Advertisement          uintptr // ([out] [retval] Windows.Devices.Bluetooth.Advertisement.BluetoothLEAdvertisement** value);
}

func (v *IBluetoothLEAdvertisementReceivedEventArgs) VTable() *IBluetoothLEAdvertisementReceivedEventArgsVtbl {
	return (*IBluetoothLEAdvertisementReceivedEventArgsVtbl)(unsafe.Pointer(v.RawVTable))
}

func (v *IBluetoothLEAdvertisementReceivedEventArgs) RawSignalStrengthInDBm() (rssi int16) {
	hr, _, _ := syscall.Syscall(
		v.VTable().RawSignalStrengthInDBm,
		2,
		uintptr(unsafe.Pointer(v)),
		uintptr(unsafe.Pointer(&rssi)),
		0)
	mustSucceed(hr)
	return
}

func (v *IBluetoothLEAdvertisementReceivedEventArgs) BluetoothAddress() (address uint64) {
	hr, _, _ := syscall.Syscall(
		v.VTable().BluetoothAddress,
		2,
		uintptr(unsafe.Pointer(v)),
		uintptr(unsafe.Pointer(&address)),
		0)
	mustSucceed(hr)
	return
}

func (v *IBluetoothLEAdvertisementReceivedEventArgs) Advertisement() (advertisement *IBluetoothLEAdvertisement) {
	hr, _, _ := syscall.Syscall(
		v.VTable().Advertisement,
		2,
		uintptr(unsafe.Pointer(v)),
		uintptr(unsafe.Pointer(&advertisement)),
		0)
	mustSucceed(hr)
	return
}

type IBluetoothLEAdvertisementWatcherStoppedEventArgs struct {
	ole.IInspectable
}

type IBluetoothLEAdvertisement struct {
	ole.IInspectable
}

type IBluetoothLEAdvertisementVtbl struct {
	ole.IInspectableVtbl
	GetFlags                       uintptr // ([out] [retval] Windows.Foundation.IReference<Windows.Devices.Bluetooth.Advertisement.BluetoothLEAdvertisementFlags>** value);
	SetFlags                       uintptr // ([in] Windows.Foundation.IReference<Windows.Devices.Bluetooth.Advertisement.BluetoothLEAdvertisementFlags>* value);
	GetLocalName                   uintptr // ([out] [retval] HSTRING* value);
	SetLocalName                   uintptr // ([in] HSTRING value);
	GetServiceUuids                uintptr // ([out] [retval] Windows.Foundation.Collections.IVector<GUID>** value);
	GetManufacturerData            uintptr // ([out] [retval] Windows.Foundation.Collections.IVector<Windows.Devices.Bluetooth.Advertisement.BluetoothLEManufacturerData*>** value);
	GetDataSections                uintptr // ([out] [retval] Windows.Foundation.Collections.IVector<Windows.Devices.Bluetooth.Advertisement.BluetoothLEAdvertisementDataSection*>** value);
	GetManufacturerDataByCompanyId uintptr // ([in] UINT16 companyId, [out] [retval] Windows.Foundation.Collections.IVectorView<Windows.Devices.Bluetooth.Advertisement.BluetoothLEManufacturerData*>** dataList);
	GetSectionsByType              uintptr // ([in] BYTE type, [out] [retval] Windows.Foundation.Collections.IVectorView<Windows.Devices.Bluetooth.Advertisement.BluetoothLEAdvertisementDataSection*>** sectionList);
}

func (v *IBluetoothLEAdvertisement) VTable() *IBluetoothLEAdvertisementVtbl {
	return (*IBluetoothLEAdvertisementVtbl)(unsafe.Pointer(v.RawVTable))
}

func (v *IBluetoothLEAdvertisement) LocalName() string {
	var hstring ole.HString
	hr, _, _ := syscall.Syscall(
		v.VTable().GetLocalName,
		2,
		uintptr(unsafe.Pointer(v)),
		uintptr(unsafe.Pointer(&hstring)),
		0)
	if hr != 0 {
		// Should not happen.
		panic(ole.NewError(hr))
	}
	name := hstring.String()
	ole.DeleteHString(hstring)
	return name
}

func (v *IBluetoothLEAdvertisement) DataSections() (vector *IVector) {
	hr, _, _ := syscall.Syscall(
		v.VTable().GetDataSections,
		2,
		uintptr(unsafe.Pointer(v)),
		uintptr(unsafe.Pointer(&vector)),
		0)
	mustSucceed(hr)
	return
}

type IBluetoothLEAdvertisementDataSection struct {
	ole.IInspectable
}

type IBluetoothLEAdvertisementDataSectionVtbl struct {
	ole.IInspectableVtbl
	GetDataType uintptr // ([out] [retval] BYTE* value)
	SetDataType uintptr // ([in] BYTE value)
	GetData     uintptr // ([out] [retval] Windows.Storage.Streams.IBuffer** value)
	SetData     uintptr // ([in] Windows.Storage.Streams.IBuffer* value)
}

func (v *IBluetoothLEAdvertisementDataSection) VTable() *IBluetoothLEAdvertisementDataSectionVtbl {
	return (*IBluetoothLEAdvertisementDataSectionVtbl)(unsafe.Pointer(v.RawVTable))
}

func (v *IBluetoothLEAdvertisementDataSection) DataType() (value byte) {
	hr, _, _ := syscall.Syscall(
		v.VTable().GetDataType,
		2,
		uintptr(unsafe.Pointer(v)),
		uintptr(unsafe.Pointer(&value)),
		0)
	mustSucceed(hr)
	return
}
