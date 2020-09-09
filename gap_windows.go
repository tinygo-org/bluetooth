package bluetooth

import (
	"tinygo.org/x/bluetooth/winbt"
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

	a.watcher, err = winbt.NewBluetoothLEAdvertisementWatcher()
	if err != nil {
		return
	}
	defer a.watcher.Release()

	// Listen for incoming BLE advertisement packets.
	err = a.watcher.AddReceivedEvent(func(watcher *winbt.IBluetoothLEAdvertisementWatcher, args *winbt.IBluetoothLEAdvertisementReceivedEventArgs) {
		var result ScanResult
		result.RSSI = args.RawSignalStrengthInDBm()
		addr := args.BluetoothAddress()
		adr := result.Address.(Address)
		for i := range adr.MAC {
			adr.MAC[i] = byte(addr)
			addr >>= 8
		}
		// Note: the IsRandom bit is never set.
		advertisement := args.Advertisement()
		result.AdvertisementPayload = &advertisementFields{
			AdvertisementFields{
				LocalName: advertisement.LocalName(),
			},
		}
		callback(a, result)
	})
	if err != nil {
		return
	}

	// Wait for when advertisement has stopped by a call to StopScan().
	// Advertisement doesn't seem to stop right away, there is an
	// intermediate Stopping state.
	stoppingChan := make(chan struct{})
	err = a.watcher.AddStoppedEvent(func(watcher *winbt.IBluetoothLEAdvertisementWatcher, args *winbt.IBluetoothLEAdvertisementWatcherStoppedEventArgs) {
		// Note: the args parameter has an Error property that should
		// probably be checked, but I'm not sure when stopping the
		// advertisement watcher could ever result in an error (except
		// for bugs).
		close(stoppingChan)
	})
	if err != nil {
		return
	}

	err = a.watcher.Start()
	if err != nil {
		return err
	}

	// Wait until advertisement has stopped, and finish.
	<-stoppingChan
	a.watcher = nil
	return nil
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
