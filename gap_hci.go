//go:build hci || ninafw || cyw43439

package bluetooth

import (
	"encoding/binary"
	"errors"
	"slices"
	"strconv"
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

			rp := rawAdvertisementPayload{len: a.hci.advData.eirLength}
			copy(rp.data[:], a.hci.advData.eirData[:a.hci.advData.eirLength])
			if rp.LocalName() != "" {
				println("LocalName:", rp.LocalName())
				adf.LocalName = rp.LocalName()
			}

			// Complete List of 16-bit Service Class UUIDs
			if b := rp.findField(0x03); len(b) > 0 {
				for i := 0; i < len(b)/2; i++ {
					uuid := uint16(b[i*2]) | (uint16(b[i*2+1]) << 8)
					adf.ServiceUUIDs = append(adf.ServiceUUIDs, New16BitUUID(uuid))
				}
			}
			// Incomplete List of 16-bit Service Class UUIDs
			if b := rp.findField(0x02); len(b) > 0 {
				for i := 0; i < len(b)/2; i++ {
					uuid := uint16(b[i*2]) | (uint16(b[i*2+1]) << 8)
					adf.ServiceUUIDs = append(adf.ServiceUUIDs, New16BitUUID(uuid))
				}
			}

			// Complete List of 128-bit Service Class UUIDs
			if b := rp.findField(0x07); len(b) > 0 {
				for i := 0; i < len(b)/16; i++ {
					var uuid [16]byte
					copy(uuid[:], b[i*16:i*16+16])
					adf.ServiceUUIDs = append(adf.ServiceUUIDs, NewUUID(uuid))
				}
			}

			// Incomplete List of 128-bit Service Class UUIDs
			if b := rp.findField(0x06); len(b) > 0 {
				for i := 0; i < len(b)/16; i++ {
					var uuid [16]byte
					copy(uuid[:], b[i*16:i*16+16])
					adf.ServiceUUIDs = append(adf.ServiceUUIDs, NewUUID(uuid))
				}
			}

			// service data
			sd := rp.ServiceData()
			if len(sd) > 0 {
				adf.ServiceData = append(adf.ServiceData, sd...)
			}

			// manufacturer data
			md := rp.ManufacturerData()
			if len(md) > 0 {
				adf.ManufacturerData = append(adf.ManufacturerData, md...)
			}

			random := a.hci.advData.peerBdaddrType == GAPAddressTypeRandomStatic

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

	peerRandom := uint8(0)
	if address.isRandom {
		peerRandom = GAPAddressTypeRandomStatic
	}
	localRandom := uint8(0)
	if a.hci.address.isRandom {
		localRandom = GAPAddressTypeRandomStatic
	}
	if err := a.hci.leCreateConn(0x0060, // interval
		0x0030,                       // window
		0x00,                         // initiatorFilter
		peerRandom,                   // peerBdaddrType
		makeNINAAddress(address.MAC), // peerBdaddr
		localRandom,                  // ownBdaddrType
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

			if a.connectHandler != nil {
				a.connectHandler(d, true)
			}

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

	if options.LocalName != "" {
		a.localName = []byte(options.LocalName)
	}

	a.serviceUUIDs = append([]UUID{}, options.ServiceUUIDs...)
	if options.Interval == 0 {
		// Pick an advertisement interval recommended by Apple (section 35.5
		// Advertising Interval):
		// https://developer.apple.com/accessories/Accessory-Design-Guidelines.pdf
		options.Interval = NewDuration(152500 * time.Microsecond) // 152.5ms
	}
	a.interval = uint16(options.Interval)
	a.manufacturerData = append([]ManufacturerDataElement{}, options.ManufacturerData...)
	a.serviceData = append([]ServiceDataElement{}, options.ServiceData...)

	a.configureGenericServices(string(a.localName), 0x0540) // Generic Sensor. TODO: make this configurable

	return nil
}

// via https://www.bluetooth.com/wp-content/uploads/Files/Specification/HTML/Core-54/out/en/low-energy-controller/link-layer-specification.html
// 4.4.3.5. Advertising reports
// The maximum size of the advertising report is 31 bytes.
const maxAdvLen = 31

// Start advertisement. May only be called after it has been configured.
func (a *Advertisement) Start() error {
	// uint8_t type = (_connectable) ? 0x00 : (_localName ? 0x02 : 0x03);
	typ := uint8(a.advertisementType)

	localRandom := uint8(0)
	if a.adapter.hci.address.isRandom {
		localRandom = GAPAddressTypeRandomStatic
	}

	if err := a.adapter.hci.leSetAdvertisingParameters(a.interval, a.interval,
		typ, localRandom, 0x00, [6]byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00}, 0x07, 0); err != nil {
		return err
	}

	var advertisingData [maxAdvLen]byte
	advertisingDataLen := uint8(0)

	// flags, only if not non-connectable
	if a.advertisementType != AdvertisingTypeNonConnInd {
		advertisingData[0] = 0x02 // length
		advertisingData[1] = ADFlags
		advertisingData[2] = ADTypeGeneralDiscoverable + ADTypeFlagsBREDRNotSupported
		advertisingDataLen += 3
	}

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

		advertisingData[advertisingDataLen] = 0x03 // length
		advertisingData[advertisingDataLen+1] = ADCompleteAdvertisedService16
		advertisingDataLen += sz + 2
	}

	if len(a.manufacturerData) > 0 {
		for _, md := range a.manufacturerData {
			if advertisingDataLen+4+uint8(len(md.Data)) > maxAdvLen {
				return errors.New("ManufacturerData too long:" + strconv.Itoa(int(advertisingDataLen+4+uint8(len(md.Data)))))
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

			switch {
			case a.adapter.hci.connectData.connected:
				random := a.adapter.hci.connectData.peerBdaddrType == 0x01

				d := Device{
					Address: Address{
						MACAddress{
							MAC:      makeAddress(a.adapter.hci.connectData.peerBdaddr),
							isRandom: random},
					},
					deviceInternal: &deviceInternal{
						adapter:                   a.adapter,
						handle:                    a.adapter.hci.connectData.handle,
						mtu:                       defaultMTU,
						notificationRegistrations: make([]notificationRegistration, 0),
					},
				}
				a.adapter.addConnection(d)

				if a.adapter.connectHandler != nil {
					a.adapter.connectHandler(d, true)
				}

				a.adapter.hci.clearConnectData()
			case a.adapter.hci.connectData.disconnected:
				d := Device{
					deviceInternal: &deviceInternal{
						adapter: a.adapter,
						handle:  a.adapter.hci.connectData.handle,
					},
				}
				a.adapter.removeConnection(d)

				if a.adapter.connectHandler != nil {
					a.adapter.connectHandler(d, false)
				}

				a.adapter.hci.clearConnectData()
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
