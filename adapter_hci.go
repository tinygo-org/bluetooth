//go:build hci || ninafw || cyw43439 || espradio

package bluetooth

import (
	"runtime"
	"time"

	"tinygo.org/x/bluetooth/hci"
)

// hciAdapter represents the implementation for the connection to the HCI controller.
type hciAdapter struct {
	hciport hci.Transport
	hci     *hci.HCI
	att     *hci.ATT

	isDefault bool
	scanning  bool
	scanType  ScanType

	connectHandler func(device Device, connected bool)

	connectedDevices     []Device
	notificationsStarted bool
	charWriteHandlers    []charWriteHandler
}

func (a *hciAdapter) enable() error {
	if err := a.hci.Start(); err != nil {
		if debug {
			println("error starting HCI:", err.Error())
		}
		return err
	}

	if err := a.hci.Reset(); err != nil {
		if debug {
			println("error resetting HCI:", err.Error())
		}

		return err
	}

	time.Sleep(150 * time.Millisecond)

	if err := a.hci.SetEventMask(0x3FFFFFFFFFFFFFFF); err != nil {
		return err
	}

	return a.hci.SetLEEventMask(0x00000000000003FF)
}

func (a *hciAdapter) Address() (MACAddress, error) {
	var empty MAC
	if a.hci.Address().MAC != empty {
		return a.hci.Address(), nil
	}

	if err := a.hci.ReadBdAddr(); err != nil {
		return MACAddress{}, err
	}

	return a.hci.Address(), nil
}

// ScanType selects whether the scanner transmits while scanning.
type ScanType uint8

const (
	// ScanTypeActive sends a SCAN_REQ to advertisers that allow it, so scan
	// response data (usually the complete local name) is reported as well.
	// This is the default.
	ScanTypeActive ScanType = iota

	// ScanTypePassive only listens. It uses less power and does not reveal the
	// scanner's presence, but misses scan response data.
	ScanTypePassive
)

// hciValue returns the scan_type field for HCI LE Set Scan Parameters.
func (t ScanType) hciValue() uint8 {
	if t == ScanTypePassive {
		return 0x00
	}
	return 0x01
}

// SetScanType sets the scan type used by Scan. Call it before Scan; changing it
// during a scan takes effect on the next call to Scan.
func (a *Adapter) SetScanType(t ScanType) {
	a.scanType = t
}

// SetRandomAddress sets the random static address that the controller uses.
func (a *Adapter) SetRandomAddress(mac MAC) error {
	return a.hci.SetRandomAddress(mac)
}

// initStack builds the protocol stack on a transport.
func (a *hciAdapter) initStack(port hci.Transport) {
	stack := hci.NewStack(port)
	a.hci = stack.HCI
	a.att = stack.ATT
}

// Convert a NINA MAC address into a Go MAC address.
func makeAddress(mac [6]uint8) MAC {
	return MAC{
		uint8(mac[0]),
		uint8(mac[1]),
		uint8(mac[2]),
		uint8(mac[3]),
		uint8(mac[4]),
		uint8(mac[5]),
	}
}

// Convert a Go MAC address into a NINA MAC Address.
func makeNINAAddress(mac MAC) [6]uint8 {
	return [6]uint8{
		uint8(mac[0]),
		uint8(mac[1]),
		uint8(mac[2]),
		uint8(mac[3]),
		uint8(mac[4]),
		uint8(mac[5]),
	}
}

func (a *hciAdapter) startNotifications() {
	if a.notificationsStarted {
		return
	}

	if debug {
		println("starting notifications...")
	}

	a.notificationsStarted = true

	// go routine to poll for HCI events for ATT notifications
	go func() {
		for {
			if err := a.att.Poll(); err != nil {
				// TODO: handle error
				if debug {
					println("error polling for notifications:", err.Error())
				}
			}

			time.Sleep(5 * time.Millisecond)
		}
	}()

	// go routine to handle characteristic notifications
	go func() {
		for {
			select {
			case not := <-a.att.Notifications():
				if debug {
					println("notification received", not.ConnectionHandle, not.Handle, not.Data)
				}

				d := a.findConnection(not.ConnectionHandle)
				if d.deviceInternal == nil {
					if debug {
						println("no device found for handle", not.ConnectionHandle)
					}
					continue
				}

				n := d.findNotificationRegistration(not.Handle)
				if n == nil {
					if debug {
						println("no notification registered for handle", not.Handle)
					}
					continue
				}

				if n.callback != nil {
					n.callback(not.Data)
				}

			default:
			}

			runtime.Gosched()
		}
	}()
}

func (a *hciAdapter) addConnection(d Device) {
	a.connectedDevices = append(a.connectedDevices, d)
}

func (a *hciAdapter) removeConnection(d Device) {
	for i := range a.connectedDevices {
		if d.handle == a.connectedDevices[i].handle {
			a.connectedDevices[i] = a.connectedDevices[len(a.connectedDevices)-1]
			a.connectedDevices[len(a.connectedDevices)-1] = Device{}
			a.connectedDevices = a.connectedDevices[:len(a.connectedDevices)-1]

			return
		}
	}
}

func (a *hciAdapter) findConnection(handle uint16) Device {
	for _, d := range a.connectedDevices {
		if d.handle == handle {
			if debug {
				println("found device", handle, d.Address.String(), "with notifications registered", len(d.notificationRegistrations))
			}

			return d
		}
	}

	return Device{}
}

// charWriteHandler contains a handler->callback mapping for characteristic
// writes.
type charWriteHandler struct {
	handle   uint16
	callback func(connection Connection, offset int, value []byte)
}

// getCharWriteHandler returns a characteristic write handler if one matches the
// handle, or nil otherwise.
func (a *Adapter) getCharWriteHandler(handle uint16) *charWriteHandler {
	for i := range a.charWriteHandlers {
		h := &a.charWriteHandlers[i]
		if h.handle == handle {
			return h
		}
	}

	return nil
}
