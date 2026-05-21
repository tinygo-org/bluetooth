package bluetooth

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"syscall"
	"unsafe"

	"github.com/go-ole/go-ole"
	"github.com/saltosystems/winrt-go"
	"github.com/saltosystems/winrt-go/windows/devices/bluetooth"
	"github.com/saltosystems/winrt-go/windows/devices/bluetooth/advertisement"
	"github.com/saltosystems/winrt-go/windows/devices/bluetooth/genericattributeprofile"
	"github.com/saltosystems/winrt-go/windows/foundation"
	"github.com/saltosystems/winrt-go/windows/storage/streams"
)

// Address contains a Bluetooth MAC address.
type Address struct {
	MACAddress
}

type Advertisement struct {
	advertisement *advertisement.BluetoothLEAdvertisement
	publisher     *advertisement.BluetoothLEAdvertisementPublisher
}

// DefaultAdvertisement returns the default advertisement instance but does not
// configure it.
func (a *Adapter) DefaultAdvertisement() *Advertisement {
	if a.defaultAdvertisement == nil {
		a.defaultAdvertisement = &Advertisement{}
	}

	return a.defaultAdvertisement
}

// Configure this advertisement.
// on Windows we're only able to set "Manufacturer Data" for advertisements.
// https://learn.microsoft.com/en-us/uwp/api/windows.devices.bluetooth.advertisement.bluetoothleadvertisementpublisher?view=winrt-22621#remarks
// following this c# source for this implementation: https://github.com/microsoft/Windows-universal-samples/blob/main/Samples/BluetoothAdvertisement/cs/Scenario2_Publisher.xaml.cs
// adding service data / localname leads to errors when starting the advertisement.
func (a *Advertisement) Configure(options AdvertisementOptions) error {
	// we can only advertise manufacturer / company data on windows, so no need to continue if we have none
	if len(options.ManufacturerData) == 0 {
		return nil
	}

	if a.publisher != nil {
		a.publisher.Release()
	}

	if a.advertisement != nil {
		a.advertisement.Release()
	}

	pub, err := advertisement.NewBluetoothLEAdvertisementPublisher()
	if err != nil {
		return err
	}

	a.publisher = pub

	ad, err := a.publisher.GetAdvertisement()
	if err != nil {
		return err
	}

	a.advertisement = ad

	vec, err := ad.GetManufacturerData()
	if err != nil {
		return err
	}
	defer vec.Release()

	for _, optManData := range options.ManufacturerData {
		if err := appendManufacturerData(vec, optManData); err != nil {
			return err
		}
	}

	return nil
}

// appendManufacturerData writes one ManufacturerDataElement into the given
// WinRT vector, releasing every intermediate WinRT object on the way out so
// they are not leaked per advertisement entry.
func appendManufacturerData(vec interface {
	Append(unsafe.Pointer) error
}, optManData ManufacturerDataElement) error {
	writer, err := streams.NewDataWriter()
	if err != nil {
		return err
	}
	// Release the writer at end of *this iteration*, not at end of function.
	defer writer.Release()

	if err := writer.WriteBytes(uint32(len(optManData.Data)), optManData.Data); err != nil {
		return err
	}

	buf, err := writer.DetachBuffer()
	if err != nil {
		return err
	}
	defer buf.Release()

	manData, err := advertisement.BluetoothLEManufacturerDataCreate(optManData.CompanyID, buf)
	if err != nil {
		return err
	}
	defer manData.Release()

	return vec.Append(unsafe.Pointer(&manData.IUnknown.RawVTable))
}

// Start advertisement. May only be called after it has been configured.
func (a *Advertisement) Start() error {
	// publisher will be present if we actually have manufacturer data to advertise.
	if a.publisher != nil {
		return a.publisher.Start()
	}

	return nil
}

// Stop advertisement. May only be called after it has been started.
func (a *Advertisement) Stop() error {
	if a.publisher != nil {
		return a.publisher.Stop()
	}

	return nil
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

	// Set scanning mode to active so we receive scan responses
	// from devices in advertising mode
	err = a.watcher.SetScanningMode(advertisement.BluetoothLEScanningModeActive)
	if err != nil {
		return
	}

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
	stoppingChan := make(chan error)
	// TypedEventHandler<BluetoothLEAdvertisementWatcher, BluetoothLEAdvertisementWatcherStoppedEventArgs>
	eventStoppedGuid := winrt.ParameterizedInstanceGUID(
		foundation.GUIDTypedEventHandler,
		advertisement.SignatureBluetoothLEAdvertisementWatcher,
		advertisement.SignatureBluetoothLEAdvertisementWatcherStoppedEventArgs,
	)
	stoppedHandler := foundation.NewTypedEventHandler(ole.NewGUID(eventStoppedGuid), func(_ *foundation.TypedEventHandler, _, arg unsafe.Pointer) {
		args := (*advertisement.BluetoothLEAdvertisementWatcherStoppedEventArgs)(arg)
		errCode, err := args.GetError()
		if err != nil {
			// Got an error while getting the error value, that shouldn't
			// happen.
			stoppingChan <- fmt.Errorf("failed to get stopping error value: %w", err)
		} else if errCode != bluetooth.BluetoothErrorSuccess {
			// Could not stop the scan? I'm not sure when this would actually
			// happen.
			stoppingChan <- fmt.Errorf("failed to stop scanning (error code %d)", errCode)
		}
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
	return <-stoppingChan
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

	winAdv, err := args.GetAdvertisement()
	if err != nil || winAdv == nil {
		return result
	}
	defer winAdv.Release()

	var manufacturerData []ManufacturerDataElement
	var serviceUUIDs []UUID

	// Extract manufacturer data
	if manVec, err := winAdv.GetManufacturerData(); err == nil && manVec != nil {
		size, _ := manVec.GetSize()
		for i := uint32(0); i < size; i++ {
			element, err := manVec.GetAt(i)
			if err != nil || element == nil {
				continue
			}
			manData := (*advertisement.BluetoothLEManufacturerData)(element)

			companyID, _ := manData.GetCompanyId()
			if buffer, err := manData.GetData(); err == nil && buffer != nil {
				manufacturerData = append(manufacturerData, ManufacturerDataElement{
					CompanyID: companyID,
					Data:      bufferToSlice(buffer),
				})
			}
			manData.Release()
		}
		manVec.Release()
	}

	// Extract service UUIDs
	if uuidVec, err := winAdv.GetServiceUuids(); err == nil && uuidVec != nil {
		size, _ := uuidVec.GetSize()
		for i := uint32(0); i < size; i++ {
			element, err := uuidVec.GetAt(i)
			if err != nil {
				continue
			}
			// element is not a pointer, but a GUID struct. But we cannot convert
			// unsafe.Pointer to a non-pointer type, so instead we are doing this:
			serviceGUID := (*syscall.GUID)(unsafe.Pointer(&element))
			uuid := GUIDToUUID(*serviceGUID)
			serviceUUIDs = append(serviceUUIDs, uuid)
		}
		uuidVec.Release()
	}

	// Note: the IsRandom bit is never set.
	localName, _ := winAdv.GetLocalName()
	result.AdvertisementPayload = &advertisementFields{
		AdvertisementFields{
			LocalName:        localName,
			ServiceUUIDs:     serviceUUIDs,
			ManufacturerData: manufacturerData,
		},
	}

	return result
}

func GUIDToUUID(guid syscall.GUID) UUID {
	return NewUUID([16]byte{
		byte(guid.Data1 >> 24),
		byte(guid.Data1 >> 16),
		byte(guid.Data1 >> 8),
		byte(guid.Data1),
		byte(guid.Data2 >> 8),
		byte(guid.Data2),
		byte(guid.Data3 >> 8),
		byte(guid.Data3),
		guid.Data4[0], guid.Data4[1],
		guid.Data4[2], guid.Data4[3],
		guid.Data4[4], guid.Data4[5],
		guid.Data4[6], guid.Data4[7],
	})
}

// bufferToSlice reads the contents of buffer into a Go slice and releases the
// IBuffer (so callers do not need to). The DataReader created here is also
// released before returning.
func bufferToSlice(buffer *streams.IBuffer) []byte {
	if buffer == nil {
		return nil
	}
	defer buffer.Release()

	dataReader, err := streams.DataReaderFromBuffer(buffer)
	if err != nil || dataReader == nil {
		return nil
	}
	defer dataReader.Release()

	bufferSize, _ := buffer.GetLength()
	if bufferSize == 0 {
		return nil
	}
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

var _ GAPDevice = Device{}

// Device is a connection to a remote peripheral.
type Device struct {
	// All mutable / releasable state lives behind a pointer so that the
	// value-receiver methods (Connected, Disconnect, ...) share the same
	// underlying lifecycle. This is also what makes Disconnect safe to call
	// multiple times: closeOnce guarantees we only release the COM objects
	// once even if the ConnectionStatusChanged callback also calls
	// Disconnect().
	*deviceState

	Address Address // the MAC address of the device
}

type deviceState struct {
	ctx    context.Context
	cancel context.CancelFunc

	device                        *bluetooth.BluetoothLEDevice
	session                       *genericattributeprofile.GattSession
	connectionStatusListenerToken foundation.EventRegistrationToken
	connectionStatusListener      *foundation.TypedEventHandler

	closeOnce sync.Once
	closeErr  error
}

// Connect starts a connection attempt to the given peripheral device address.
//
// On Linux and Windows, the IsRandom part of the address is ignored.
func (a *Adapter) Connect(address Address, params ConnectionParams) (Device, error) {
	var winAddr uint64
	for i := range address.MAC {
		winAddr += uint64(address.MAC[i]) << (8 * i)
	}

	// IAsyncOperation<BluetoothLEDevice>
	bleDeviceOp, err := bluetooth.BluetoothLEDeviceFromBluetoothAddressAsync(winAddr)
	if err != nil {
		return Device{}, err
	}
	defer bleDeviceOp.Release()

	// We need to pass the signature of the parameter returned by the async operation:
	// IAsyncOperation<BluetoothLEDevice>
	if err := awaitAsyncOperation(bleDeviceOp, bluetooth.SignatureBluetoothLEDevice); err != nil {
		return Device{}, fmt.Errorf("error connecting to device: %w", err)
	}

	res, err := bleDeviceOp.GetResults()
	if err != nil {
		return Device{}, err
	}

	// The returned BluetoothLEDevice is set to null if FromBluetoothAddressAsync can't find the device identified by bluetoothAddress
	if uintptr(res) == 0x0 {
		return Device{}, fmt.Errorf("device with the given address was not found")
	}

	bleDevice := (*bluetooth.BluetoothLEDevice)(res)

	// Creating a BluetoothLEDevice object by calling this method alone doesn't (necessarily) initiate a connection.
	// To initiate a connection, we need to set GattSession.MaintainConnection to true.
	dID, err := bleDevice.GetBluetoothDeviceId()
	if err != nil {
		bleDevice.Release()
		return Device{}, err
	}
	defer dID.Release()

	// Windows does not support explicitly connecting to a device.
	// Instead it has the concept of a GATT session that is owned
	// by the calling program.
	gattSessionOp, err := genericattributeprofile.GattSessionFromDeviceIdAsync(dID) // IAsyncOperation<GattSession>
	if err != nil {
		bleDevice.Release()
		return Device{}, err
	}
	defer gattSessionOp.Release()

	if err := awaitAsyncOperation(gattSessionOp, genericattributeprofile.SignatureGattSession); err != nil {
		bleDevice.Release()
		return Device{}, fmt.Errorf("error getting gatt session: %w", err)
	}

	gattRes, err := gattSessionOp.GetResults()
	if err != nil {
		bleDevice.Release()
		return Device{}, err
	}
	newSession := (*genericattributeprofile.GattSession)(gattRes)
	// This keeps the device connected until we set maintain_connection = False.
	if err := newSession.SetMaintainConnection(true); err != nil {
		newSession.Release()
		bleDevice.Release()
		return Device{}, err
	}

	ctx, cancel := context.WithCancel(context.Background())

	state := &deviceState{
		ctx:     ctx,
		cancel:  cancel,
		device:  bleDevice,
		session: newSession,
	}
	device := Device{
		deviceState: state,
		Address:     address,
	}

	// https://learn.microsoft.com/es-es/uwp/api/windows.devices.bluetooth.bluetoothledevice.connectionstatuschanged?view=winrt-26100
	// TypedEventHandler<BluetoothLEDevice,object>
	connectionStatusChangedGUID := winrt.ParameterizedInstanceGUID(
		foundation.GUIDTypedEventHandler,
		bluetooth.SignatureBluetoothLEDevice,
		"cinterface(IInspectable)", // object
	)

	handler := foundation.NewTypedEventHandler(ole.NewGUID(connectionStatusChangedGUID), func(instance *foundation.TypedEventHandler, sender, arg unsafe.Pointer) {
		status, err := bleDevice.GetConnectionStatus()
		if err != nil {
			return
		}
		if status == bluetooth.BluetoothConnectionStatusDisconnected {
			device.Disconnect()
		}

		if a.connectHandler != nil {
			a.connectHandler(device, status == bluetooth.BluetoothConnectionStatusConnected)
		}
	})

	token, err := state.device.AddConnectionStatusChanged(handler)
	if err != nil {
		handler.Release()
		_ = newSession.Close()
		newSession.Release()
		_ = bleDevice.Close()
		bleDevice.Release()
		cancel()
		return Device{}, err
	}
	state.connectionStatusListenerToken = token
	state.connectionStatusListener = handler

	return device, nil
}

// Disconnect from the BLE device. This method is non-blocking and does not
// wait until the connection is fully gone.
//
// Disconnect is safe to call multiple times: subsequent calls are no-ops and
// return the original disconnect error (if any).
func (d Device) Disconnect() error {
	if d.deviceState == nil {
		return nil
	}
	d.closeOnce.Do(func() {
		d.closeErr = d.shutdown()
	})
	return d.closeErr
}

func (s *deviceState) shutdown() error {
	s.cancel()

	var firstErr error
	if s.session != nil {
		if err := s.session.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
		s.session.Release()
		s.session = nil
	}

	if s.device != nil {
		if s.connectionStatusListener != nil {
			_ = s.device.RemoveConnectionStatusChanged(s.connectionStatusListenerToken)
		}
		if err := s.device.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
		s.device.Release()
		s.device = nil
	}

	if s.connectionStatusListener != nil {
		s.connectionStatusListener.Release()
		s.connectionStatusListener = nil
	}

	return firstErr
}

// Connected returns whether the device is currently connected.
func (d Device) Connected() (bool, error) {
	if d.deviceState == nil || d.device == nil {
		return false, nil
	}
	status, err := d.device.GetConnectionStatus()
	if err != nil {
		return false, err
	}
	return status == bluetooth.BluetoothConnectionStatusConnected, nil
}

// RequestConnectionParams requests a different connection latency and timeout
// of the given device connection. Fields that are unset will be left alone.
// Whether or not the device will actually honor this, depends on the device and
// on the specific parameters.
//
// On Windows, this call doesn't do anything.
func (d Device) RequestConnectionParams(params ConnectionParams) error {
	// TODO: implement this using
	// BluetoothLEDevice.RequestPreferredConnectionParameters.
	return nil
}

// SetRandomAddress sets the random address to be used for advertising.
func (a *Adapter) SetRandomAddress(mac MAC) error {
	return errors.ErrUnsupported
}
