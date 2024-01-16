//go:build ninafw

package bluetooth

import (
	"machine"
	"runtime"

	"time"
)

const maxConnections = 1

// Adapter represents the UART connection to the NINA fw.
type Adapter struct {
	hci *hci
	att *att

	isDefault bool
	scanning  bool

	connectHandler func(device Device, connected bool)

	connectedDevices     []Device
	notificationsStarted bool
}

// DefaultAdapter is the default adapter on the current system.
//
// Make sure to call Enable() before using it to initialize the adapter.
var DefaultAdapter = &Adapter{
	isDefault: true,
	connectHandler: func(device Device, connected bool) {
		return
	},
	connectedDevices: make([]Device, 0, maxConnections),
}

// Enable configures the BLE stack. It must be called before any
// Bluetooth-related calls (unless otherwise indicated).
func (a *Adapter) Enable() error {
	// reset the NINA in BLE mode
	machine.NINA_CS.Configure(machine.PinConfig{Mode: machine.PinOutput})
	machine.NINA_CS.Low()

	if machine.NINA_RESET_INVERTED {
		resetNINAInverted()
	} else {
		resetNINA()
	}

	// serial port for nina chip
	uart := machine.UART_NINA
	cfg := machine.UARTConfig{
		TX:       machine.NINA_TX,
		RX:       machine.NINA_RX,
		BaudRate: machine.NINA_BAUDRATE,
	}
	if !machine.NINA_SOFT_FLOWCONTROL {
		cfg.CTS = machine.NINA_CTS
		cfg.RTS = machine.NINA_RTS
	}

	uart.Configure(cfg)

	a.hci, a.att = newBLEStack(uart)
	if machine.NINA_SOFT_FLOWCONTROL {
		a.hci.softRTS = machine.NINA_RTS
		a.hci.softRTS.Configure(machine.PinConfig{Mode: machine.PinOutput})
		a.hci.softRTS.High()

		a.hci.softCTS = machine.NINA_CTS
		machine.NINA_CTS.Configure(machine.PinConfig{Mode: machine.PinInput})
	}

	a.hci.start()

	if err := a.hci.reset(); err != nil {
		return err
	}

	time.Sleep(150 * time.Millisecond)

	if err := a.hci.setEventMask(0x3FFFFFFFFFFFFFFF); err != nil {
		return err
	}

	if err := a.hci.setLeEventMask(0x00000000000003FF); err != nil {
		return err
	}

	return nil
}

func (a *Adapter) Address() (MACAddress, error) {
	if err := a.hci.readBdAddr(); err != nil {
		return MACAddress{}, err
	}

	return MACAddress{MAC: makeAddress(a.hci.address)}, nil
}

func newBLEStack(uart *machine.UART) (*hci, *att) {
	h := newHCI(uart)
	a := newATT(h)
	h.att = a

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

func resetNINA() {
	machine.NINA_RESETN.Configure(machine.PinConfig{Mode: machine.PinOutput})

	machine.NINA_RESETN.High()
	time.Sleep(100 * time.Millisecond)
	machine.NINA_RESETN.Low()
	time.Sleep(1000 * time.Millisecond)
}

func resetNINAInverted() {
	machine.NINA_RESETN.Configure(machine.PinConfig{Mode: machine.PinOutput})

	machine.NINA_RESETN.Low()
	time.Sleep(100 * time.Millisecond)
	machine.NINA_RESETN.High()
	time.Sleep(1000 * time.Millisecond)
}

func (a *Adapter) startNotifications() {
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

			time.Sleep(10 * time.Millisecond)
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

				d := a.findDevice(not.connectionHandle)
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

func (a *Adapter) findDevice(handle uint16) Device {
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
