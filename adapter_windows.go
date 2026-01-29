package bluetooth

import (
	"errors"
	"fmt"

	"github.com/go-ole/go-ole"
	"github.com/saltosystems/winrt-go"
	"github.com/saltosystems/winrt-go/windows/devices/bluetooth/advertisement"
	"github.com/saltosystems/winrt-go/windows/devices/bluetooth/genericattributeprofile"
	"github.com/saltosystems/winrt-go/windows/foundation"
)

type Adapter struct {
	watcher *advertisement.BluetoothLEAdvertisementWatcher

	connectHandler func(device Device, connected bool)

	defaultAdvertisement *Advertisement

	gattServiceProvider *genericattributeprofile.GattServiceProvider
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
		return fmt.Errorf("async operation failed with status %d", status)
	}
	return nil
}

func (a *Adapter) Address() (MACAddress, error) {
	// TODO: get mac address
	return MACAddress{}, errors.New("not implemented")
}
