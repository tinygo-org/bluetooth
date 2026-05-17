package bluetooth

import (
	"errors"
	"fmt"
	"syscall"
	"unsafe"

	"github.com/go-ole/go-ole"
	"github.com/saltosystems/winrt-go"
	"github.com/saltosystems/winrt-go/windows/devices/bluetooth/advertisement"
	"github.com/saltosystems/winrt-go/windows/foundation"
)

var _ BLEAdapter = (*Adapter)(nil)

type Adapter struct {
	watcher *advertisement.BluetoothLEAdvertisementWatcher

	connectHandler func(device Device, connected bool)

	defaultAdvertisement *Advertisement
}

// DefaultAdapter is the default adapter on the system.
//
// Make sure to call Enable() before using it to initialize the adapter.
var DefaultAdapter = &Adapter{
	connectHandler: func(device Device, connected bool) {
		return
	},
}

// Enable configures the BLE stack. It must be called before any
// Bluetooth-related calls (unless otherwise indicated).
func (a *Adapter) Enable() error {
	return ole.RoInitialize(1) // initialize with multithreading enabled
}

// Reset is a no-op on Windows. Provided for interface symmetry.
func (a *Adapter) Reset() error {
	return nil
}

func awaitAsyncOperation(asyncOperation *foundation.IAsyncOperation, genericParamSignature string) error {
	var status foundation.AsyncStatus

	// We need to obtain the GUID of the AsyncOperationCompletedHandler, but its a generic delegate
	// so we also need the generic parameter type's signature:
	// AsyncOperationCompletedHandler<genericParamSignature>
	iid := winrt.ParameterizedInstanceGUID(foundation.GUIDAsyncOperationCompletedHandler, genericParamSignature)

	// Wait until the async operation completes.
	waitChan := make(chan struct{})
	handler := foundation.NewAsyncOperationCompletedHandler(ole.NewGUID(iid), func(instance *foundation.AsyncOperationCompletedHandler, asyncInfo *foundation.IAsyncOperation, asyncStatus foundation.AsyncStatus) {
		status = asyncStatus
		close(waitChan)
	})
	defer handler.Release()

	asyncOperation.SetCompleted(handler)

	// Wait until async operation has stopped, and finish.
	<-waitChan

	if status != foundation.AsyncStatusCompleted {
		if err := getAsyncError(asyncOperation); err != nil {
			return fmt.Errorf("async operation failed with status %d: %w", status, err)
		}
		return fmt.Errorf("async operation failed with status %d", status)
	}
	return nil
}

// getAsyncError queries IAsyncInfo from an IAsyncOperation to retrieve
// the error code of a failed async operation. If the HRESULT corresponds
// to a Bluetooth ATT error (facility 0x65), it returns an AttributeProtocolError.
func getAsyncError(asyncOperation *foundation.IAsyncOperation) error {
	iid := ole.NewGUID(foundation.GUIDIAsyncInfo)

	var asyncInfo *foundation.IAsyncInfo
	hr, _, _ := syscall.SyscallN(
		asyncOperation.VTable().QueryInterface,
		uintptr(unsafe.Pointer(asyncOperation)),
		uintptr(unsafe.Pointer(iid)),
		uintptr(unsafe.Pointer(&asyncInfo)),
	)
	if hr != 0 {
		return nil
	}
	defer asyncInfo.Release()

	result, err := asyncInfo.GetErrorCode()
	if err != nil {
		return err
	}
	if result.Value == 0 {
		return nil
	}

	return hresultToError(uint32(result.Value))
}

// hresultToError converts an HRESULT to an appropriate error type.
// For Bluetooth ATT errors (facility 0x65), it returns an error with the ATT error code.
// For other errors, it returns a generic error with the HRESULT value.
func hresultToError(hr uint32) error {
	facility := (hr >> 16) & 0x1FFF
	code := hr & 0xFFFF

	if facility == 0x65 { // FACILITY_BLUETOOTH_ATT
		return fmt.Errorf("Bluetooth ATT error (code 0x%04X)", code)
	}

	return fmt.Errorf("HRESULT 0x%08X", hr)
}

func (a *Adapter) Address() (MACAddress, error) {
	// TODO: get mac address
	return MACAddress{}, errors.New("not implemented")
}
