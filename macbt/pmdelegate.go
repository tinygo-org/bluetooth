// Implements the PeripheralManagerDelegate interface.
// CoreBluetooth communicates events asynchronously via callbacks.  This file
// implements a synchronous interface by translating these callbacks into
// channel operations.

package macbt

import (
	"github.com/JuulLabs-OSS/cbgo"
)

// PMDelegate to handle callbacks from CoreBluetooth.
type PMDelegate struct {
}

func (d *PMDelegate) PeripheralManagerDidUpdateState(pmgr cbgo.PeripheralManager) {
}

func (d *PMDelegate) DidAddService(pmgr cbgo.PeripheralManager, svc cbgo.Service, err error) {
}

func (d *PMDelegate) DidStartAdvertising(pmgr cbgo.PeripheralManager, err error) {
}

func (d *PMDelegate) DidReceiveReadRequest(pmgr cbgo.PeripheralManager, cbreq cbgo.ATTRequest) {
}

func (d *PMDelegate) DidReceiveWriteRequests(pmgr cbgo.PeripheralManager, cbreqs []cbgo.ATTRequest) {
}

func (d *PMDelegate) CentralDidSubscribe(pmgr cbgo.PeripheralManager, cent cbgo.Central, cbchr cbgo.Characteristic) {
}

func (d *PMDelegate) CentralDidUnsubscribe(pmgr cbgo.PeripheralManager, cent cbgo.Central, chr cbgo.Characteristic) {
}
