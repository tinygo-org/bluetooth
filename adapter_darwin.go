package bluetooth

import (
	"errors"
	"time"

	"github.com/JuulLabs-OSS/cbgo"
)

// Adapter is a connection to BLE devices.
type Adapter struct {
	cbgo.CentralManagerDelegateBase
	cbgo.PeripheralManagerDelegateBase

	cm cbgo.CentralManager
	pm cbgo.PeripheralManager

	peripheralFoundHandler func(*Adapter, ScanResult)
	scanChan               chan error
	poweredChan            chan error
	connectChan            chan cbgo.Peripheral
}

// DefaultAdapter is the default adapter on the system.
//
// Make sure to call Enable() before using it to initialize the adapter.
var DefaultAdapter = &Adapter{
	cm:          cbgo.NewCentralManager(nil),
	pm:          cbgo.NewPeripheralManager(nil),
	connectChan: make(chan cbgo.Peripheral),
}

// Enable configures the BLE stack. It must be called before any
// Bluetooth-related calls (unless otherwise indicated).
func (a *Adapter) Enable() error {
	if a.poweredChan != nil {
		return errors.New("already calling Enable function")
	}

	// wait until powered
	a.poweredChan = make(chan error)
	a.cm.SetDelegate(a)
	select {
	case <-a.poweredChan:
	case <-time.NewTimer(10 * time.Second).C:
		return errors.New("timeout enabling CentralManager")
	}
	a.poweredChan = nil

	// wait until powered?
	//a.pm.SetDelegate(a)

	return nil
}

// CentralManager delegate functions

// CentralManagerDidUpdateState when central manager state updated.
func (a *Adapter) CentralManagerDidUpdateState(cmgr cbgo.CentralManager) {
	// powered on?
	if cmgr.State() == cbgo.ManagerStatePoweredOn {
		close(a.poweredChan)
	}

	// TODO: handle other state changes.
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
	// Unblock now that we're connected.
	a.connectChan <- prph
}

// makeScanResult creates a ScanResult when peripheral is found.
func makeScanResult(prph cbgo.Peripheral, advFields cbgo.AdvFields, rssi int) ScanResult {
	uuid, _ := ParseUUID(prph.Identifier().String())

	var serviceUUIDs []UUID
	for _, u := range advFields.ServiceUUIDs {
		parsedUUID, _ := ParseUUID(u.String())
		serviceUUIDs = append(serviceUUIDs, parsedUUID)
	}

	// It is never a random address on macOS.
	return ScanResult{
		RSSI: int16(rssi),
		Address: Address{
			UUID: uuid,
		},
		AdvertisementPayload: &advertisementFields{
			AdvertisementFields{
				LocalName:    advFields.LocalName,
				ServiceUUIDs: serviceUUIDs,
			},
		},
	}
}
