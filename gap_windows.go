package bluetooth

import (
	"unsafe"

	"github.com/go-ole/go-ole"
	"github.com/saltosystems/winrt-go"
	"github.com/saltosystems/winrt-go/windows/devices/bluetooth/advertisement"
	"github.com/saltosystems/winrt-go/windows/foundation"
	"github.com/saltosystems/winrt-go/windows/storage/streams"
)

// Address contains a Bluetooth MAC address.
type Address struct {
	MACAddress
}

// Scan starts a BLE scan. It is stopped by a call to StopScan. A common pattern
// is to cancel the scan when a particular device has been found.
func (a *Adapter) Scan(callback func(*Adapter, ScanResult)) (err error) {
	if a.watcher != nil {
		// Cannot scan more than once: which one should ScanStop()
		// stop?
		return errScanning
	}

	a.watcher, err = advertisement.NewBluetoothLEAdvertisementWatcher()
	if err != nil {
		return
	}
	defer func() {
		_ = a.watcher.Release()
		a.watcher = nil
	}()

	// Listen for incoming BLE advertisement packets.
	// We need a TypedEventHandler<TSender, TResult> to listen to events, but since this is a parameterized delegate
	// its GUID depends on the classes used as sender and result, so we need to compute it:
	// TypedEventHandler<BluetoothLEAdvertisementWatcher, BluetoothLEAdvertisementReceivedEventArgs>
	eventReceivedGuid := winrt.ParameterizedInstanceGUID(
		foundation.GUIDTypedEventHandler,
		advertisement.SignatureBluetoothLEAdvertisementWatcher,
		advertisement.SignatureBluetoothLEAdvertisementReceivedEventArgs,
	)
	handler := foundation.NewTypedEventHandler(ole.NewGUID(eventReceivedGuid), func(instance *foundation.TypedEventHandler, sender, arg unsafe.Pointer) {
		args := (*advertisement.BluetoothLEAdvertisementReceivedEventArgs)(arg)
		result := getScanResultFromArgs(args)
		callback(a, result)
	})
	defer handler.Release()

	token, err := a.watcher.AddReceived(handler)
	if err != nil {
		return
	}
	defer a.watcher.RemoveReceived(token)

	// Wait for when advertisement has stopped by a call to StopScan().
	// Advertisement doesn't seem to stop right away, there is an
	// intermediate Stopping state.
	stoppingChan := make(chan struct{})
	// TypedEventHandler<BluetoothLEAdvertisementWatcher, BluetoothLEAdvertisementWatcherStoppedEventArgs>
	eventStoppedGuid := winrt.ParameterizedInstanceGUID(
		foundation.GUIDTypedEventHandler,
		advertisement.SignatureBluetoothLEAdvertisementWatcher,
		advertisement.SignatureBluetoothLEAdvertisementWatcherStoppedEventArgs,
	)
	stoppedHandler := foundation.NewTypedEventHandler(ole.NewGUID(eventStoppedGuid), func(_ *foundation.TypedEventHandler, _, _ unsafe.Pointer) {
		// Note: the args parameter has an Error property that should
		// probably be checked, but I'm not sure when stopping the
		// advertisement watcher could ever result in an error (except
		// for bugs).
		close(stoppingChan)
	})
	defer stoppedHandler.Release()

	token, err = a.watcher.AddStopped(stoppedHandler)
	if err != nil {
		return
	}
	defer a.watcher.RemoveStopped(token)

	err = a.watcher.Start()
	if err != nil {
		return err
	}

	// Wait until advertisement has stopped, and finish.
	<-stoppingChan
	return nil
}

func getScanResultFromArgs(args *advertisement.BluetoothLEAdvertisementReceivedEventArgs) ScanResult {
	// parse bluetooth address
	addr, _ := args.GetBluetoothAddress()
	adr := Address{}
	for i := range adr.MAC {
		adr.MAC[i] = byte(addr)
		addr >>= 8
	}
	sigStrength, _ := args.GetRawSignalStrengthInDBm()
	result := ScanResult{
		RSSI:    sigStrength,
		Address: adr,
	}

	var manufacturerData map[uint16][]byte = make(map[uint16][]byte)
	if winAdv, err := args.GetAdvertisement(); err == nil && winAdv != nil {
		vector, _ := winAdv.GetManufacturerData()
		size, _ := vector.GetSize()
		for i := uint32(0); i < size; i++ {
			element, _ := vector.GetAt(i)
			manData := (*advertisement.BluetoothLEManufacturerData)(element)
			companyID, _ := manData.GetCompanyId()
			buffer, _ := manData.GetData()
			manufacturerData[companyID] = bufferToSlice(buffer)
		}
	}

	// Note: the IsRandom bit is never set.
	advertisement, _ := args.GetAdvertisement()
	localName, _ := advertisement.GetLocalName()
	result.AdvertisementPayload = &advertisementFields{
		AdvertisementFields{
			LocalName:        localName,
			ManufacturerData: manufacturerData,
		},
	}

	return result
}

func bufferToSlice(buffer *streams.IBuffer) []byte {
	dataReader, _ := streams.FromBuffer(buffer)
	defer dataReader.Release()
	bufferSize, _ := buffer.GetLength()
	data, _ := dataReader.ReadBytes(bufferSize)
	return data
}

// StopScan stops any in-progress scan. It can be called from within a Scan
// callback to stop the current scan. If no scan is in progress, an error will
// be returned.
func (a *Adapter) StopScan() error {
	if a.watcher == nil {
		return errNotScanning
	}
	return a.watcher.Stop()
}
