package bluetooth

import (
	"errors"
	"fmt"
	"time"

	"github.com/tinygo-org/cbgo"
)

// default connection timeout
const defaultConnectionTimeout time.Duration = 10 * time.Second

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
func (ad *Address) SetRandom(val bool) {
}

// Set the address
func (ad *Address) Set(val string) {
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

	services map[UUID]DeviceService
}

// Connect starts a connection attempt to the given peripheral device address.
func (a *Adapter) Connect(address Address, params ConnectionParams) (*Device, error) {
	uuid, err := cbgo.ParseUUID(address.UUID.String())
	if err != nil {
		return nil, err
	}
	prphs := a.cm.RetrievePeripheralsWithIdentifiers([]cbgo.UUID{uuid})
	if len(prphs) == 0 {
		return nil, fmt.Errorf("Connect failed: no peer with address: %s", address.UUID.String())
	}

	timeout := defaultConnectionTimeout
	if params.ConnectionTimeout != 0 {
		timeout = time.Duration(int64(params.ConnectionTimeout)*625) * time.Microsecond
	}

	id := prphs[0].Identifier().String()
	prphCh := make(chan cbgo.Peripheral)

	a.connectMap.Store(id, prphCh)
	defer a.connectMap.Delete(id)

	a.cm.Connect(prphs[0], nil)
	timeoutTimer := time.NewTimer(timeout)
	var connectionError error

	for {
		// wait on channel for connect
		select {
		case p := <-prphCh:

			// check if we have received a disconnected peripheral
			if p.State() == cbgo.PeripheralStateDisconnected {
				return nil, connectionError
			}

			d := &Device{
				cm:           a.cm,
				prph:         p,
				servicesChan: make(chan error),
				charsChan:    make(chan error),
			}

			d.delegate = &peripheralDelegate{d: d}
			p.SetDelegate(d.delegate)

			a.connectHandler(address, true)

			return d, nil

		case <-timeoutTimer.C:
			// we need to cancel the connection if we have timed out ourselves
			a.cm.CancelConnect(prphs[0])

			// record an error to use when the disconnect comes through later.
			connectionError = errors.New("timeout on Connect")

			// we are not ready to return yet, we need to wait for the disconnect event to come through
			// so continue on from this case and wait for something to show up on prphCh
			continue
		}
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
	svcuuid, _ := ParseUUID(chr.Service().UUID().String())

	if svc, ok := pd.d.services[svcuuid]; ok {
		for _, char := range svc.characteristics {

			if char.characteristic == chr && uuid == char.UUID() { // compare pointers
				if err == nil && char.callback != nil {
					go char.callback(chr.Value())
				}

				if char.readChan != nil {
					char.readChan <- err
				}
			}

		}

	}
}

// DidWriteValueForCharacteristic is called after the characteristic for a Service
// for a Peripheral trigger a write with response. It contains the returned error or nil.
func (pd *peripheralDelegate) DidWriteValueForCharacteristic(_ cbgo.Peripheral, chr cbgo.Characteristic, err error) {
	uuid, _ := ParseUUID(chr.UUID().String())
	svcuuid, _ := ParseUUID(chr.Service().UUID().String())

	if svc, ok := pd.d.services[svcuuid]; ok {
		for _, char := range svc.characteristics {
			if char.characteristic == chr && uuid == char.UUID() { // compare pointers
				if char.writeChan != nil {
					char.writeChan <- err
				}
			}
		}
	}
}
