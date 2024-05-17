//go:build hci || ninafw || cyw43439

package bluetooth

import (
	"encoding/binary"
	"errors"
	"slices"
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
				if debug {
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
					adf.ServiceUUIDs = append(adf.ServiceUUIDs, New16BitUUID(binary.LittleEndian.Uint16(a.hci.advData.eirData[i+2:i+4])))
				case 0x06, 0x07:
					// 128-bit Service Class UUID
					var uuid [16]byte
					copy(uuid[:], a.hci.advData.eirData[i+2:i+18])
					adf.ServiceUUIDs = append(adf.ServiceUUIDs, NewUUID(uuid))
				case 0x08, 0x09:
					if debug {
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
			time.Sleep(5 * time.Millisecond)

		default:
			if !a.scanning {
				return nil
			}

			if debug && (time.Now().UnixNano()-lastUpdate)/int64(time.Second) > 1 {
				println("still scanning...")
				lastUpdate = time.Now().UnixNano()
			}

			time.Sleep(5 * time.Millisecond)
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
func (a *Adapter) Connect(address Address, params ConnectionParams) (Device, error) {
	if debug {
		println("Connect")
	}

	random := uint8(0)
	if address.isRandom {
		random = 1
	}
	if err := a.hci.leCreateConn(0x0060, 0x0030, 0x00,
		random, makeNINAAddress(address.MAC),
		0x00, 0x0006, 0x000c, 0x0000, 0x00c8, 0x0004, 0x0006); err != nil {
		return Device{}, err
	}

	// are we connected?
	start := time.Now().UnixNano()
	for {
		if err := a.hci.poll(); err != nil {
			return Device{}, err
		}

		if a.hci.connectData.connected {
			defer a.hci.clearConnectData()

			random := false
			if address.isRandom {
				random = true
			}

			d := Device{
				Address: Address{
					MACAddress{
						MAC:      makeAddress(a.hci.connectData.peerBdaddr),
						isRandom: random},
				},
				deviceInternal: &deviceInternal{
					adapter:                   a,
					handle:                    a.hci.connectData.handle,
					mtu:                       defaultMTU,
					notificationRegistrations: make([]notificationRegistration, 0),
				},
			}
			a.addConnection(d)

			return d, nil

		} else {
			// check for timeout
			if (time.Now().UnixNano()-start)/int64(time.Second) > 5 {
				break
			}

			time.Sleep(5 * time.Millisecond)
		}
	}

	// cancel connection attempt that failed
	if err := a.hci.leCancelConn(); err != nil {
		return Device{}, err
	}

	return Device{}, ErrConnect
}

type notificationRegistration struct {
	handle   uint16
	callback func([]byte)
}

// Device is a connection to a remote peripheral.
type Device struct {
	Address Address
	*deviceInternal
}

type deviceInternal struct {
	adapter *Adapter
	handle  uint16
	mtu     uint16

	notificationRegistrations []notificationRegistration
}

// Disconnect from the BLE device.
func (d Device) Disconnect() error {
	if debug {
		println("Disconnect")
	}
	if err := d.adapter.hci.disconnect(d.handle); err != nil {
		return err
	}

	d.adapter.removeConnection(d)
	return nil
}

// RequestConnectionParams requests a different connection latency and timeout
// of the given device connection. Fields that are unset will be left alone.
// Whether or not the device will actually honor this, depends on the device and
// on the specific parameters.
//
// On NINA, this call hasn't been implemented yet.
func (d Device) RequestConnectionParams(params ConnectionParams) error {
	return nil
}

func (d Device) findNotificationRegistration(handle uint16) *notificationRegistration {
	for _, n := range d.notificationRegistrations {
		if n.handle == handle {
			return &n
		}
	}

	return nil
}

func (d Device) addNotificationRegistration(handle uint16, callback func([]byte)) {
	d.notificationRegistrations = append(d.notificationRegistrations,
		notificationRegistration{
			handle:   handle,
			callback: callback,
		})
}

func (d Device) startNotifications() {
	d.adapter.startNotifications()
}

var defaultAdvertisement Advertisement

// Advertisement encapsulates a single advertisement instance.
type Advertisement struct {
	adapter *Adapter

	localName    []byte
	serviceUUIDs []UUID
	interval     uint16
}

// DefaultAdvertisement returns the default advertisement instance but does not
// configure it.
func (a *Adapter) DefaultAdvertisement() *Advertisement {
	if defaultAdvertisement.adapter == nil {
		defaultAdvertisement.adapter = a
	}

	return &defaultAdvertisement
}

// Configure this advertisement.
func (a *Advertisement) Configure(options AdvertisementOptions) error {
	switch {
	case options.LocalName != "":
		a.localName = []byte(options.LocalName)
	default:
		a.localName = []byte("TinyGo")
	}

	a.serviceUUIDs = append([]UUID{}, options.ServiceUUIDs...)
	a.interval = uint16(options.Interval)

	a.adapter.AddService(
		&Service{
			UUID: ServiceUUIDGenericAccess,
			Characteristics: []CharacteristicConfig{
				{
					UUID:  CharacteristicUUIDDeviceName,
					Flags: CharacteristicReadPermission,
					Value: a.localName,
				},
				{
					UUID:  CharacteristicUUIDAppearance,
					Flags: CharacteristicReadPermission,
				},
			},
		})
	a.adapter.AddService(
		&Service{
			UUID: ServiceUUIDGenericAttribute,
			Characteristics: []CharacteristicConfig{
				{
					UUID:  CharacteristicUUIDServiceChanged,
					Flags: CharacteristicIndicatePermission,
				},
			},
		})

	return nil
}

// Start advertisement. May only be called after it has been configured.
func (a *Advertisement) Start() error {
	// uint8_t type = (_connectable) ? 0x00 : (_localName ? 0x02 : 0x03);
	typ := uint8(0x00)

	if err := a.adapter.hci.leSetAdvertisingParameters(a.interval, a.interval,
		typ, 0x00, 0x00, [6]byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00}, 0x07, 0); err != nil {
		return err
	}

	var advertisingData [31]byte
	advertisingDataLen := uint8(0)

	advertisingData[0] = 0x02
	advertisingData[1] = 0x01
	advertisingData[2] = 0x06
	advertisingDataLen += 3

	// TODO: handle multiple service UUIDs
	if len(a.serviceUUIDs) > 0 {
		uuid := a.serviceUUIDs[0]
		var sz uint8

		switch {
		case uuid.Is16Bit():
			sz = 2
			binary.LittleEndian.PutUint16(advertisingData[5:], uuid.Get16Bit())
		case uuid.Is32Bit():
			sz = 6
			data := uuid.Bytes()
			slices.Reverse(data[:])
			copy(advertisingData[5:], data[:])
		}

		advertisingData[3] = sz + 1
		advertisingData[4] = sz
		advertisingDataLen += sz + 2
	}

	// TODO: handle manufacturer data

	if err := a.adapter.hci.leSetAdvertisingData(advertisingData[:advertisingDataLen]); err != nil {
		return err
	}

	var scanResponseData [31]byte
	scanResponseDataLen := uint8(0)

	switch {
	case len(a.localName) > 29:
		scanResponseData[1] = 0x08
		scanResponseData[0] = 1 + 29
		copy(scanResponseData[2:], a.localName[:29])
		scanResponseDataLen = 31
	case len(a.localName) > 0:
		scanResponseData[1] = 0x09
		scanResponseData[0] = uint8(1 + len(a.localName))
		copy(scanResponseData[2:], a.localName)
		scanResponseDataLen = uint8(2 + len(a.localName))
	}

	if err := a.adapter.hci.leSetScanResponseData(scanResponseData[:scanResponseDataLen]); err != nil {
		return err
	}

	if err := a.adapter.hci.leSetAdvertiseEnable(true); err != nil {
		return err
	}

	// go routine to poll for HCI events while advertising
	go func() {
		for {
			if err := a.adapter.att.poll(); err != nil {
				// TODO: handle error
				if debug {
					println("error polling while advertising:", err.Error())
				}
			}

			time.Sleep(5 * time.Millisecond)
		}
	}()

	return nil
}

// Stop advertisement. May only be called after it has been started.
func (a *Advertisement) Stop() error {
	return a.adapter.hci.leSetAdvertiseEnable(false)
}
