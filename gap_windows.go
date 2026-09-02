package bluetooth

import (
	"context"
	"errors"
	"fmt"
	"sync"
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

	for _, optManData := range options.ManufacturerData {
		writer, err := streams.NewDataWriter()
		if err != nil {
			return err
		}
		defer writer.Release()

		err = writer.WriteBytes(uint32(len(optManData.Data)), optManData.Data)
		if err != nil {
			return err
		}

		buf, err := writer.DetachBuffer()
		if err != nil {
			return err
		}

		manData, err := advertisement.BluetoothLEManufacturerDataCreate(optManData.CompanyID, buf)
		if err != nil {
			return err
		}

		if err = vec.Append(unsafe.Pointer(&manData.IUnknown.RawVTable)); err != nil {
			return err
		}
	}

	return nil
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
	if err != nil {
		return result
	}
	defer winAdv.Release()

	var manufacturerData []ManufacturerDataElement
	mVector, _ := winAdv.GetManufacturerData()
	if mVector != nil {
		defer mVector.Release()
		mSize, _ := mVector.GetSize()
		for i := uint32(0); i < mSize; i++ {
			element, _ := mVector.GetAt(i)
			manData := (*advertisement.BluetoothLEManufacturerData)(element)
			companyID, _ := manData.GetCompanyId()
			buffer, _ := manData.GetData()
			manufacturerData = append(manufacturerData, ManufacturerDataElement{
				CompanyID: companyID,
				Data:      bufferToSlice(buffer),
			})
			buffer.Release()
			manData.Release()
		}
	}

	// Note: the IsRandom bit is never set.
	localName, _ := winAdv.GetLocalName()
	result.AdvertisementPayload = &advertisementFields{
		AdvertisementFields{
			LocalName:        localName,
			ManufacturerData: manufacturerData,
		},
	}

	return result
}

func bufferToSlice(buffer *streams.IBuffer) []byte {
	dataReader, _ := streams.DataReaderFromBuffer(buffer)
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
	ctx    context.Context
	cancel context.CancelFunc

	Address Address // the MAC address of the device

	device                        *bluetooth.BluetoothLEDevice
	session                       *genericattributeprofile.GattSession
	connectionStatusListenerToken foundation.EventRegistrationToken
	connectionStatusListener      *foundation.TypedEventHandler
	connParams                    *connectionParamsState
}

// connectionParamsState keeps the open connection parameters request.
// Windows applies the request only while the object is open:
// https://learn.microsoft.com/en-us/uwp/api/windows.devices.bluetooth.bluetoothledevice.requestpreferredconnectionparameters
type connectionParamsState struct {
	mu      sync.Mutex
	request *bluetooth.BluetoothLEPreferredConnectionParametersRequest
}

// replace closes the open request and then keeps the new one.
// A nil request only closes the open request and restores the system defaults.
func (s *connectionParamsState) replace(request *bluetooth.BluetoothLEPreferredConnectionParametersRequest) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.request != nil {
		_ = s.request.Close()
		s.request.Release()
	}
	s.request = request
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
		return Device{}, err
	}

	// Windows does not support explicitly connecting to a device.
	// Instead it has the concept of a GATT session that is owned
	// by the calling program.
	gattSessionOp, err := genericattributeprofile.GattSessionFromDeviceIdAsync(dID) // IAsyncOperation<GattSession>
	if err != nil {
		return Device{}, err
	}

	if err := awaitAsyncOperation(gattSessionOp, genericattributeprofile.SignatureGattSession); err != nil {
		return Device{}, fmt.Errorf("error getting gatt session: %w", err)
	}

	gattRes, err := gattSessionOp.GetResults()
	if err != nil {
		return Device{}, err
	}
	newSession := (*genericattributeprofile.GattSession)(gattRes)
	// This keeps the device connected until we set maintain_connection = False.
	if err := newSession.SetMaintainConnection(true); err != nil {
		return Device{}, err
	}

	ctx, cancel := context.WithCancel(context.Background())

	device := Device{
		ctx:    ctx,
		cancel: cancel,

		Address: address,

		device:     bleDevice,
		session:    newSession,
		connParams: &connectionParamsState{},
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

	token, err := device.device.AddConnectionStatusChanged(handler)

	device.connectionStatusListenerToken = token
	device.connectionStatusListener = handler

	if err != nil {
		_ = handler.Release()
		return device, err
	}

	return device, nil
}

// Disconnect from the BLE device. This method is non-blocking and does not
// wait until the connection is fully gone.
func (d Device) Disconnect() error {
	defer d.device.Release()
	defer d.session.Release()
	if d.connectionStatusListener != nil {
		defer d.connectionStatusListener.Release()
	}

	d.cancel()

	// Close the request while the device is still open.
	if d.connParams != nil {
		d.connParams.replace(nil)
	}

	if err := d.session.Close(); err != nil {
		return err
	}

	_ = d.device.RemoveConnectionStatusChanged(d.connectionStatusListenerToken)

	if err := d.device.Close(); err != nil {
		return err
	}

	return nil
}

// Connected returns whether the device is currently connected.
func (d Device) Connected() (bool, error) {
	if d.device == nil {
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
// Windows uses only ConnectionParams.Priority. It ignores MinInterval,
// MaxInterval and Timeout. It needs Windows 11 build 22000 or later.
func (d Device) RequestConnectionParams(params ConnectionParams) error {
	if params.Priority == ConnectionPriorityUnspecified {
		return nil
	}
	if d.device == nil || d.connParams == nil {
		return errors.New("bluetooth: device is not connected")
	}

	preferred, err := preferredConnectionParameters(params.Priority)
	if err != nil {
		return err
	}
	defer preferred.Release()

	request, err := d.device.RequestPreferredConnectionParameters(preferred)
	if err != nil {
		return err
	}

	// The status shows only that the system accepted the request. The agreed
	// parameters come later, in the ConnectionParametersChanged event.
	status, err := request.GetStatus()
	if err != nil {
		discardConnectionParamsRequest(request)
		return err
	}
	if status != bluetooth.BluetoothLEPreferredConnectionParametersRequestStatusSuccess {
		discardConnectionParamsRequest(request)
		return fmt.Errorf("bluetooth: connection parameters request was not accepted: %s",
			connectionParamsRequestStatusString(status))
	}

	// Keep the request. Windows applies it only while the object is open.
	d.connParams.replace(request)

	return nil
}

// preferredConnectionParameters returns the Windows preset for the priority.
// The caller must release the result.
func preferredConnectionParameters(priority ConnectionPriority) (*bluetooth.BluetoothLEPreferredConnectionParameters, error) {
	var (
		preferred *bluetooth.BluetoothLEPreferredConnectionParameters
		err       error
	)
	switch priority {
	case ConnectionPriorityThroughput:
		preferred, err = bluetooth.BluetoothLEPreferredConnectionParametersGetThroughputOptimized()
	case ConnectionPriorityBalanced:
		preferred, err = bluetooth.BluetoothLEPreferredConnectionParametersGetBalanced()
	case ConnectionPriorityPowerSaving:
		preferred, err = bluetooth.BluetoothLEPreferredConnectionParametersGetPowerOptimized()
	default:
		return nil, fmt.Errorf("bluetooth: unknown connection priority %d", priority)
	}
	if err != nil {
		// Windows 10 does not have the activation factory for these presets.
		return nil, fmt.Errorf("bluetooth: connection priorities need Windows 11 build 22000 or later: %w", err)
	}
	return preferred, nil
}

// discardConnectionParamsRequest closes a request that the device does not keep.
func discardConnectionParamsRequest(request *bluetooth.BluetoothLEPreferredConnectionParametersRequest) {
	_ = request.Close()
	request.Release()
}

func connectionParamsRequestStatusString(status bluetooth.BluetoothLEPreferredConnectionParametersRequestStatus) string {
	switch status {
	case bluetooth.BluetoothLEPreferredConnectionParametersRequestStatusSuccess:
		return "success"
	case bluetooth.BluetoothLEPreferredConnectionParametersRequestStatusDeviceNotAvailable:
		return "device not available"
	case bluetooth.BluetoothLEPreferredConnectionParametersRequestStatusAccessDenied:
		return "access denied"
	default:
		return "unspecified"
	}
}

// SetRandomAddress sets the random address to be used for advertising.
func (a *Adapter) SetRandomAddress(mac MAC) error {
	return errors.ErrUnsupported
}
