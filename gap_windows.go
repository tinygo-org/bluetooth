package bluetooth

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/go-ole/go-ole"
	"github.com/saltosystems/winrt-go"
	"github.com/saltosystems/winrt-go/windows/devices/bluetooth"
	"github.com/saltosystems/winrt-go/windows/devices/bluetooth/advertisement"
	"github.com/saltosystems/winrt-go/windows/devices/bluetooth/genericattributeprofile"
	"github.com/saltosystems/winrt-go/windows/foundation"
	"github.com/saltosystems/winrt-go/windows/foundation/collections"
	"github.com/saltosystems/winrt-go/windows/storage/streams"
)

const defaultConnectionTimeout time.Duration = 10 * time.Second

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
	var seen scanCache
	handler := foundation.NewTypedEventHandler(ole.NewGUID(eventReceivedGuid), func(instance *foundation.TypedEventHandler, sender, arg unsafe.Pointer) {
		args := (*advertisement.BluetoothLEAdvertisementReceivedEventArgs)(arg)
		result := getScanResultFromArgs(args)
		// WinRT reports a scan response as its own event, so fill in what a packet lacks from earlier ones.
		// See https://learn.microsoft.com/en-us/uwp/api/windows.devices.bluetooth.advertisement.bluetoothleadvertisementtype
		if p, ok := result.AdvertisementPayload.(*advertisementFields); ok {
			p.AdvertisementFields = seen.merge(result.Address, p.AdvertisementFields)
		}
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
	var serviceUUIDs []UUID

	// Extract manufacturer data
	manDataVector, _ := winAdv.GetManufacturerData()
	if manDataVector != nil {
		defer manDataVector.Release()
		size, _ := manDataVector.GetSize()
		for i := uint32(0); i < size; i++ {
			element, _ := manDataVector.GetAt(i)
			manData := (*advertisement.BluetoothLEManufacturerData)(element)

			companyID, _ := manData.GetCompanyId()
			buffer, _ := manData.GetData()
			if buffer != nil {
				manufacturerData = append(manufacturerData, ManufacturerDataElement{
					CompanyID: companyID,
					Data:      bufferToSlice(buffer),
				})
				buffer.Release()
			}
			manData.Release()
		}
	}

	// Extract service UUIDs
	serviceUuidsVector, _ := winAdv.GetServiceUuids()
	if serviceUuidsVector != nil {
		defer serviceUuidsVector.Release()
		size, _ := serviceUuidsVector.GetSize()
		for i := uint32(0); i < size; i++ {
			serviceGUID, err := guidVectorAt(serviceUuidsVector, i)
			if err != nil {
				continue
			}
			serviceUUIDs = append(serviceUUIDs, GUIDToUUID(serviceGUID))
		}
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

// scanCacheSize limits how many devices a scan keeps fields for.
// When it is full, the device seen longest ago is dropped.
const scanCacheSize = 256

// scanCache keeps the advertisement fields each device has sent during a scan.
type scanCache struct {
	mu      sync.Mutex
	tick    uint64
	devices map[Address]*scanCacheEntry
}

type scanCacheEntry struct {
	fields   AdvertisementFields
	lastSeen uint64
}

// merge fills in what fields lack from the fields addr sent before, keeps the
// result, and returns a copy of it that shares no memory with the cache.
func (c *scanCache) merge(addr Address, fields AdvertisementFields) AdvertisementFields {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.tick++
	e, ok := c.devices[addr]
	if !ok {
		if c.devices == nil {
			c.devices = make(map[Address]*scanCacheEntry)
		}
		if len(c.devices) >= scanCacheSize {
			c.dropOldest()
		}
		e = &scanCacheEntry{}
		c.devices[addr] = e
	}
	e.fields = mergeAdvertisementFields(e.fields, fields)
	e.lastSeen = c.tick
	return cloneAdvertisementFields(e.fields)
}

func (c *scanCache) dropOldest() {
	var oldest Address
	var oldestSeen uint64
	for addr, e := range c.devices {
		if oldestSeen == 0 || e.lastSeen < oldestSeen {
			oldest, oldestSeen = addr, e.lastSeen
		}
	}
	delete(c.devices, oldest)
}

// mergeAdvertisementFields merges cur into last, with lists merged by key like BlueZ.
// See https://github.com/bluez/bluez/blob/fa6ba55020dee3c3183041c854c347b4582448be/src/device.c#L2497-L2567
func mergeAdvertisementFields(last, cur AdvertisementFields) AdvertisementFields {
	if cur.LocalName == "" {
		cur.LocalName = last.LocalName
	}
	cur.ServiceUUIDs = mergeByKey(last.ServiceUUIDs, cur.ServiceUUIDs, func(u UUID) UUID { return u })
	cur.ManufacturerData = mergeByKey(last.ManufacturerData, cur.ManufacturerData, func(m ManufacturerDataElement) uint16 { return m.CompanyID })
	cur.ServiceData = mergeByKey(last.ServiceData, cur.ServiceData, func(s ServiceDataElement) UUID { return s.UUID })
	return cur
}

// mergeByKey returns a copy of last where each element of cur replaces the one with
// the same key in place, or is appended, so the order stays stable.
func mergeByKey[E any, K comparable](last, cur []E, key func(E) K) []E {
	merged := slices.Clone(last)
	for _, c := range cur {
		if i := slices.IndexFunc(merged, func(m E) bool { return key(m) == key(c) }); i >= 0 {
			merged[i] = c
		} else {
			merged = append(merged, c)
		}
	}
	return merged
}

func cloneAdvertisementFields(f AdvertisementFields) AdvertisementFields {
	f.ServiceUUIDs = slices.Clone(f.ServiceUUIDs)
	f.ManufacturerData = slices.Clone(f.ManufacturerData)
	for i := range f.ManufacturerData {
		f.ManufacturerData[i].Data = slices.Clone(f.ManufacturerData[i].Data)
	}
	f.ServiceData = slices.Clone(f.ServiceData)
	for i := range f.ServiceData {
		f.ServiceData[i].Data = slices.Clone(f.ServiceData[i].Data)
	}
	return f
}

// IVector.GetAt has room for a pointer, but GetAt on an IVector<GUID> writes a 16 byte GUID.
// See https://learn.microsoft.com/en-us/uwp/api/windows.foundation.collections.ivector-1.getat
func guidVectorAt(v *collections.IVector, i uint32) (syscall.GUID, error) {
	var guid syscall.GUID
	hr, _, _ := syscall.SyscallN(
		v.VTable().GetAt,
		uintptr(unsafe.Pointer(v)),
		uintptr(i),
		uintptr(unsafe.Pointer(&guid)),
	)
	if hr != 0 {
		return syscall.GUID{}, ole.NewError(hr)
	}
	return guid, nil
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
	services                      *openServices

	// pairing holds a pairing request waiting for EnterPasskey or
	// ConfirmPasskey. It is shared by every copy of this Device.
	pairing *pairingRequest
}

var errDeviceNotConnected = errors.New("bluetooth: device is not connected")

// connectionParamsState keeps the open connection parameters request.
// Windows applies the request only while the object is open:
// https://learn.microsoft.com/en-us/uwp/api/windows.devices.bluetooth.bluetoothledevice.requestpreferredconnectionparameters
type connectionParamsState struct {
	mu      sync.Mutex
	request *bluetooth.BluetoothLEPreferredConnectionParametersRequest
	closed  bool
}

// close closes the open request, which restores the system defaults. A request
// after this gets an error.
func (s *connectionParamsState) close() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.closed = true
	if s.request != nil {
		discardConnectionParamsRequest(s.request)
		s.request = nil
	}
}

// openServices are the services DiscoverServices returned, which Disconnect closes.
type openServices struct {
	mu   sync.Mutex
	list []*genericattributeprofile.GattDeviceService
}

func (s *openServices) add(srv *genericattributeprofile.GattDeviceService) {
	s.mu.Lock()
	if !slices.Contains(s.list, srv) {
		s.list = append(s.list, srv)
	}
	s.mu.Unlock()
}

func (s *openServices) close() {
	s.mu.Lock()
	list := s.list
	s.list = nil
	s.mu.Unlock()
	for _, srv := range list {
		_ = srv.Close()
	}
}

// Connect starts a connection attempt to the given peripheral device address.
//
// On Linux and Windows, the IsRandom part of the address is ignored.
//
// Windows requests params.Priority after the connection, and ignores an error.
// Call RequestConnectionParams to see the error.
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
		return Device{}, err
	}
	defer dID.Release()

	// Windows does not support explicitly connecting to a device.
	// Instead it has the concept of a GATT session that is owned
	// by the calling program.
	gattSessionOp, err := genericattributeprofile.GattSessionFromDeviceIdAsync(dID) // IAsyncOperation<GattSession>
	if err != nil {
		return Device{}, err
	}
	defer gattSessionOp.Release()

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
		services:   &openServices{},

		pairing: &pairingRequest{},
	}

	// https://learn.microsoft.com/es-es/uwp/api/windows.devices.bluetooth.bluetoothledevice.connectionstatuschanged?view=winrt-26100
	// TypedEventHandler<BluetoothLEDevice,object>
	connectionStatusChangedGUID := winrt.ParameterizedInstanceGUID(
		foundation.GUIDTypedEventHandler,
		bluetooth.SignatureBluetoothLEDevice,
		"cinterface(IInspectable)", // object
	)

	up := make(chan struct{}, 1)
	handler := foundation.NewTypedEventHandler(ole.NewGUID(connectionStatusChangedGUID), func(instance *foundation.TypedEventHandler, sender, arg unsafe.Pointer) {
		status, err := bleDevice.GetConnectionStatus()
		if err != nil {
			return
		}

		if status == bluetooth.BluetoothConnectionStatusConnected {
			select {
			case up <- struct{}{}:
			default:
			}
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

	// MaintainConnection only starts connecting, so wait like the other backends do.
	// See https://learn.microsoft.com/en-us/uwp/api/windows.devices.bluetooth.genericattributeprofile.gattsession.maintainconnection
	timeout := defaultConnectionTimeout
	if params.ConnectionTimeout != 0 {
		timeout = time.Duration(int64(params.ConnectionTimeout)*625) * time.Microsecond
	}
	if err := waitConnected(up, device.Connected, timeout); err != nil {
		_ = device.Disconnect()
		return Device{}, err
	}

	// WinRT has no priority at connection time. The connection stays if this
	// fails, as on the platforms that have no priority.
	if params.Priority != ConnectionPriorityUnspecified {
		_ = device.RequestConnectionParams(params)
	}

	return device, nil
}

// waitConnected returns once connected reports true or up receives, or fails after timeout.
func waitConnected(up <-chan struct{}, connected func() (bool, error), timeout time.Duration) error {
	if ok, err := connected(); err == nil && ok {
		return nil
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-up:
		return nil
	case <-timer.C:
		return errors.New("timeout on Connect")
	}
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
	d.services.close()

	// Close the request while the device is still open.
	if d.connParams != nil {
		d.connParams.close()
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
		return errDeviceNotConnected
	}

	// Disconnect waits for the lock, so the device stays open during the request.
	d.connParams.mu.Lock()
	defer d.connParams.mu.Unlock()
	if d.connParams.closed {
		return errDeviceNotConnected
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

	// Keep the request and close the previous one. Windows applies a request
	// only while the object is open.
	if d.connParams.request != nil {
		discardConnectionParamsRequest(d.connParams.request)
	}
	d.connParams.request = request

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
	if isClassNotRegistered(err) {
		// Windows 10 does not have the activation factory for these presets.
		return nil, fmt.Errorf("bluetooth: connection priorities need Windows 11 build 22000 or later: %w", err)
	}
	if err != nil {
		return nil, err
	}
	return preferred, nil
}

// isClassNotRegistered reports whether err is REGDB_E_CLASSNOTREG (0x80040154).
// https://learn.microsoft.com/en-us/windows/win32/com/com-error-codes-1
func isClassNotRegistered(err error) bool {
	var oleErr *ole.OleError
	return errors.As(err, &oleErr) && uint32(oleErr.Code()) == 0x80040154
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
	case bluetooth.BluetoothLEPreferredConnectionParametersRequestStatusUnspecified:
		return "unspecified"
	default:
		return fmt.Sprintf("BluetoothLEPreferredConnectionParametersRequestStatus(%d)", status)
	}
}

// SetRandomAddress sets the random address to be used for advertising.
func (a *Adapter) SetRandomAddress(mac MAC) error {
	return errors.ErrUnsupported
}

// SetTxPower sets the radio transmit power for advertising, in dBm.
// Windows does not give this control, so it is not supported.
func (a *Advertisement) SetTxPower(dbm int8) error {
	return errNotSupported
}
