//go:build hci || ninafw || cyw43439

package bluetooth

import (
	"runtime"

	"time"
)

// hciAdapter represents the implementation for the connection to the HCI controller.
type hciAdapter struct {
	hciport hciTransport
	hci     *hci
	att     *att

	isDefault bool
	scanning  bool

	connectHandler func(device Device, connected bool)

	connectedDevices     []Device
	notificationsStarted bool
	charWriteHandlers    []charWriteHandler
}

func (a *hciAdapter) enable() error {
	if err := a.hci.start(); err != nil {
		if debug {
			println("error starting HCI:", err.Error())
		}
		return err
	}

	if err := a.hci.reset(); err != nil {
		if debug {
			println("error resetting HCI:", err.Error())
		}

		return err
	}

	time.Sleep(150 * time.Millisecond)

	if err := a.hci.setEventMask(0x3FFFFFFFFFFFFFFF); err != nil {
		return err
	}

	return a.hci.setLeEventMask(0x00000000000003FF)
}

func (a *hciAdapter) Address() (MACAddress, error) {
	if err := a.hci.readBdAddr(); err != nil {
		return MACAddress{}, err
	}

	return MACAddress{MAC: makeAddress(a.hci.address)}, nil
}

func newBLEStack(port hciTransport) (*hci, *att) {
	h := newHCI(port)
	a := newATT(h)
	h.att = a

	l := newL2CAP(h)
	h.l2cap = l

	return h, a
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
			if err := a.att.poll(); err != nil {
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
			case not := <-a.att.notifications:
				if debug {
					println("notification received", not.connectionHandle, not.handle, not.data)
				}

				d := a.findConnection(not.connectionHandle)
				if d.deviceInternal == nil {
					if debug {
						println("no device found for handle", not.connectionHandle)
					}
					continue
				}

				n := d.findNotificationRegistration(not.handle)
				if n == nil {
					if debug {
						println("no notification registered for handle", not.handle)
					}
					continue
				}

				if n.callback != nil {
					n.callback(not.data)
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
