//go:build ninafw

package bluetooth

import (
	"machine"
	"runtime"

	"time"
)

const maxConnections = 1

// NINAConfig encapsulates the hardware options for the NINA firmware
type NINAConfig struct {
	UART *machine.UART

	CS     machine.Pin
	ACK    machine.Pin
	GPIO0  machine.Pin
	RESETN machine.Pin

	TX  machine.Pin
	RX  machine.Pin
	CTS machine.Pin
	RTS machine.Pin

	BaudRate        uint32
	ResetInverted   bool
	SoftFlowControl bool
}

// AdapterConfig is used to set the hardware options for the NINA adapter prior
// to calling DefaultAdapter.Enable()
var AdapterConfig NINAConfig

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
	AdapterConfig.CS.Configure(machine.PinConfig{Mode: machine.PinOutput})
	AdapterConfig.CS.Low()

	if AdapterConfig.ResetInverted {
		resetNINAInverted()
	} else {
		resetNINA()
	}

	// serial port for nina chip
	uart := AdapterConfig.UART
	cfg := machine.UARTConfig{
		TX:       AdapterConfig.TX,
		RX:       AdapterConfig.RX,
		BaudRate: AdapterConfig.BaudRate,
	}
	if !AdapterConfig.SoftFlowControl {
		cfg.CTS = AdapterConfig.CTS
		cfg.RTS = AdapterConfig.RTS
	}

	uart.Configure(cfg)

	a.hci, a.att = newBLEStack(uart)
	if AdapterConfig.SoftFlowControl {
		a.hci.softRTS = AdapterConfig.RTS
		a.hci.softRTS.Configure(machine.PinConfig{Mode: machine.PinOutput})
		a.hci.softRTS.High()

		a.hci.softCTS = AdapterConfig.CTS
		AdapterConfig.CTS.Configure(machine.PinConfig{Mode: machine.PinInput})
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

func resetNINA() {
	AdapterConfig.RESETN.Configure(machine.PinConfig{Mode: machine.PinOutput})

	AdapterConfig.RESETN.High()
	time.Sleep(100 * time.Millisecond)
	AdapterConfig.RESETN.Low()
	time.Sleep(1000 * time.Millisecond)
}

func resetNINAInverted() {
	AdapterConfig.RESETN.Configure(machine.PinConfig{Mode: machine.PinOutput})

	AdapterConfig.RESETN.Low()
	time.Sleep(100 * time.Millisecond)
	AdapterConfig.RESETN.High()
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

func (a *Adapter) addConnection(d Device) {
	a.connectedDevices = append(a.connectedDevices, d)
}

func (a *Adapter) removeConnection(d Device) {
	for i := range a.connectedDevices {
		if d.handle == a.connectedDevices[i].handle {
			a.connectedDevices[i] = a.connectedDevices[len(a.connectedDevices)-1]
			a.connectedDevices[len(a.connectedDevices)-1] = Device{}
			a.connectedDevices = a.connectedDevices[:len(a.connectedDevices)-1]

			return
		}
	}
}

func (a *Adapter) findConnection(handle uint16) Device {
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
