package bluetooth

import (
	"github.com/JuulLabs-OSS/cbgo"
)

type Adapter struct {
	cbgo.CentralManagerDelegateBase
	cbgo.PeripheralManagerDelegateBase

	cm cbgo.CentralManager
	pm cbgo.PeripheralManager

	peripheralFoundHandler func(*Adapter, ScanResult)
	cancelChan             chan struct{}
}

// DefaultAdapter is the default adapter on the system.
//
// Make sure to call Enable() before using it to initialize the adapter.
var DefaultAdapter = &Adapter{
	cm: cbgo.NewCentralManager(nil),
	pm: cbgo.NewPeripheralManager(nil),
}

// Enable configures the BLE stack. It must be called before any
// Bluetooth-related calls (unless otherwise indicated).
func (a *Adapter) Enable() error {
	a.cm.SetDelegate(a)
	// TODO: wait until powered
	a.pm.SetDelegate(a)
	// TODO: wait until powered

	return nil
}

// CentralManager delegate functions

func (a *Adapter) CentralManagerDidUpdateState(cmgr cbgo.CentralManager) {
}

// DidDiscoverPeripheral when peripheral is discovered.
func (a *Adapter) DidDiscoverPeripheral(cmgr cbgo.CentralManager, prph cbgo.Peripheral,
	advFields cbgo.AdvFields, rssi int) {
	if a.peripheralFoundHandler != nil {
		sr := makeScanResult(prph, advFields, rssi)
		a.peripheralFoundHandler(a, sr)
	}
}

// DidConnectPeripheral when peripheral is connected.
func (a *Adapter) DidConnectPeripheral(cmgr cbgo.CentralManager, prph cbgo.Peripheral) {
}

// DidDisconnectPeripheral when peripheral is disconnected.
func (a *Adapter) DidDisconnectPeripheral(cmgr cbgo.CentralManager, prph cbgo.Peripheral, err error) {
}

// PeripheralManager delegate functions

// PeripheralManagerDidUpdateState when state updated.
func (a *Adapter) PeripheralManagerDidUpdateState(pmgr cbgo.PeripheralManager) {
}

// DidAddService when service added.
func (a *Adapter) DidAddService(pmgr cbgo.PeripheralManager, svc cbgo.Service, err error) {
}

// DidStartAdvertising when advertising starts.
func (a *Adapter) DidStartAdvertising(pmgr cbgo.PeripheralManager, err error) {
}

// DidReceiveReadRequest when read request received.
func (a *Adapter) DidReceiveReadRequest(pmgr cbgo.PeripheralManager, cbreq cbgo.ATTRequest) {
}

// DidReceiveWriteRequests when write requests received.
func (a *Adapter) DidReceiveWriteRequests(pmgr cbgo.PeripheralManager, cbreqs []cbgo.ATTRequest) {
}

// CentralDidSubscribe when central subscribed.
func (a *Adapter) CentralDidSubscribe(pmgr cbgo.PeripheralManager, cent cbgo.Central, cbchr cbgo.Characteristic) {
}

// CentralDidUnsubscribe when central unsubscribed.
func (a *Adapter) CentralDidUnsubscribe(pmgr cbgo.PeripheralManager, cent cbgo.Central, chr cbgo.Characteristic) {
}

// makeScanResult creates a ScanResult when peripheral is found.
func makeScanResult(prph cbgo.Peripheral, advFields cbgo.AdvFields, rssi int) ScanResult {
	// TODO: figure out the peripheral info.

	// TODO: create a list of serviceUUIDs.

	return ScanResult{
		RSSI:    int16(rssi),
		Address: Address{
			// TODO: fill in this info
			//MAC:      prph.Identifier(),
			//IsRandom: prph.Identifier == "random",
		},
		AdvertisementPayload: &advertisementFields{
			AdvertisementFields{
				LocalName: advFields.LocalName,
				// TODO: fill in this info
				//ServiceUUIDs: serviceUUIDs,
			},
		},
	}
}
