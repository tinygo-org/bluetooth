package bluetooth

import (
	"errors"
	"fmt"
	"time"

	"github.com/JuulLabs-OSS/cbgo"
)

// Address contains a Bluetooth address, which on macOS instead of a MAC address
// is instead a UUID.
type Address struct {
	// UUID since this is macOS.
	UUID
}

// IsRandom ignored on macOS.
func (ad Address) IsRandom() bool {
	return false
}

// SetRandom ignored on macOS.
func (ad Address) SetRandom(val bool) {
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

	if a.scanChan != nil {
		return errors.New("already calling Scan function")
	}

	a.peripheralFoundHandler = callback

	// Channel that will be closed when the scan is stopped.
	// Detecting whether the scan is stopped can be done by doing a non-blocking
	// read from it. If it succeeds, the scan is stopped.
	a.scanChan = make(chan error)

	a.cm.Scan(nil, &cbgo.CentralManagerScanOpts{
		AllowDuplicates: false,
	})

	// Check whether the scan is stopped. This is necessary to avoid a race
	// condition between the signal channel and the cancelScan channel when
	// the callback calls StopScan() (no new callbacks may be called after
	// StopScan is called).
	select {
	case <-a.scanChan:
		close(a.scanChan)
		a.scanChan = nil
		return nil
	}
}

// StopScan stops any in-progress scan. It can be called from within a Scan
// callback to stop the current scan. If no scan is in progress, an error will
// be returned.
func (a *Adapter) StopScan() error {
	if a.scanChan == nil {
		return errors.New("not calling Scan function")
	}

	a.scanChan <- nil
	a.cm.StopScan()

	return nil
}

// Device is a connection to a remote peripheral.
type Device struct {
	cbgo.PeripheralDelegateBase

	cm   cbgo.CentralManager
	prph cbgo.Peripheral

	servicesChan chan error
	charsChan    chan error
}

// Connect starts a connection attempt to the given peripheral device address.
func (a *Adapter) Connect(address Addresser, params ConnectionParams) (*Device, error) {
	adr := address.(Address)
	uuid, err := cbgo.ParseUUID(adr.UUID.String())
	if err != nil {
		return nil, err
	}
	prphs := a.cm.RetrievePeripheralsWithIdentifiers([]cbgo.UUID{uuid})
	if len(prphs) == 0 {
		return nil, fmt.Errorf("Connect failed: no peer with address: %s", adr.UUID.String())
	}
	a.cm.Connect(prphs[0], nil)

	// wait on channel for connect
	select {
	case p := <-a.connectChan:
		d := &Device{
			cm:           a.cm,
			prph:         p,
			servicesChan: make(chan error),
			charsChan:    make(chan error),
		}

		p.SetDelegate(d)
		return d, nil
	case <-time.NewTimer(10 * time.Second).C:
		return nil, errors.New("timeout on Connect")
	}
}

// Peripheral returns the Device's cbgo.Peripheral
func (d *Device) Peripheral() (prph cbgo.Peripheral) {
	return d.prph
}

// CharsChan returns the Device's charsChan channel used for
// blocking on discovering the characteristics for a service.
func (d *Device) CharsChan() chan error {
	return d.charsChan
}

// Peripheral delegate functions

// DidDiscoverServices is called when the services for a Peripheral
// have been discovered.
func (d *Device) DidDiscoverServices(prph cbgo.Peripheral, err error) {
	d.servicesChan <- nil
}

// DidDiscoverCharacteristics is called when the characteristics for a Service
// for a Peripheral have been discovered.
func (d *Device) DidDiscoverCharacteristics(prph cbgo.Peripheral, svc cbgo.Service, err error) {
	d.charsChan <- nil
}
