package hci

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
	"slices"
	"sync"
	"time"

	"tinygo.org/x/bluetooth/ble"
)

const (
	OpError               = 0x01
	OpMTUReq              = 0x02
	OpMTUResponse         = 0x03
	OpFindInfoReq         = 0x04
	OpFindInfoResponse    = 0x05
	OpFindByTypeReq       = 0x06
	OpFindByTypeResponse  = 0x07
	OpReadByTypeReq       = 0x08
	OpReadByTypeResponse  = 0x09
	OpReadReq             = 0x0a
	OpReadResponse        = 0x0b
	OpReadBlobReq         = 0x0c
	OpReadBlobResponse    = 0x0d
	OpReadMultiReq        = 0x0e
	OpReadMultiResponse   = 0x0f
	OpReadByGroupReq      = 0x10
	OpReadByGroupResponse = 0x11
	OpWriteReq            = 0x12
	OpWriteResponse       = 0x13
	OpWriteCmd            = 0x52
	OpPrepWriteReq        = 0x16
	OpPrepWriteResponse   = 0x17
	OpExecWriteReq        = 0x18
	OpExecWriteResponse   = 0x19
	OpHandleNotify        = 0x1b
	OpHandleInd           = 0x1d
	OpHandleCNF           = 0x1e
	OpSignedWriteCmd      = 0xd2
)

const (
	MaximumMTU          = 248
	maximumMTUBufferLen = MaximumMTU + 8
)

// ShortUUID is a 16 bit UUID as it appears on the wire.
type ShortUUID uint16

// The GATT attribute types, from the Bluetooth Core Specification, Section
// 3.2 of Part G.
const (
	UUIDUnknown                    ShortUUID = 0x0000
	UUIDPrimaryService             ShortUUID = 0x2800
	UUIDCharacteristic             ShortUUID = 0x2803
	UUIDDescriptor                 ShortUUID = 0x2900
	UUIDClientCharacteristicConfig ShortUUID = 0x2902
)

// UUID returns the full length UUID for this short UUID.
func (s ShortUUID) UUID() ble.UUID {
	return ble.New16BitUUID(uint16(s))
}

var errNotImplemented = errors.New("bluetooth/hci: not implemented")

var (
	ErrATTTimeout           = errors.New("bluetooth: ATT timeout")
	ErrATTUnknownEvent      = errors.New("bluetooth: ATT unknown event")
	ErrATTUnknown           = errors.New("bluetooth: ATT unknown error")
	ErrATTOp                = errors.New("bluetooth: ATT OP error")
	ErrATTUnknownConnection = errors.New("bluetooth: ATT unknown connection")
	ErrATTAttributeNotFound = errors.New("bluetooth: ATT attribute not found")
)

const defaultTimeoutSeconds = 10

// Service is a service found on a remote device, or one that this stack makes
// available.
type Service struct {
	StartHandle uint16
	EndHandle   uint16
	UUID        ble.UUID
}

func (s *Service) unmarshal(buf []byte) (int, error) {
	s.StartHandle = binary.LittleEndian.Uint16(buf[0:])
	s.EndHandle = binary.LittleEndian.Uint16(buf[2:])

	sz := 4
	switch len(buf) - 4 {
	case 2:
		s.UUID = ble.New16BitUUID(binary.LittleEndian.Uint16(buf[4:]))
		sz += 2
	case 16:
		var uuid [16]byte
		copy(uuid[:], buf[4:])
		slices.Reverse(uuid[:])
		s.UUID = ble.NewUUID(uuid)
		sz += 16
	}

	return sz, nil
}

func (s *Service) marshal(p []byte) (int, error) {
	binary.LittleEndian.PutUint16(p[0:], s.StartHandle)
	binary.LittleEndian.PutUint16(p[2:], s.EndHandle)

	sz := 4
	switch {
	case s.UUID.Is16Bit():
		binary.LittleEndian.PutUint16(p[4:], s.UUID.Get16Bit())
		sz += 2
	default:
		uuid := s.UUID.BytesLittleEndian()
		copy(p[4:], uuid[:])
		sz += 16
	}

	return sz, nil
}

// CharacteristicHandler is the callback set for a local characteristic that
// this stack makes available to remote devices. Any field may be nil, and ATT
// then answers the matching request with an error.
//
// It is a struct of functions rather than an interface, because on TinyGo an
// interface call on this path costs about 5 kB of flash.
type CharacteristicHandler struct {
	// ReadValue returns the current value of the characteristic.
	ReadValue func() ([]byte, error)

	// WriteValue stores a value written by a remote device.
	WriteValue func(data []byte) (int, error)

	// ReadCCCD returns the Client Characteristic Configuration descriptor.
	ReadCCCD func() (uint16, error)

	// WriteCCCD stores the Client Characteristic Configuration descriptor.
	WriteCCCD func(value uint16) error
}

// Characteristic is a characteristic found on a remote device, or one that
// this stack makes available.
type Characteristic struct {
	StartHandle uint16
	Properties  uint8
	ValueHandle uint16
	UUID        ble.UUID

	handler CharacteristicHandler
}

func (c *Characteristic) unmarshal(buf []byte) (int, error) {
	c.StartHandle = binary.LittleEndian.Uint16(buf[0:])
	c.Properties = buf[2]
	c.ValueHandle = binary.LittleEndian.Uint16(buf[3:])

	sz := 5
	switch len(buf) - 5 {
	case 2:
		c.UUID = ble.New16BitUUID(binary.LittleEndian.Uint16(buf[5:]))
		sz += 2
	case 16:
		var uuid [16]byte
		copy(uuid[:], buf[5:])
		slices.Reverse(uuid[:])
		c.UUID = ble.NewUUID(uuid)
		sz += 16
	}

	return sz, nil
}

func (c *Characteristic) marshal(p []byte) (int, error) {
	binary.LittleEndian.PutUint16(p[0:], c.StartHandle)
	p[2] = c.Properties
	binary.LittleEndian.PutUint16(p[3:], c.ValueHandle)

	sz := 5
	switch {
	case c.UUID.Is16Bit():
		binary.LittleEndian.PutUint16(p[5:], c.UUID.Get16Bit())
		sz += 2
	default:
		uuid := c.UUID.BytesLittleEndian()
		copy(p[5:], uuid[:])
		sz += 16
	}

	return sz, nil
}

// Descriptor is a descriptor found on a remote device.
type Descriptor struct {
	Handle uint16
	Data   []byte
}

func (d *Descriptor) unmarshal(buf []byte) (int, error) {
	d.Handle = binary.LittleEndian.Uint16(buf[0:])
	d.Data = append(d.Data, buf[2:]...)

	return len(d.Data) + 2, nil
}

func (d *Descriptor) marshal(p []byte) (int, error) {
	binary.LittleEndian.PutUint16(p[0:], d.Handle)
	copy(p[2:], d.Data)

	return len(d.Data) + 2, nil
}

// Notification is a notification or indication from a remote device.
type Notification struct {
	ConnectionHandle uint16
	Handle           uint16
	Data             []byte
}

type AttributeType int

const (
	AttributeTypeService AttributeType = iota
	AttributeTypeCharacteristic
	AttributeTypeCharacteristicValue
	AttributeTypeDescriptor
)

type attribute struct {
	typ         AttributeType
	parent      uint16
	handle      uint16
	uuid        ble.UUID
	permissions uint8
	value       []byte
}

func (a *attribute) unmarshal(buf []byte) (int, error) {
	return 0, errNotImplemented
}

func (a *attribute) marshal(p []byte) (int, error) {
	binary.LittleEndian.PutUint16(p[0:], a.handle)
	sz := 2

	switch a.typ {
	case AttributeTypeCharacteristicValue, AttributeTypeDescriptor:
		switch {
		case a.uuid.Is16Bit():
			binary.LittleEndian.PutUint16(p[sz:], a.uuid.Get16Bit())
			sz += 2
		default:
			uuid := a.uuid.BytesLittleEndian()
			copy(p[sz:], uuid[:])
			sz += 16
		}
	default:
		copy(p[sz:], a.value)
		sz += len(a.value)
	}

	return sz, nil
}

func (a *attribute) length() int {
	switch a.typ {
	case AttributeTypeCharacteristicValue, AttributeTypeDescriptor:
		switch {
		case a.uuid.Is16Bit():
			return 2
		default:
			return 16
		}
	default:
		return len(a.value)
	}
}

// ConnectionData holds what has been discovered on one connection, and the
// value of the last read. The caller clears the slices when it has consumed
// them.
type ConnectionData struct {
	MTU             uint16
	Services        []Service
	Characteristics []Characteristic
	Descriptors     []Descriptor
	Value           []byte

	responded       bool
	errored         bool
	lastErrorOpcode uint8
	lastErrorHandle uint16
	lastErrorCode   ble.AttributeProtocolError
	maxMTU          uint16
}

// ATT is the Attribute Protocol layer.
type ATT struct {
	hci           *HCI
	busy          sync.Mutex
	mtu           uint16
	maxMTU        uint16
	notifications chan Notification

	connections          []uint16
	connectionsData      map[uint16]*ConnectionData
	lastHandle           uint16
	localServices        []Service
	localCharacteristics []Characteristic
	attributes           []attribute
}

func newATT(hci *HCI) *ATT {
	return &ATT{
		hci:                  hci,
		localCharacteristics: []Characteristic{},
		notifications:        make(chan Notification, 32),
		connections:          []uint16{},
		connectionsData:      make(map[uint16]*ConnectionData),
		lastHandle:           0x0001,
		attributes:           []attribute{},
		localServices:        []Service{},
		maxMTU:               MaximumMTU,
	}
}

func (a *ATT) ReadByGroupReq(connectionHandle, startHandle, endHandle uint16, uuid ShortUUID) error {
	if debug {
		println("att.readByGroupReq:", connectionHandle, startHandle, endHandle, uuid)
	}

	a.busy.Lock()
	defer a.busy.Unlock()

	var b [7]byte
	b[0] = OpReadByGroupReq
	binary.LittleEndian.PutUint16(b[1:], startHandle)
	binary.LittleEndian.PutUint16(b[3:], endHandle)
	binary.LittleEndian.PutUint16(b[5:], uint16(uuid))

	if err := a.sendReq(connectionHandle, b[:]); err != nil {
		return err
	}

	return a.waitUntilResponse(connectionHandle)
}

func (a *ATT) ReadByTypeReq(connectionHandle, startHandle, endHandle uint16, typ ShortUUID) error {
	if debug {
		println("att.readByTypeReq:", connectionHandle, startHandle, endHandle, typ)
	}

	a.busy.Lock()
	defer a.busy.Unlock()

	var b [7]byte
	b[0] = OpReadByTypeReq
	binary.LittleEndian.PutUint16(b[1:], startHandle)
	binary.LittleEndian.PutUint16(b[3:], endHandle)
	binary.LittleEndian.PutUint16(b[5:], uint16(typ))

	if err := a.sendReq(connectionHandle, b[:]); err != nil {
		return err
	}

	return a.waitUntilResponse(connectionHandle)
}

func (a *ATT) FindInfoReq(connectionHandle, startHandle, endHandle uint16) error {
	if debug {
		println("att.findInfoReq:", connectionHandle, startHandle, endHandle)
	}

	a.busy.Lock()
	defer a.busy.Unlock()

	var b [5]byte
	b[0] = OpFindInfoReq
	binary.LittleEndian.PutUint16(b[1:], startHandle)
	binary.LittleEndian.PutUint16(b[3:], endHandle)

	if err := a.sendReq(connectionHandle, b[:]); err != nil {
		return err
	}

	return a.waitUntilResponse(connectionHandle)
}

func (a *ATT) ReadReq(connectionHandle, valueHandle uint16) error {
	if debug {
		println("att.readReq:", connectionHandle, valueHandle)
	}

	a.busy.Lock()
	defer a.busy.Unlock()

	var b [3]byte
	b[0] = OpReadReq
	binary.LittleEndian.PutUint16(b[1:], valueHandle)

	if err := a.sendReq(connectionHandle, b[:]); err != nil {
		return err
	}

	return a.waitUntilResponse(connectionHandle)
}

func (a *ATT) WriteCmd(connectionHandle, valueHandle uint16, data []byte) error {
	if debug {
		println("att.writeCmd:", connectionHandle, valueHandle, hex.EncodeToString(data))
	}

	a.busy.Lock()
	defer a.busy.Unlock()

	var b [3]byte
	b[0] = OpWriteCmd
	binary.LittleEndian.PutUint16(b[1:], valueHandle)

	if err := a.sendReq(connectionHandle, append(b[:], data...)); err != nil {
		return err
	}

	return nil
}

func (a *ATT) WriteReq(connectionHandle, valueHandle uint16, data []byte) error {
	if debug {
		println("att.writeReq:", connectionHandle, valueHandle, hex.EncodeToString(data))
	}

	a.busy.Lock()
	defer a.busy.Unlock()

	var b [3]byte
	b[0] = OpWriteReq
	binary.LittleEndian.PutUint16(b[1:], valueHandle)

	if err := a.sendReq(connectionHandle, append(b[:], data...)); err != nil {
		return err
	}

	return a.waitUntilResponse(connectionHandle)
}

func (a *ATT) MTUReq(connectionHandle uint16) error {
	if debug {
		println("att.mtuReq:", connectionHandle)
	}

	cd, err := a.ConnectionData(connectionHandle)
	if err != nil {
		return err
	}

	a.busy.Lock()
	defer a.busy.Unlock()

	var b [3]byte
	b[0] = OpMTUReq
	binary.LittleEndian.PutUint16(b[1:], cd.MTU)

	if err := a.sendReq(connectionHandle, b[:]); err != nil {
		return err
	}

	return a.waitUntilResponse(connectionHandle)
}

// MTU returns the MTU negotiated for the most recent exchange.
func (a *ATT) MTU() uint16 {
	return a.mtu
}

// Notifications returns the channel that carries notifications and
// indications from remote devices. It is buffered, and a notification is
// dropped when it is full.
func (a *ATT) Notifications() <-chan Notification {
	return a.notifications
}

// SetMaxMTU sets the largest MTU that this stack accepts.
func (a *ATT) SetMaxMTU(mtu uint16) error {
	a.maxMTU = mtu

	return nil
}

func (a *ATT) sendReq(handle uint16, data []byte) error {
	if err := a.clearResponse(handle); err != nil {
		return err
	}

	if debug {
		println("att.sendReq:", handle, "data:", hex.EncodeToString(data))
	}

	if err := a.hci.SendACLPacket(handle, CIDATT, data); err != nil {
		return err
	}

	return nil
}

func (a *ATT) SendNotification(handle uint16, data []byte) error {
	if debug {
		println("att.sendNotifications:", handle, "data:", hex.EncodeToString(data))
	}

	a.busy.Lock()
	defer a.busy.Unlock()

	var b [3]byte
	b[0] = OpHandleNotify
	binary.LittleEndian.PutUint16(b[1:], handle)

	for _, connection := range a.connections {
		if debug {
			println("att.sendNotifications: sending to", connection)
		}

		if err := a.hci.SendACLPacket(connection, CIDATT, append(b[:], data...)); err != nil {
			return err
		}
	}

	return nil
}

func (a *ATT) sendError(handle uint16, opcode uint8, hdl uint16, code ble.AttributeProtocolError) error {
	if err := a.clearResponse(handle); err != nil {
		return err
	}

	if debug {
		println("att.sendError:", handle, "data:", opcode, hdl, code.Code())
	}

	var b [5]byte
	b[0] = OpError
	b[1] = opcode
	binary.LittleEndian.PutUint16(b[2:], hdl)
	b[4] = code.Code()

	if err := a.hci.SendACLPacket(handle, CIDATT, b[:]); err != nil {
		return err
	}

	return nil
}

func (a *ATT) handleData(handle uint16, buf []byte) error {
	if debug {
		println("att.handleData:", handle, "data:", hex.EncodeToString(buf))
	}

	cd, err := a.ConnectionData(handle)
	if err != nil {
		return err
	}

	switch buf[0] {
	case OpError:
		cd.errored = true
		cd.lastErrorOpcode = buf[1]
		cd.lastErrorHandle = binary.LittleEndian.Uint16(buf[2:])
		cd.lastErrorCode = ble.AttributeProtocolError(buf[4])

		if debug {
			println("att.handleData: OpERROR", handle, cd.lastErrorOpcode, cd.lastErrorCode)
		}

		switch cd.lastErrorCode {
		case ble.ErrAttNotFound:
			return ErrATTAttributeNotFound
		default:
			return ErrATTOp
		}

	case OpMTUReq:
		if debug {
			println("att.handleData: OpMTUReq", hex.EncodeToString(buf))
		}
		mtu := binary.LittleEndian.Uint16(buf[1:])
		if mtu > a.maxMTU {
			mtu = a.maxMTU
		}

		// save mtu for connection
		cd.MTU = mtu

		var b [3]byte
		b[0] = OpMTUResponse
		binary.LittleEndian.PutUint16(b[1:], mtu)

		if err := a.hci.SendACLPacket(handle, CIDATT, b[:]); err != nil {
			return err
		}

	case OpMTUResponse:
		if debug {
			println("att.handleData: OpMTUResponse")
		}
		cd.responded = true
		cd.MTU = binary.LittleEndian.Uint16(buf[1:])

	case OpFindInfoReq:
		if debug {
			println("att.handleData: OpFindInfoReq")
		}

		startHandle := binary.LittleEndian.Uint16(buf[1:])
		endHandle := binary.LittleEndian.Uint16(buf[3:])

		return a.handleFindInfoReq(handle, startHandle, endHandle)

	case OpFindInfoResponse:
		if debug {
			println("att.handleData: OpFindInfoResponse")
		}
		cd.responded = true

		lengthPerDescriptor := int(buf[1])

		for i := 2; i < len(buf); i += lengthPerDescriptor {
			d := Descriptor{}
			d.unmarshal(buf[i : i+lengthPerDescriptor])

			if debug {
				println("att.handleData: descriptor", d.Handle, hex.EncodeToString(d.Data))
			}

			cd.Descriptors = append(cd.Descriptors, d)
		}

	case OpFindByTypeReq:
		if debug {
			println("att.handleData: OpFindByTypeReq")
		}

	case OpReadByTypeReq:
		if debug {
			println("att.handleData: OpReadByTypeReq")
		}

		startHandle := binary.LittleEndian.Uint16(buf[1:])
		endHandle := binary.LittleEndian.Uint16(buf[3:])
		uuid := ShortUUID(binary.LittleEndian.Uint16(buf[5:]))

		return a.handleReadByTypeReq(handle, startHandle, endHandle, uuid)

	case OpReadByTypeResponse:
		if debug {
			println("att.handleData: OpReadByTypeResponse")
		}
		cd.responded = true

		lengthPerCharacteristic := int(buf[1])

		for i := 2; i < len(buf); i += lengthPerCharacteristic {
			c := Characteristic{}
			c.unmarshal(buf[i : i+lengthPerCharacteristic])

			if debug {
				println("att.handleData: characteristic", c.StartHandle, c.Properties, c.ValueHandle, c.UUID.String())
			}

			cd.Characteristics = append(cd.Characteristics, c)
		}

		return nil

	case OpReadByGroupReq:
		if debug {
			println("att.handleData: OpReadByGroupReq")
		}

		startHandle := binary.LittleEndian.Uint16(buf[1:])
		endHandle := binary.LittleEndian.Uint16(buf[3:])
		uuid := ShortUUID(binary.LittleEndian.Uint16(buf[5:]))

		return a.handleReadByGroupReq(handle, startHandle, endHandle, uuid)

	case OpReadByGroupResponse:
		if debug {
			println("att.handleData: OpReadByGroupResponse")
		}
		cd.responded = true

		lengthPerService := int(buf[1])

		for i := 2; i < len(buf); i += lengthPerService {
			service := Service{}
			service.unmarshal(buf[i : i+lengthPerService])

			if debug {
				println("att.handleData: service", service.StartHandle, service.EndHandle, service.UUID.String())
			}

			cd.Services = append(cd.Services, service)
		}

		return nil

	case OpReadReq:
		if debug {
			println("att.handleData: OpReadReq")
		}

		attrHandle := binary.LittleEndian.Uint16(buf[1:])
		return a.handleReadReq(handle, attrHandle)

	case OpReadBlobReq:
		if debug {
			println("att.handleData: OpReadBlobReq")
		}

	case OpReadResponse:
		if debug {
			println("att.handleData: OpReadResponse")
		}
		cd.responded = true
		cd.Value = append(cd.Value, buf[1:]...)

	case OpWriteReq:
		if debug {
			println("att.handleData: OpWriteReq")
		}

		attrHandle := binary.LittleEndian.Uint16(buf[1:])
		return a.handleWriteReq(handle, attrHandle, buf[3:])

	case OpWriteCmd:
		if debug {
			println("att.handleData: OpWriteCmd")
		}

	case OpWriteResponse:
		if debug {
			println("att.handleData: OpWriteResponse")
		}
		cd.responded = true

	case OpPrepWriteReq:
		if debug {
			println("att.handleData: OpPrepWriteReq")
		}

	case OpExecWriteReq:
		if debug {
			println("att.handleData: OpExecWriteReq")
		}

	case OpHandleNotify:
		if debug {
			println("att.handleData: OpHandleNotify")
		}

		not := Notification{
			ConnectionHandle: handle,
			Handle:           binary.LittleEndian.Uint16(buf[1:]),
			Data:             []byte{},
		}
		not.Data = append(not.Data, buf[3:]...)

		select {
		case a.notifications <- not:
		default:
			// out of space, drop notification :(
		}

	case OpHandleInd:
		if debug {
			println("att.handleData: OpHandleInd")
		}

	case OpHandleCNF:
		if debug {
			println("att.handleData: OpHandleCNF")
		}

	case OpReadMultiReq:
		if debug {
			println("att.handleData: OpReadMultiReq")
		}

	case OpSignedWriteCmd:
		if debug {
			println("att.handleData: OpSignedWriteCmd")
		}

	default:
		if debug {
			println("att.handleData: unknown")
		}
	}

	return nil
}

func (a *ATT) handleReadByGroupReq(handle, start, end uint16, uuid ShortUUID) error {
	var response [maximumMTUBufferLen]byte
	response[0] = OpReadByGroupResponse
	response[1] = 0x0 // length per service
	pos := 2

	switch uuid {
	case ShortUUID(UUIDPrimaryService):
		for _, s := range a.localServices {
			if s.StartHandle >= start && s.EndHandle <= end {
				if debug {
					println("OpReadByGroupReq: replying with service", s.StartHandle, s.EndHandle, s.UUID.String())
				}

				length := 20
				if s.UUID.Is16Bit() {
					length = 6
				}

				if response[1] == 0 {
					response[1] = byte(length)
				} else if response[1] != byte(length) {
					// change of ble.UUID size
					break
				}

				s.marshal(response[pos : pos+length])
				pos += length

				if uint16(pos+length) > a.mtu {
					break
				}
			}
		}

		switch {
		case pos > 2:
			if err := a.hci.SendACLPacket(handle, CIDATT, response[:pos]); err != nil {
				return err
			}
		default:
			if err := a.sendError(handle, OpReadByGroupReq, start, ble.ErrAttNotFound); err != nil {
				return err
			}
		}

		return nil

	default:
		if debug {
			println("handleReadByGroupReq: unknown uuid", ble.New16BitUUID(uint16(uuid)).String())
		}
		if err := a.sendError(handle, OpReadByGroupReq, start, ble.ErrAttNotFound); err != nil {
			return err
		}

		return nil
	}
}

func (a *ATT) handleReadByTypeReq(handle, start, end uint16, uuid ShortUUID) error {
	var response [maximumMTUBufferLen]byte
	response[0] = OpReadByTypeResponse
	pos := 0

	switch uuid {
	case ShortUUID(UUIDCharacteristic):
		pos = 2
		response[1] = 0

		for _, c := range a.localCharacteristics {
			if debug {
				println("handleReadByTypeReq: looking at characteristic", c.StartHandle, c.UUID.String())
			}

			if c.StartHandle >= start && c.ValueHandle <= end {
				if debug {
					println("handleReadByTypeReq: replying with characteristic", c.StartHandle, c.UUID.String())
				}

				length := 21
				if c.UUID.Is16Bit() {
					length = 7
				}

				if response[1] == 0 {
					response[1] = byte(length)
				} else if response[1] != byte(length) {
					// change of ble.UUID size
					break
				}

				c.marshal(response[pos : pos+length])
				pos += length

				if uint16(pos+length) > a.mtu {
					break
				}
			}
		}
		switch {
		case pos > 2:
			if err := a.hci.SendACLPacket(handle, CIDATT, response[:pos]); err != nil {
				return err
			}
		default:
			if err := a.sendError(handle, OpReadByTypeReq, start, ble.ErrAttNotFound); err != nil {
				return err
			}
		}

		return nil

	default:
		if debug {
			println("handleReadByTypeReq: unknown uuid", ble.New16BitUUID(uint16(uuid)).String())
		}
		if err := a.sendError(handle, OpReadByTypeReq, start, ble.ErrAttNotFound); err != nil {
			return err
		}

		return nil
	}
}

func (a *ATT) handleFindInfoReq(handle, start, end uint16) error {
	var response [maximumMTUBufferLen]byte
	response[0] = OpFindInfoResponse
	pos := 0

	pos = 2
	infoType := 0
	response[1] = 0

	for _, attr := range a.attributes {
		if debug {
			println("handleFindInfoReq: looking at attribute")
		}

		if attr.handle >= start && attr.handle <= end {
			if debug {
				println("handleFindInfoReq: replying with attribute", attr.handle, attr.uuid.String(), attr.typ)
			}

			if attr.typ == AttributeTypeCharacteristicValue || attr.typ == AttributeTypeDescriptor {
				infoType = 1
			} else {
				infoType = 2
			}

			length := attr.length() + 2
			if response[1] == 0 {
				response[1] = byte(infoType)
			} else if response[1] != byte(infoType) {
				// change of info type
				break
			}

			attr.marshal(response[pos : pos+length])
			pos += length

			if uint16(pos+length) >= a.mtu {
				break
			}
		}
	}
	switch {
	case pos > 2:
		if err := a.hci.SendACLPacket(handle, CIDATT, response[:pos]); err != nil {
			return err
		}
	default:
		if err := a.sendError(handle, OpFindInfoReq, start, ble.ErrAttNotFound); err != nil {
			return err
		}
	}

	return nil
}

func (a *ATT) handleReadReq(handle, attrHandle uint16) error {
	attr := a.findAttribute(attrHandle)
	if attr == nil {
		if debug {
			println("att.handleReadReq: attribute not found", attrHandle)
		}
		return a.sendError(handle, OpReadReq, attrHandle, ble.ErrAttNotFound)
	}

	var response [maximumMTUBufferLen]byte
	response[0] = OpReadResponse
	pos := 1

	switch attr.typ {
	case AttributeTypeCharacteristicValue:
		if debug {
			println("att.handleReadReq: reading characteristic value", attrHandle)
		}

		c := a.findCharacteristic(attr.parent)
		if c != nil && c.handler.ReadValue != nil {
			value, err := c.handler.ReadValue()
			if err != nil {
				return a.sendError(handle, OpReadReq, attrHandle, ble.ErrAttReadNotPermitted)
			}

			copy(response[pos:], value)
			pos += len(value)

			if err := a.hci.SendACLPacket(handle, CIDATT, response[:pos]); err != nil {
				return err
			}

			return nil
		}

	case AttributeTypeDescriptor:
		if debug {
			println("att.handleReadReq: reading descriptor", attrHandle)
		}

		c := a.findCharacteristic(attr.parent)
		if c != nil && c.handler.ReadCCCD != nil {
			cccd, err := c.handler.ReadCCCD()
			if err != nil {
				return a.sendError(handle, OpReadReq, attrHandle, ble.ErrAttReadNotPermitted)
			}

			binary.LittleEndian.PutUint16(response[pos:], cccd)
			pos += 2

			if err := a.hci.SendACLPacket(handle, CIDATT, response[:pos]); err != nil {
				return err
			}

			return nil
		}
	}

	return a.sendError(handle, OpReadReq, attrHandle, ble.ErrAttReadNotPermitted)
}

func (a *ATT) handleWriteReq(handle, attrHandle uint16, data []byte) error {
	attr := a.findAttribute(attrHandle)
	if attr == nil {
		if debug {
			println("att.handleWriteReq: attribute not found", attrHandle)
		}
		return a.sendError(handle, OpWriteReq, attrHandle, ble.ErrAttNotFound)
	}

	switch attr.typ {
	case AttributeTypeCharacteristicValue:
		if debug {
			println("att.handleWriteReq: writing characteristic value", attrHandle, hex.EncodeToString(data))
		}

		c := a.findCharacteristic(attr.parent)
		if c != nil && c.handler.WriteValue != nil {
			if _, err := c.handler.WriteValue(data); err != nil {
				return a.sendError(handle, OpWriteReq, attrHandle, ble.ErrAttWriteNotPermitted)
			}

			if err := a.hci.SendACLPacket(handle, CIDATT, []byte{OpWriteResponse}); err != nil {
				return err
			}

			return nil
		}

	case AttributeTypeDescriptor:
		if debug {
			println("att.handleWriteReq: writing descriptor", attrHandle, hex.EncodeToString(data))
		}

		c := a.findCharacteristic(attr.parent)
		if c != nil && c.handler.WriteCCCD != nil {
			if err := c.handler.WriteCCCD(binary.LittleEndian.Uint16(data)); err != nil {
				return a.sendError(handle, OpWriteReq, attrHandle, ble.ErrAttWriteNotPermitted)
			}

			if err := a.hci.SendACLPacket(handle, CIDATT, []byte{OpWriteResponse}); err != nil {
				return err
			}

			return nil

		}
	}

	return a.sendError(handle, OpWriteReq, attrHandle, ble.ErrAttWriteNotPermitted)
}

func (a *ATT) clearResponse(handle uint16) error {
	cd, err := a.ConnectionData(handle)
	if err != nil {
		return err
	}

	cd.responded = false
	cd.errored = false
	cd.lastErrorOpcode = 0
	cd.lastErrorHandle = 0
	cd.lastErrorCode = 0
	cd.Value = []byte{}

	return nil
}

func (a *ATT) waitUntilResponse(handle uint16) error {
	cd, err := a.ConnectionData(handle)
	if err != nil {
		return err
	}

	start := time.Now().UnixNano()
	for {
		if err := a.hci.Poll(); err != nil {
			return err
		}

		switch {
		case cd.responded:
			return nil

		case (time.Now().UnixNano()-start)/int64(time.Second) > defaultTimeoutSeconds:
			return ErrATTTimeout

		default:
			time.Sleep(5 * time.Millisecond)
		}
	}
}

func (a *ATT) Poll() error {
	a.busy.Lock()
	defer a.busy.Unlock()

	if err := a.hci.Poll(); err != nil {
		return err
	}

	return nil
}

func (a *ATT) addConnection(handle uint16) error {
	if debug {
		println("att.addConnection:", handle)
	}
	a.connections = append(a.connections, handle)
	a.connectionsData[handle] = &ConnectionData{
		Services:        []Service{},
		Characteristics: []Characteristic{},
		Value:           []byte{},
	}

	return nil
}

func (a *ATT) removeConnection(handle uint16) error {
	if debug {
		println("att.removeConnection:", handle)
	}

	for i := range a.connections {
		if a.connections[i] == handle {
			a.connections = append(a.connections[:i], a.connections[i+1:]...)
			delete(a.connectionsData, handle)
			break
		}
	}

	return nil
}

func (a *ATT) AddLocalAttribute(typ AttributeType, parent uint16, uuid ble.UUID, permissions uint8, value []byte) uint16 {
	handle := a.lastHandle
	a.attributes = append(a.attributes,
		attribute{
			typ:         typ,
			parent:      parent,
			handle:      handle,
			uuid:        uuid,
			permissions: permissions,
			value:       append([]byte{}, value...),
		})
	a.lastHandle++

	return handle
}

func (a *ATT) AddLocalService(start, end uint16, uuid ble.UUID) {
	a.localServices = append(a.localServices, Service{
		StartHandle: start,
		EndHandle:   end,
		UUID:        uuid,
	})
}

func (a *ATT) AddLocalCharacteristic(startHandle uint16, properties uint8, valueHandle uint16, uuid ble.UUID, handler CharacteristicHandler) {
	a.localCharacteristics = append(a.localCharacteristics,
		Characteristic{
			StartHandle: startHandle,
			Properties:  properties,
			ValueHandle: valueHandle,
			UUID:        uuid,
			handler:     handler,
		})
}

func (a *ATT) ClearLocalData() {
	a.attributes = []attribute{}
	a.localServices = []Service{}
	a.localCharacteristics = []Characteristic{}
}

func (a *ATT) findAttribute(hdl uint16) *attribute {
	for i := range a.attributes {
		if a.attributes[i].handle == hdl {
			return &a.attributes[i]
		}
	}

	return nil
}

func (a *ATT) findCharacteristic(hdl uint16) *Characteristic {
	for i := range a.localCharacteristics {
		if a.localCharacteristics[i].StartHandle == hdl {
			return &a.localCharacteristics[i]
		}
	}

	return nil
}

func (a *ATT) ConnectionData(handle uint16) (*ConnectionData, error) {
	cd, ok := a.connectionsData[handle]
	if !ok {
		return nil, ErrATTUnknownConnection
	}

	return cd, nil
}

// AttributeError is the content of an ATT Error Response.
type AttributeError struct {
	Opcode uint8
	Handle uint16
	Code   ble.AttributeProtocolError
}

// LastError returns the most recent ATT Error Response on a connection, and
// whether one has arrived.
func (a *ATT) LastError(handle uint16) (AttributeError, bool) {
	cd, err := a.ConnectionData(handle)
	if err != nil {
		return AttributeError{}, false
	}

	if cd.lastErrorOpcode == 0 {
		return AttributeError{}, false
	}

	return AttributeError{
		Opcode: cd.lastErrorOpcode,
		Handle: cd.lastErrorHandle,
		Code:   cd.lastErrorCode,
	}, true
}
