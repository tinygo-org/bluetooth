package bluetooth

import (
	"errors"

	"github.com/JuulLabs-OSS/cbgo"
)

// Address contains a Bluetooth address, which is a MAC address plus some extra
// information.
type Address struct {
	// UUID if this is macOS.
	UUID

	isRandom bool
}

// IsRandom if the address is randomly created.
func (ad Address) IsRandom() bool {
	return ad.isRandom
}

// SetRandom if is a random address.
func (ad Address) SetRandom(val bool) {
	ad.isRandom = val
}

// Set the address
func (ad Address) Set(val interface{}) {
	ad.UUID = val.(UUID)
}

// Scan starts a BLE scan. It is stopped by a call to StopScan. A common pattern
// is to cancel the scan when a particular device has been found.
func (a *Adapter) Scan(callback func(*Adapter, ScanResult)) (err error) {
	if callback == nil {
		return errors.New("must provide callback to Scan function")
	}

	if a.cancelChan != nil {
		return errors.New("already calling Scan function")
	}

	a.peripheralFoundHandler = callback

	// Channel that will be closed when the scan is stopped.
	// Detecting whether the scan is stopped can be done by doing a non-blocking
	// read from it. If it succeeds, the scan is stopped.
	cancelChan := make(chan struct{})
	a.cancelChan = cancelChan

	a.cm.Scan(nil, &cbgo.CentralManagerScanOpts{
		AllowDuplicates: true,
	})

	for {
		// Check whether the scan is stopped. This is necessary to avoid a race
		// condition between the signal channel and the cancelScan channel when
		// the callback calls StopScan() (no new callbacks may be called after
		// StopScan is called).
		select {
		case <-cancelChan:
			// stop scanning here?
			return nil
		default:
		}
	}
}

// StopScan stops any in-progress scan. It can be called from within a Scan
// callback to stop the current scan. If no scan is in progress, an error will
// be returned.
func (a *Adapter) StopScan() error {
	if a.cancelChan != nil {
		return errors.New("already calling Scan function")
	}

	a.cm.StopScan()
	close(a.cancelChan)
	a.cancelChan = nil

	return nil
}
