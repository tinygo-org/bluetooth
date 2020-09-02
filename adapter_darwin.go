package bluetooth

import (
	"errors"
	"time"

	"github.com/JuulLabs-OSS/cbgo"
)

// Adapter is a connection to BLE devices.
type Adapter struct {
	cmd *centralManagerDelegate
	pmd *peripheralManagerDelegate

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

	a.cmd = &centralManagerDelegate{a: a}
	a.cm.SetDelegate(a.cmd)
	select {
	case <-a.poweredChan:
	case <-time.NewTimer(10 * time.Second).C:
		return errors.New("timeout enabling CentralManager")
	}
	a.poweredChan = nil

	// wait until powered?
	a.pmd = &peripheralManagerDelegate{a: a}
	a.pm.SetDelegate(a.pmd)

	return nil
}

// CentralManager delegate functions

type centralManagerDelegate struct {
	cbgo.CentralManagerDelegateBase

	a *Adapter
}

// CentralManagerDidUpdateState when central manager state updated.
func (cmd *centralManagerDelegate) CentralManagerDidUpdateState(cmgr cbgo.CentralManager) {
	// powered on?
	if cmgr.State() == cbgo.ManagerStatePoweredOn {
		close(cmd.a.poweredChan)
	}

	// TODO: handle other state changes.
}

// DidDiscoverPeripheral when peripheral is discovered.
func (cmd *centralManagerDelegate) DidDiscoverPeripheral(cmgr cbgo.CentralManager, prph cbgo.Peripheral,
	advFields cbgo.AdvFields, rssi int) {
	if cmd.a.peripheralFoundHandler != nil {
		sr := makeScanResult(prph, advFields, rssi)
		cmd.a.peripheralFoundHandler(cmd.a, sr)
	}
}

// DidConnectPeripheral when peripheral is connected.
func (cmd *centralManagerDelegate) DidConnectPeripheral(cmgr cbgo.CentralManager, prph cbgo.Peripheral) {
	// Unblock now that we're connected.
	cmd.a.connectChan <- prph
}

// makeScanResult creates a ScanResult when peripheral is found.
func makeScanResult(prph cbgo.Peripheral, advFields cbgo.AdvFields, rssi int) ScanResult {
	uuid, _ := ParseUUID(prph.Identifier().String())

	var serviceUUIDs []UUID
	for _, u := range advFields.ServiceUUIDs {
		parsedUUID, _ := ParseUUID(u.String())
		serviceUUIDs = append(serviceUUIDs, parsedUUID)
	}

	// Peripheral UUID is randomized on macOS, which means to
	// different centrals it will appear to have a different UUID.
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

// PeripheralManager delegate functions

type peripheralManagerDelegate struct {
	cbgo.PeripheralManagerDelegateBase

	a *Adapter
}
