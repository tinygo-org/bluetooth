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

const (
	ADTypeLimitedDiscoverable    = 0x01
	ADTypeGeneralDiscoverable    = 0x02
	ADTypeFlagsBREDRNotSupported = 0x04

	ADFlags                          = 0x01
	ADIncompleteAdvertisedService16  = 0x02
	ADCompleteAdvertisedService16    = 0x03
	ADIncompleteAdvertisedService128 = 0x06
	ADCompleteAdvertisedService128   = 0x07
	ADShortLocalName                 = 0x08
	ADCompleteLocalName              = 0x09
	ADServiceData                    = 0x16
	ADManufacturerData               = 0xFF
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
				case ADIncompleteAdvertisedService16, ADCompleteAdvertisedService16:
					adf.ServiceUUIDs = append(adf.ServiceUUIDs, New16BitUUID(binary.LittleEndian.Uint16(a.hci.advData.eirData[i+2:i+4])))
				case ADIncompleteAdvertisedService128, ADCompleteAdvertisedService128:
					var uuid [16]byte
					copy(uuid[:], a.hci.advData.eirData[i+2:i+18])
					adf.ServiceUUIDs = append(adf.ServiceUUIDs, NewUUID(uuid))
				case ADShortLocalName, ADCompleteLocalName:
					if debug {
						println("local name", string(a.hci.advData.eirData[i+2:i+1+l]))
					}

					adf.LocalName = string(a.hci.advData.eirData[i+2 : i+1+l])
				case ADServiceData:
					// TODO: handle service data
				case ADManufacturerData:
					// TODO: handle manufacturer data
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
	if err := a.hci.leCreateConn(0x0060, // interval
		0x0030,                       // window
		0x00,                         // initiatorFilter
		random,                       // peerBdaddrType
		makeNINAAddress(address.MAC), // peerBdaddr
		0x00,                         // ownBdaddrType
		0x0006,                       // minInterval
		0x000c,                       // maxInterval
		0x0000,                       // latency
		0x00c8,                       // supervisionTimeout
		0x0004,                       // minCeLength
		0x0006); err != nil {         // maxCeLength
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

	advertisementType  AdvertisingType
	localName          []byte
	serviceUUIDs       []UUID
	interval           uint16
	manufacturerData   []ManufacturerDataElement
	serviceData        []ServiceDataElement
	stop               chan struct{}
	genericServiceInit bool
}

// DefaultAdvertisement returns the default advertisement instance but does not
// configure it.
func (a *Adapter) DefaultAdvertisement() *Advertisement {
	if defaultAdvertisement.adapter == nil {
		defaultAdvertisement.adapter = a
		defaultAdvertisement.stop = make(chan struct{})
	}

	return &defaultAdvertisement
}

// Configure this advertisement.
func (a *Advertisement) Configure(options AdvertisementOptions) error {
	a.advertisementType = options.AdvertisementType

	switch {
	case options.LocalName != "":
		a.localName = []byte(options.LocalName)
	default:
		a.localName = []byte("TinyGo")
	}

	a.serviceUUIDs = append([]UUID{}, options.ServiceUUIDs...)
	a.interval = uint16(options.Interval)
	if a.interval == 0 {
		a.interval = 0x0800 // default interval is 1.28 seconds
	}
	a.manufacturerData = append([]ManufacturerDataElement{}, options.ManufacturerData...)
	a.serviceData = append([]ServiceDataElement{}, options.ServiceData...)

	a.configureGenericServices(string(a.localName), 0x0540) // Generic Sensor. TODO: make this configurable

	return nil
}

// Start advertisement. May only be called after it has been configured.
func (a *Advertisement) Start() error {
	// uint8_t type = (_connectable) ? 0x00 : (_localName ? 0x02 : 0x03);
	typ := uint8(a.advertisementType)

	if err := a.adapter.hci.leSetAdvertisingParameters(a.interval, a.interval,
		typ, 0x00, 0x00, [6]byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00}, 0x07, 0); err != nil {
		return err
	}

	var advertisingData [31]byte
	advertisingDataLen := uint8(0)

	advertisingData[0] = 0x02 // length
	advertisingData[1] = ADFlags
	advertisingData[2] = ADTypeGeneralDiscoverable + ADTypeFlagsBREDRNotSupported
	advertisingDataLen += 3

	// TODO: handle multiple service UUIDs
	if len(a.serviceUUIDs) == 1 {
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

		advertisingData[3] = 0x03 // length
		advertisingData[4] = ADCompleteAdvertisedService16
		advertisingDataLen += sz + 2
	}

	if len(a.manufacturerData) > 0 {
		for _, md := range a.manufacturerData {
			if advertisingDataLen+4+uint8(len(md.Data)) > 31 {
				return errors.New("ManufacturerData too long")
			}

			advertisingData[advertisingDataLen] = 3 + uint8(len(md.Data)) // length
			advertisingData[advertisingDataLen+1] = ADManufacturerData

			binary.LittleEndian.PutUint16(advertisingData[advertisingDataLen+2:], md.CompanyID)

			copy(advertisingData[advertisingDataLen+4:], md.Data)
			advertisingDataLen += 4 + uint8(len(md.Data))
		}
	}

	if err := a.adapter.hci.leSetAdvertisingData(advertisingData[:advertisingDataLen]); err != nil {
		return err
	}

	if err := a.setServiceData(a.serviceData); err != nil {
		return err
	}

	if err := a.adapter.hci.leSetAdvertiseEnable(true); err != nil {
		return err
	}

	// go routine to poll for HCI events while advertising
	go func() {
		for {
			select {
			case <-a.stop:
				return
			default:
			}

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
	err := a.adapter.hci.leSetAdvertiseEnable(false)
	if err != nil {
		return err
	}

	time.Sleep(5 * time.Millisecond)
	// stop the go routine that polls for HCI events
	a.adapter.att.clearLocalData()
	a.stop <- struct{}{}
	return nil
}

// SetServiceData sets the service data for the advertisement.
func (a *Advertisement) setServiceData(sd []ServiceDataElement) error {
	a.serviceData = sd

	var scanResponseData [31]byte
	scanResponseDataLen := uint8(0)

	switch {
	case len(a.localName) > 29:
		scanResponseData[0] = 1 + 29 // length
		scanResponseData[1] = ADCompleteLocalName
		copy(scanResponseData[2:], a.localName[:29])
		scanResponseDataLen = 31
	case len(a.localName) > 0:
		scanResponseData[0] = uint8(1 + len(a.localName)) // length
		scanResponseData[1] = ADShortLocalName
		copy(scanResponseData[2:], a.localName)
		scanResponseDataLen = uint8(2 + len(a.localName))
	}

	if len(a.serviceData) > 0 {
		for _, sde := range a.serviceData {
			if scanResponseDataLen+4+uint8(len(sde.Data)) > 31 {
				return errors.New("ServiceData too long")
			}

			switch {
			case sde.UUID.Is16Bit():
				binary.LittleEndian.PutUint16(scanResponseData[scanResponseDataLen+2:], sde.UUID.Get16Bit())
			case sde.UUID.Is32Bit():
				return errors.New("32-bit ServiceData UUIDs not yet supported")
			}

			scanResponseData[scanResponseDataLen] = 3 + uint8(len(sde.Data)) // length
			scanResponseData[scanResponseDataLen+1] = ADServiceData

			copy(scanResponseData[scanResponseDataLen+4:], sde.Data)
			scanResponseDataLen += 4 + uint8(len(sde.Data))
		}
	}

	if err := a.adapter.hci.leSetScanResponseData(scanResponseData[:scanResponseDataLen]); err != nil {
		return err
	}

	return nil
}

// configureGenericServices adds the Generic Access and Generic Attribute services that are
// required by the Bluetooth specification.
// Note that once these services are added, they cannot be removed or changed.
func (a *Advertisement) configureGenericServices(name string, appearance uint16) {
	if a.genericServiceInit {
		return
	}

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
					Value: []byte{byte(appearance & 0xff), byte(appearance >> 8)},
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
	a.genericServiceInit = true
}
