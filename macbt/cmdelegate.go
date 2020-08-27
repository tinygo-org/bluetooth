// Implements the CentralManagerDelegate interface.  CoreBluetooth
// communicates events asynchronously via callbacks.  This file implements a
// synchronous interface by translating these callbacks into channel
// operations.

package macbt

import (
	"github.com/JuulLabs-OSS/cbgo"
)

// CMDelegate to handle callbacks from CoreBluetooth.
type CMDelegate struct {
}

func (d *CMDelegate) CentralManagerDidUpdateState(cmgr cbgo.CentralManager) {
}

func (d *CMDelegate) DidDiscoverPeripheral(cmgr cbgo.CentralManager, prph cbgo.Peripheral,
	advFields cbgo.AdvFields, rssi int) {
}

func (d *CMDelegate) DidConnectPeripheral(cmgr cbgo.CentralManager, prph cbgo.Peripheral) {
}

func (d *CMDelegate) DidDisconnectPeripheral(cmgr cbgo.CentralManager, prph cbgo.Peripheral, err error) {
}
