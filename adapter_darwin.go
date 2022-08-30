package bluetooth

import (
	"errors"
	"sync"
	"time"

	"github.com/tinygo-org/cbgo"
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

	// connectMap is a mapping of peripheralId -> chan cbgo.Peripheral,
	// used to allow multiple callers to call Connect concurrently.
	connectMap sync.Map

	connectHandler func(device Addresser, connected bool)
}

// DefaultAdapter is the default adapter on the system.
//
// Make sure to call Enable() before using it to initialize the adapter.
var DefaultAdapter = &Adapter{
	cm:         cbgo.NewCentralManager(nil),
	pm:         cbgo.NewPeripheralManager(nil),
	connectMap: sync.Map{},

	connectHandler: func(device Addresser, connected bool) {
		return
	},
}

// Enable configures the BLE stack. It must be called before any
// Bluetooth-related calls (unless otherwise indicated).
func (a *Adapter) Enable() error {
	if a.poweredChan != nil {
		return errors.New("already calling Enable function")
	}

	// wait until powered
	a.poweredChan = make(chan error, 1)

	a.cmd = &centralManagerDelegate{a: a}
	a.cm.SetDelegate(a.cmd)

	if a.cm.State() != cbgo.ManagerStatePoweredOn {
		select {
		case <-a.poweredChan:
		case <-time.NewTimer(10 * time.Second).C:
			return errors.New("timeout enabling CentralManager")
		}
	}

	// drain any extra powered-on events from channel
	for len(a.poweredChan) > 0 {
		<-a.poweredChan
	}

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
		cmd.a.poweredChan <- nil
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
	id := prph.Identifier().String()

	// Check if we have a chan allocated for this peripheral, and remove it
	// from the map if so (it's single-use, will be garbage collected after
	// receiver receives the peripheral).
	//
	// If we don't have a chan allocated, the receiving side timed out, so
	// ignore this connection.
	if ch, ok := cmd.a.connectMap.LoadAndDelete(id); ok {
		// Unblock now that we're connected.
		ch.(chan cbgo.Peripheral) <- prph
	}
}

// makeScanResult creates a ScanResult when peripheral is found.
func makeScanResult(prph cbgo.Peripheral, advFields cbgo.AdvFields, rssi int) ScanResult {
	uuid, _ := ParseUUID(prph.Identifier().String())

	var serviceUUIDs []UUID
	for _, u := range advFields.ServiceUUIDs {
		parsedUUID, _ := ParseUUID(u.String())
		serviceUUIDs = append(serviceUUIDs, parsedUUID)
	}

	manufacturerData := make(map[uint16][]byte)
	if len(advFields.ManufacturerData) > 2 {
		manufacturerID := uint16(advFields.ManufacturerData[0])
		manufacturerID += uint16(advFields.ManufacturerData[1]) << 8
		manufacturerData[manufacturerID] = advFields.ManufacturerData[2:]
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
				LocalName:        advFields.LocalName,
				ServiceUUIDs:     serviceUUIDs,
				ManufacturerData: manufacturerData,
			},
		},
	}
}

// PeripheralManager delegate functions

type peripheralManagerDelegate struct {
	cbgo.PeripheralManagerDelegateBase

	a *Adapter
}
