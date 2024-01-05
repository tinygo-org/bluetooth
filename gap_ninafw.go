//go:build ninafw

package bluetooth

import (
	"errors"
	"time"
)

const defaultMTU = 23

var (
	ErrConnect = errors.New("bluetooth: could not connect")
)

// Scan starts a BLE scan.
func (a *Adapter) Scan(callback func(*Adapter, ScanResult)) error {
	if a.scanning {
		return errScanning
	}

	if err := a.hci.leSetScanEnable(false, true); err != nil {
		return err
	}

	// passive scanning, every 40ms, for 30ms
	if err := a.hci.leSetScanParameters(0x00, 0x0080, 0x0030, 0x00, 0x00); err != nil {
		return err
	}

	a.scanning = true

	// scan with duplicates
	if err := a.hci.leSetScanEnable(true, false); err != nil {
		return err
	}

	lastUpdate := time.Now().UnixNano()

	for {
		if err := a.hci.poll(); err != nil {
			return err
		}

		switch {
		case a.hci.advData.reported:
			adf := AdvertisementFields{}
			if a.hci.advData.eirLength > 31 {
				if _debug {
					println("eirLength too long")
				}

				a.hci.clearAdvData()
				continue
			}

			for i := 0; i < int(a.hci.advData.eirLength); {
				l, t := int(a.hci.advData.eirData[i]), a.hci.advData.eirData[i+1]
				if l < 1 {
					break
				}

				switch t {
				case 0x02, 0x03:
					// 16-bit Service Class UUID
				case 0x06, 0x07:
					// 128-bit Service Class UUID
				case 0x08, 0x09:
					if _debug {
						println("local name", string(a.hci.advData.eirData[i+2:i+1+l]))
					}

					adf.LocalName = string(a.hci.advData.eirData[i+2 : i+1+l])
				case 0xFF:
					// Manufacturer Specific Data
				}

				i += l + 1
			}

			random := a.hci.advData.peerBdaddrType == 0x01

			callback(a, ScanResult{
				Address: Address{
					MACAddress{
						MAC:      makeAddress(a.hci.advData.peerBdaddr),
						isRandom: random,
					},
				},
				RSSI: int16(a.hci.advData.rssi),
				AdvertisementPayload: &advertisementFields{
					AdvertisementFields: adf,
				},
			})

			a.hci.clearAdvData()
			time.Sleep(10 * time.Millisecond)

		default:
			if !a.scanning {
				return nil
			}

			if _debug && (time.Now().UnixNano()-lastUpdate)/int64(time.Second) > 1 {
				println("still scanning...")
				lastUpdate = time.Now().UnixNano()
			}

			time.Sleep(10 * time.Millisecond)
		}
	}

	return nil
}

func (a *Adapter) StopScan() error {
	if !a.scanning {
		return errNotScanning
	}

	if err := a.hci.leSetScanEnable(false, false); err != nil {
		return err
	}

	a.scanning = false

	return nil
}

// Address contains a Bluetooth MAC address.
type Address struct {
	MACAddress
}

// Connect starts a connection attempt to the given peripheral device address.
func (a *Adapter) Connect(address Address, params ConnectionParams) (*Device, error) {
	if _debug {
		println("Connect")
	}

	random := uint8(0)
	if address.isRandom {
		random = 1
	}
	if err := a.hci.leCreateConn(0x0060, 0x0030, 0x00,
		random, makeNINAAddress(address.MAC),
		0x00, 0x0006, 0x000c, 0x0000, 0x00c8, 0x0004, 0x0006); err != nil {
		return nil, err
	}

	// are we connected?
	start := time.Now().UnixNano()
	for {
		if err := a.hci.poll(); err != nil {
			return nil, err
		}

		if a.hci.connectData.connected {
			defer a.hci.clearConnectData()

			random := false
			if address.isRandom {
				random = true
			}

			d := &Device{adapter: a,
				handle: a.hci.connectData.handle,
				Address: Address{
					MACAddress{
						MAC:      makeAddress(a.hci.connectData.peerBdaddr),
						isRandom: random},
				},
				mtu:                       defaultMTU,
				notificationRegistrations: make([]notificationRegistration, 0),
			}
			a.connectedDevices = append(a.connectedDevices, d)

			return d, nil

		} else {
			// check for timeout
			if (time.Now().UnixNano()-start)/int64(time.Second) > 5 {
				break
			}

			time.Sleep(10 * time.Millisecond)
		}
	}

	// cancel connection attempt that failed
	if err := a.hci.leCancelConn(); err != nil {
		return nil, err
	}

	return nil, ErrConnect
}

type notificationRegistration struct {
	handle   uint16
	callback func([]byte)
}

// Device is a connection to a remote peripheral.
type Device struct {
	adapter *Adapter
	Address Address
	handle  uint16
	mtu     uint16

	notificationRegistrations []notificationRegistration
}

// Disconnect from the BLE device.
func (d *Device) Disconnect() error {
	if _debug {
		println("Disconnect")
	}
	if err := d.adapter.hci.disconnect(d.handle); err != nil {
		return err
	}

	d.adapter.connectedDevices = []*Device{}
	return nil
}

func (d *Device) findNotificationRegistration(handle uint16) *notificationRegistration {
	for _, n := range d.notificationRegistrations {
		if n.handle == handle {
			return &n
		}
	}

	return nil
}

func (d *Device) addNotificationRegistration(handle uint16, callback func([]byte)) {
	d.notificationRegistrations = append(d.notificationRegistrations,
		notificationRegistration{
			handle:   handle,
			callback: callback,
		})
}

func (d *Device) startNotifications() {
	d.adapter.startNotifications()
}
