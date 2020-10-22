package bluetooth

import (
	"errors"
	"fmt"
	"time"

	"github.com/JuulLabs-OSS/cbgo"
)

// Address contains a Bluetooth address which on macOS is a UUID.
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
func (ad Address) Set(val string) {
	uuid, err := ParseUUID(val)
	if err != nil {
		return
	}
	ad.UUID = uuid
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
	delegate *peripheralDelegate

	cm   cbgo.CentralManager
	prph cbgo.Peripheral

	servicesChan chan error
	charsChan    chan error

	services        map[UUID]*DeviceService
	characteristics map[UUID]*DeviceCharacteristic
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

		d.delegate = &peripheralDelegate{d: d}
		p.SetDelegate(d.delegate)

		a.connectHandler(nil, true)

		return d, nil
	case <-time.NewTimer(10 * time.Second).C:
		return nil, errors.New("timeout on Connect")
	}
}

// Disconnect from the BLE device. This method is non-blocking and does not
// wait until the connection is fully gone.
func (d *Device) Disconnect() error {
	d.cm.CancelConnect(d.prph)
	return nil
}

// Peripheral delegate functions

type peripheralDelegate struct {
	cbgo.PeripheralDelegateBase

	d *Device
}

// DidDiscoverServices is called when the services for a Peripheral
// have been discovered.
func (pd *peripheralDelegate) DidDiscoverServices(prph cbgo.Peripheral, err error) {
	pd.d.servicesChan <- nil
}

// DidDiscoverCharacteristics is called when the characteristics for a Service
// for a Peripheral have been discovered.
func (pd *peripheralDelegate) DidDiscoverCharacteristics(prph cbgo.Peripheral, svc cbgo.Service, err error) {
	pd.d.charsChan <- nil
}

// DidUpdateValueForCharacteristic is called when the characteristic for a Service
// for a Peripheral receives a notification with a new value,
// or receives a value for a read request.
func (pd *peripheralDelegate) DidUpdateValueForCharacteristic(prph cbgo.Peripheral, chr cbgo.Characteristic, err error) {
	uuid, _ := ParseUUID(chr.UUID().String())
	if char, ok := pd.d.characteristics[uuid]; ok {
		if err == nil && char.callback != nil {
			go char.callback(chr.Value())
		}

		if char.readChan != nil {
			char.readChan <- err
		}
	}
}
