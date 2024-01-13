//go:build ninafw

package bluetooth

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
	"slices"
	"sync"
	"time"
)

const (
	attCID = 0x0004
	bleCTL = 0x0008

	attOpError               = 0x01
	attOpMTUReq              = 0x02
	attOpMTUResponse         = 0x03
	attOpFindInfoReq         = 0x04
	attOpFindInfoResponse    = 0x05
	attOpFindByTypeReq       = 0x06
	attOpFindByTypeResponse  = 0x07
	attOpReadByTypeReq       = 0x08
	attOpReadByTypeResponse  = 0x09
	attOpReadReq             = 0x0a
	attOpReadResponse        = 0x0b
	attOpReadBlobReq         = 0x0c
	attOpReadBlobResponse    = 0x0d
	attOpReadMultiReq        = 0x0e
	attOpReadMultiResponse   = 0x0f
	attOpReadByGroupReq      = 0x10
	attOpReadByGroupResponse = 0x11
	attOpWriteReq            = 0x12
	attOpWriteResponse       = 0x13
	attOpWriteCmd            = 0x52
	attOpPrepWriteReq        = 0x16
	attOpPrepWriteResponse   = 0x17
	attOpExecWriteReq        = 0x18
	attOpExecWriteResponse   = 0x19
	attOpHandleNotify        = 0x1b
	attOpHandleInd           = 0x1d
	attOpHandleCNF           = 0x1e
	attOpSignedWriteCmd      = 0xd2

	attErrorInvalidHandle          = 0x01
	attErrorReadNotPermitted       = 0x02
	attErrorWriteNotPermitted      = 0x03
	attErrorInvalidPDU             = 0x04
	attErrorAuthentication         = 0x05
	attErrorRequestNotSupported    = 0x06
	attErrorInvalidOffset          = 0x07
	attErrorAuthorization          = 0x08
	attErrorPreQueueFull           = 0x09
	attErrorAttrNotFound           = 0x0a
	attErrorAttrNotLong            = 0x0b
	attErrorInsuffEncrKeySize      = 0x0c
	attErrorInvalidAttrValueLength = 0x0d
	attErrorUnlikely               = 0x0e
	attErrorInsuffEnc              = 0x0f
	attErrorUnsupportedGroupType   = 0x10
	attErrorInsufficientResources  = 0x11

	gattUnknownUUID                    = 0x0000
	gattServiceUUID                    = 0x2800
	gattCharacteristicUUID             = 0x2803
	gattDescriptorUUID                 = 0x2900
	gattClientCharacteristicConfigUUID = 0x2902
)

var (
	ErrATTTimeout      = errors.New("bluetooth: ATT timeout")
	ErrATTUnknownEvent = errors.New("bluetooth: ATT unknown event")
	ErrATTUnknown      = errors.New("bluetooth: ATT unknown error")
	ErrATTOp           = errors.New("bluetooth: ATT OP error")
)

type rawService struct {
	startHandle uint16
	endHandle   uint16
	uuid        UUID
}

func (s *rawService) Write(buf []byte) (int, error) {
	s.startHandle = binary.LittleEndian.Uint16(buf[0:])
	s.endHandle = binary.LittleEndian.Uint16(buf[2:])

	sz := 4
	switch len(buf) - 4 {
	case 2:
		s.uuid = New16BitUUID(binary.LittleEndian.Uint16(buf[4:]))
		sz += 2
	case 16:
		var uuid [16]byte
		copy(uuid[:], buf[4:])
		slices.Reverse(uuid[:])
		s.uuid = NewUUID(uuid)
		sz += 16
	}

	return sz, nil
}

func (s *rawService) Read(p []byte) (int, error) {
	binary.LittleEndian.PutUint16(p[0:], s.startHandle)
	binary.LittleEndian.PutUint16(p[2:], s.endHandle)

	sz := 4
	switch {
	case s.uuid.Is16Bit():
		binary.LittleEndian.PutUint16(p[4:], s.uuid.Get16Bit())
		sz += 2
	default:
		uuid := s.uuid.Bytes()
		copy(p[4:], uuid[:])
		sz += 16
	}

	return sz, nil
}

type rawCharacteristic struct {
	startHandle uint16
	properties  uint8
	valueHandle uint16
	uuid        UUID
	chr         *Characteristic
}

func (c *rawCharacteristic) Write(buf []byte) (int, error) {
	c.startHandle = binary.LittleEndian.Uint16(buf[0:])
	c.properties = buf[2]
	c.valueHandle = binary.LittleEndian.Uint16(buf[3:])

	sz := 5
	switch len(buf) - 5 {
	case 2:
		c.uuid = New16BitUUID(binary.LittleEndian.Uint16(buf[5:]))
		sz += 2
	case 16:
		var uuid [16]byte
		copy(uuid[:], buf[5:])
		slices.Reverse(uuid[:])
		c.uuid = NewUUID(uuid)
		sz += 16
	}

	return sz, nil
}

func (c *rawCharacteristic) Read(p []byte) (int, error) {
	binary.LittleEndian.PutUint16(p[0:], c.startHandle)
	p[2] = c.properties
	binary.LittleEndian.PutUint16(p[3:], c.valueHandle)

	sz := 5
	switch {
	case c.uuid.Is16Bit():
		binary.LittleEndian.PutUint16(p[5:], c.uuid.Get16Bit())
		sz += 2
	default:
		uuid := c.uuid.Bytes()
		copy(p[5:], uuid[:])
		sz += 16
	}

	return sz, nil
}

type rawDescriptor struct {
	handle uint16
	data   []byte
}

func (d *rawDescriptor) Write(buf []byte) (int, error) {
	d.handle = binary.LittleEndian.Uint16(buf[0:])
	d.data = append(d.data, buf[2:]...)

	return len(d.data) + 2, nil
}

func (d *rawDescriptor) Read(p []byte) (int, error) {
	binary.LittleEndian.PutUint16(p[0:], d.handle)
	copy(p[2:], d.data)

	return len(d.data) + 2, nil
}

type rawNotification struct {
	connectionHandle uint16
	handle           uint16
	data             []byte
}

type attributeType int

const (
	attributeTypeService attributeType = iota
	attributeTypeCharacteristic
	attributeTypeCharacteristicValue
	attributeTypeDescriptor
)

type rawAttribute struct {
	typ         attributeType
	parent      uint16
	handle      uint16
	uuid        UUID
	permissions CharacteristicPermissions
	value       []byte
}

func (a *rawAttribute) Write(buf []byte) (int, error) {
	return 0, errNotYetImplemented
}

func (a *rawAttribute) Read(p []byte) (int, error) {
	binary.LittleEndian.PutUint16(p[0:], a.handle)
	sz := 2

	switch a.typ {
	case attributeTypeCharacteristicValue, attributeTypeDescriptor:
		switch {
		case a.uuid.Is16Bit():
			binary.LittleEndian.PutUint16(p[sz:], a.uuid.Get16Bit())
			sz += 2
		default:
			uuid := a.uuid.Bytes()
			copy(p[sz:], uuid[:])
			sz += 16
		}
	default:
		copy(p[sz:], a.value)
		sz += len(a.value)
	}

	return sz, nil
}

func (a *rawAttribute) length() int {
	switch a.typ {
	case attributeTypeCharacteristicValue, attributeTypeDescriptor:
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

type att struct {
	hci             *hci
	busy            sync.Mutex
	responded       bool
	errored         bool
	lastErrorOpcode uint8
	lastErrorHandle uint16
	lastErrorCode   uint8
	mtu             uint16
	services        []rawService
	characteristics []rawCharacteristic
	descriptors     []rawDescriptor
	value           []byte
	notifications   chan rawNotification

	connections   []uint16
	lastHandle    uint16
	attributes    []rawAttribute
	localServices []rawService
}

func newATT(hci *hci) *att {
	return &att{
		hci:             hci,
		services:        []rawService{},
		characteristics: []rawCharacteristic{},
		value:           []byte{},
		notifications:   make(chan rawNotification, 32),
		connections:     []uint16{},
		lastHandle:      0x0001,
		attributes:      []rawAttribute{},
		localServices:   []rawService{},
	}
}

func (a *att) readByGroupReq(connectionHandle, startHandle, endHandle uint16, uuid shortUUID) error {
	if _debug {
		println("att.readByGroupReq:", connectionHandle, startHandle, endHandle, uuid)
	}

	a.busy.Lock()
	defer a.busy.Unlock()

	var b [7]byte
	b[0] = attOpReadByGroupReq
	binary.LittleEndian.PutUint16(b[1:], startHandle)
	binary.LittleEndian.PutUint16(b[3:], endHandle)
	binary.LittleEndian.PutUint16(b[5:], uint16(uuid))

	if err := a.sendReq(connectionHandle, b[:]); err != nil {
		return err
	}

	return a.waitUntilResponse()
}

func (a *att) readByTypeReq(connectionHandle, startHandle, endHandle uint16, typ uint16) error {
	if _debug {
		println("att.readByTypeReq:", connectionHandle, startHandle, endHandle, typ)
	}

	a.busy.Lock()
	defer a.busy.Unlock()

	var b [7]byte
	b[0] = attOpReadByTypeReq
	binary.LittleEndian.PutUint16(b[1:], startHandle)
	binary.LittleEndian.PutUint16(b[3:], endHandle)
	binary.LittleEndian.PutUint16(b[5:], typ)

	if err := a.sendReq(connectionHandle, b[:]); err != nil {
		return err
	}

	return a.waitUntilResponse()
}

func (a *att) findInfoReq(connectionHandle, startHandle, endHandle uint16) error {
	if _debug {
		println("att.findInfoReq:", connectionHandle, startHandle, endHandle)
	}

	a.busy.Lock()
	defer a.busy.Unlock()

	var b [5]byte
	b[0] = attOpFindInfoReq
	binary.LittleEndian.PutUint16(b[1:], startHandle)
	binary.LittleEndian.PutUint16(b[3:], endHandle)

	if err := a.sendReq(connectionHandle, b[:]); err != nil {
		return err
	}

	return a.waitUntilResponse()
}

func (a *att) readReq(connectionHandle, valueHandle uint16) error {
	if _debug {
		println("att.readReq:", connectionHandle, valueHandle)
	}

	a.busy.Lock()
	defer a.busy.Unlock()

	var b [3]byte
	b[0] = attOpReadReq
	binary.LittleEndian.PutUint16(b[1:], valueHandle)

	if err := a.sendReq(connectionHandle, b[:]); err != nil {
		return err
	}

	return a.waitUntilResponse()
}

func (a *att) writeCmd(connectionHandle, valueHandle uint16, data []byte) error {
	if _debug {
		println("att.writeCmd:", connectionHandle, valueHandle, hex.EncodeToString(data))
	}

	a.busy.Lock()
	defer a.busy.Unlock()

	var b [3]byte
	b[0] = attOpWriteCmd
	binary.LittleEndian.PutUint16(b[1:], valueHandle)

	if err := a.sendReq(connectionHandle, append(b[:], data...)); err != nil {
		return err
	}

	return a.waitUntilResponse()
}

func (a *att) writeReq(connectionHandle, valueHandle uint16, data []byte) error {
	if _debug {
		println("att.writeReq:", connectionHandle, valueHandle, hex.EncodeToString(data))
	}

	a.busy.Lock()
	defer a.busy.Unlock()

	var b [3]byte
	b[0] = attOpWriteReq
	binary.LittleEndian.PutUint16(b[1:], valueHandle)

	if err := a.sendReq(connectionHandle, append(b[:], data...)); err != nil {
		return err
	}

	return a.waitUntilResponse()
}

func (a *att) mtuReq(connectionHandle, mtu uint16) error {
	if _debug {
		println("att.mtuReq:", connectionHandle)
	}

	a.busy.Lock()
	defer a.busy.Unlock()

	var b [3]byte
	b[0] = attOpMTUReq
	binary.LittleEndian.PutUint16(b[1:], mtu)

	if err := a.sendReq(connectionHandle, b[:]); err != nil {
		return err
	}

	return a.waitUntilResponse()
}

func (a *att) sendReq(handle uint16, data []byte) error {
	a.clearResponse()

	if _debug {
		println("att.sendReq:", handle, "data:", hex.EncodeToString(data))
	}

	if err := a.hci.sendAclPkt(handle, attCID, data); err != nil {
		return err
	}

	return nil
}

func (a *att) sendNotification(handle uint16, data []byte) error {
	if _debug {
		println("att.sendNotifications:", handle, "data:", hex.EncodeToString(data))
	}

	a.busy.Lock()
	defer a.busy.Unlock()

	var b [3]byte
	b[0] = attOpHandleNotify
	binary.LittleEndian.PutUint16(b[1:], handle)

	for connection := range a.connections {
		if _debug {
			println("att.sendNotifications: sending to", connection)
		}

		if err := a.hci.sendAclPkt(uint16(connection), attCID, append(b[:], data...)); err != nil {
			return err
		}
	}

	return nil
}

func (a *att) sendError(handle uint16, opcode uint8, hdl uint16, code uint8) error {
	a.clearResponse()

	if _debug {
		println("att.sendError:", handle, "data:", opcode, hdl, code)
	}

	var b [5]byte
	b[0] = attOpError
	b[1] = opcode
	binary.LittleEndian.PutUint16(b[2:], hdl)
	b[4] = code

	if err := a.hci.sendAclPkt(handle, attCID, b[:]); err != nil {
		return err
	}

	return nil
}

func (a *att) handleData(handle uint16, buf []byte) error {
	if _debug {
		println("att.handleData:", handle, "data:", hex.EncodeToString(buf))
	}

	switch buf[0] {
	case attOpError:
		a.errored = true
		a.lastErrorOpcode = buf[1]
		a.lastErrorHandle = binary.LittleEndian.Uint16(buf[2:])
		a.lastErrorCode = buf[4]

		if _debug {
			println("att.handleData: attOpERROR", a.lastErrorOpcode, a.lastErrorCode)
		}

		return ErrATTOp

	case attOpMTUReq:
		if _debug {
			println("att.handleData: attOpMTUReq")
		}
		a.mtu = binary.LittleEndian.Uint16(buf[1:])
		response := [3]byte{attOpMTUResponse, buf[1], buf[2]}
		if err := a.hci.sendAclPkt(handle, attCID, response[:]); err != nil {
			return err
		}

	case attOpMTUResponse:
		if _debug {
			println("att.handleData: attOpMTUResponse")
		}
		a.responded = true
		a.mtu = binary.LittleEndian.Uint16(buf[1:])

	case attOpFindInfoReq:
		if _debug {
			println("att.handleData: attOpFindInfoReq")
		}

		startHandle := binary.LittleEndian.Uint16(buf[1:])
		endHandle := binary.LittleEndian.Uint16(buf[3:])

		return a.handleFindInfoReq(handle, startHandle, endHandle)

	case attOpFindInfoResponse:
		if _debug {
			println("att.handleData: attOpFindInfoResponse")
		}
		a.responded = true

		lengthPerDescriptor := int(buf[1])

		for i := 2; i < len(buf); i += lengthPerDescriptor {
			d := rawDescriptor{}
			d.Write(buf[i : i+lengthPerDescriptor])

			if _debug {
				println("att.handleData: descriptor", d.handle, hex.EncodeToString(d.data))
			}

			a.descriptors = append(a.descriptors, d)
		}

	case attOpFindByTypeReq:
		if _debug {
			println("att.handleData: attOpFindByTypeReq")
		}

	case attOpReadByTypeReq:
		if _debug {
			println("att.handleData: attOpReadByTypeReq")
		}

		startHandle := binary.LittleEndian.Uint16(buf[1:])
		endHandle := binary.LittleEndian.Uint16(buf[3:])
		uuid := shortUUID(binary.LittleEndian.Uint16(buf[5:]))

		return a.handleReadByTypeReq(handle, startHandle, endHandle, uuid)

	case attOpReadByTypeResponse:
		if _debug {
			println("att.handleData: attOpReadByTypeResponse")
		}
		a.responded = true

		lengthPerCharacteristic := int(buf[1])

		for i := 2; i < len(buf); i += lengthPerCharacteristic {
			c := rawCharacteristic{}
			c.Write(buf[i : i+lengthPerCharacteristic])

			if _debug {
				println("att.handleData: characteristic", c.startHandle, c.properties, c.valueHandle, c.uuid.String())
			}

			a.characteristics = append(a.characteristics, c)
		}

		return nil

	case attOpReadByGroupReq:
		if _debug {
			println("att.handleData: attOpReadByGroupReq")
		}

		startHandle := binary.LittleEndian.Uint16(buf[1:])
		endHandle := binary.LittleEndian.Uint16(buf[3:])
		uuid := shortUUID(binary.LittleEndian.Uint16(buf[5:]))

		return a.handleReadByGroupReq(handle, startHandle, endHandle, uuid)

	case attOpReadByGroupResponse:
		if _debug {
			println("att.handleData: attOpReadByGroupResponse")
		}
		a.responded = true

		lengthPerService := int(buf[1])

		for i := 2; i < len(buf); i += lengthPerService {
			service := rawService{}
			service.Write(buf[i : i+lengthPerService])

			if _debug {
				println("att.handleData: service", service.startHandle, service.endHandle, service.uuid.String())
			}

			a.services = append(a.services, service)
		}

		return nil

	case attOpReadReq:
		if _debug {
			println("att.handleData: attOpReadReq")
		}

		attrHandle := binary.LittleEndian.Uint16(buf[1:])
		return a.handleReadReq(handle, attrHandle)

	case attOpReadBlobReq:
		if _debug {
			println("att.handleData: attOpReadBlobReq")
		}

	case attOpReadResponse:
		if _debug {
			println("att.handleData: attOpReadResponse")
		}
		a.responded = true
		a.value = append(a.value, buf[1:]...)

	case attOpWriteReq:
		if _debug {
			println("att.handleData: attOpWriteReq")
		}

		attrHandle := binary.LittleEndian.Uint16(buf[1:])
		return a.handleWriteReq(handle, attrHandle, buf[3:])

	case attOpWriteCmd:
		if _debug {
			println("att.handleData: attOpWriteCmd")
		}

	case attOpWriteResponse:
		if _debug {
			println("att.handleData: attOpWriteResponse")
		}
		a.responded = true

	case attOpPrepWriteReq:
		if _debug {
			println("att.handleData: attOpPrepWriteReq")
		}

	case attOpExecWriteReq:
		if _debug {
			println("att.handleData: attOpExecWriteReq")
		}

	case attOpHandleNotify:
		if _debug {
			println("att.handleData: attOpHandleNotify")
		}

		not := rawNotification{
			connectionHandle: handle,
			handle:           binary.LittleEndian.Uint16(buf[1:]),
			data:             []byte{},
		}
		not.data = append(not.data, buf[3:]...)

		select {
		case a.notifications <- not:
		default:
			// out of space, drop notification :(
		}

	case attOpHandleInd:
		if _debug {
			println("att.handleData: attOpHandleInd")
		}

	case attOpHandleCNF:
		if _debug {
			println("att.handleData: attOpHandleCNF")
		}

	case attOpReadMultiReq:
		if _debug {
			println("att.handleData: attOpReadMultiReq")
		}

	case attOpSignedWriteCmd:
		if _debug {
			println("att.handleData: attOpSignedWriteCmd")
		}

	default:
		if _debug {
			println("att.handleData: unknown")
		}
	}

	return nil
}

func (a *att) handleReadByGroupReq(handle, start, end uint16, uuid shortUUID) error {
	var response [64]byte
	response[0] = attOpReadByGroupResponse
	response[1] = 0x0 // length per service
	pos := 2

	switch uuid {
	case shortUUID(gattServiceUUID):
		for _, s := range a.localServices {
			if s.startHandle >= start && s.endHandle <= end {
				if _debug {
					println("attOpReadByGroupReq: replying with service", s.startHandle, s.endHandle, s.uuid.String())
				}

				length := 20
				if s.uuid.Is16Bit() {
					length = 6
				}

				if response[1] == 0 {
					response[1] = byte(length)
				} else if response[1] != byte(length) {
					// change of UUID size
					break
				}

				s.Read(response[pos : pos+length])
				pos += length

				if uint16(pos+length) > a.mtu {
					break
				}
			}
		}

		switch {
		case pos > 2:
			if err := a.hci.sendAclPkt(handle, attCID, response[:pos]); err != nil {
				return err
			}
		default:
			if err := a.sendError(handle, attOpReadByGroupReq, start, attErrorAttrNotFound); err != nil {
				return err
			}
		}

		return nil

	default:
		if _debug {
			println("handleReadByGroupReq: unknown uuid", New16BitUUID(uint16(uuid)).String())
		}
		if err := a.sendError(handle, attOpReadByGroupReq, start, attErrorAttrNotFound); err != nil {
			return err
		}

		return nil
	}
}

func (a *att) handleReadByTypeReq(handle, start, end uint16, uuid shortUUID) error {
	var response [64]byte
	response[0] = attOpReadByTypeResponse
	pos := 0

	switch uuid {
	case shortUUID(gattCharacteristicUUID):
		pos = 2
		response[1] = 0

		for _, c := range a.characteristics {
			if _debug {
				println("handleReadByTypeReq: looking at characteristic", c.startHandle, c.uuid.String())
			}

			if c.startHandle >= start && c.valueHandle <= end {
				if _debug {
					println("handleReadByTypeReq: replying with characteristic", c.startHandle, c.uuid.String())
				}

				length := 21
				if c.uuid.Is16Bit() {
					length = 7
				}

				if response[1] == 0 {
					response[1] = byte(length)
				} else if response[1] != byte(length) {
					// change of UUID size
					break
				}

				c.Read(response[pos : pos+length])
				pos += length

				if uint16(pos+length) > a.mtu {
					break
				}
			}
		}
		switch {
		case pos > 2:
			if err := a.hci.sendAclPkt(handle, attCID, response[:pos]); err != nil {
				return err
			}
		default:
			if err := a.sendError(handle, attOpReadByTypeReq, start, attErrorAttrNotFound); err != nil {
				return err
			}
		}

		return nil

	default:
		if _debug {
			println("handleReadByTypeReq: unknown uuid", New16BitUUID(uint16(uuid)).String())
		}
		if err := a.sendError(handle, attOpReadByTypeReq, start, attErrorAttrNotFound); err != nil {
			return err
		}

		return nil
	}
}

func (a *att) handleFindInfoReq(handle, start, end uint16) error {
	var response [64]byte
	response[0] = attOpFindInfoResponse
	pos := 0

	pos = 2
	infoType := 0
	response[1] = 0

	for _, attr := range a.attributes {
		if _debug {
			println("handleFindInfoReq: looking at attribute")
		}

		if attr.handle >= start && attr.handle <= end {
			if _debug {
				println("handleFindInfoReq: replying with attribute", attr.handle, attr.uuid.String(), attr.typ)
			}

			if attr.typ == attributeTypeCharacteristicValue || attr.typ == attributeTypeDescriptor {
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

			attr.Read(response[pos : pos+length])
			pos += length

			if uint16(pos+length) >= a.mtu {
				break
			}
		}
	}
	switch {
	case pos > 2:
		if err := a.hci.sendAclPkt(handle, attCID, response[:pos]); err != nil {
			return err
		}
	default:
		if err := a.sendError(handle, attOpFindInfoReq, start, attErrorAttrNotFound); err != nil {
			return err
		}
	}

	return nil
}

func (a *att) handleReadReq(handle, attrHandle uint16) error {
	attr := a.findAttribute(attrHandle)
	if attr == nil {
		if _debug {
			println("att.handleReadReq: attribute not found", attrHandle)
		}
		return a.sendError(handle, attOpReadReq, attrHandle, attErrorAttrNotFound)
	}

	var response [64]byte
	response[0] = attOpReadResponse
	pos := 1

	switch attr.typ {
	case attributeTypeCharacteristicValue:
		if _debug {
			println("att.handleReadReq: reading characteristic value", attrHandle)
		}

		c := a.findCharacteristic(attr.parent)
		if c != nil && c.chr != nil {
			value, err := c.chr.readValue()
			if err != nil {
				return a.sendError(handle, attOpReadReq, attrHandle, attErrorReadNotPermitted)
			}

			copy(response[pos:], value)
			pos += len(value)

			if err := a.hci.sendAclPkt(handle, attCID, response[:pos]); err != nil {
				return err
			}
		}

	case attributeTypeDescriptor:
		if _debug {
			println("att.handleReadReq: reading descriptor", attrHandle)
		}

		c := a.findCharacteristic(attr.parent)
		if c != nil && c.chr != nil {
			cccd, err := c.chr.readCCCD()
			if err != nil {
				return a.sendError(handle, attOpReadReq, attrHandle, attErrorReadNotPermitted)
			}

			binary.LittleEndian.PutUint16(response[pos:], cccd)
			pos += 2

			if err := a.hci.sendAclPkt(handle, attCID, response[:pos]); err != nil {
				return err
			}
		}
	}

	return a.sendError(handle, attOpReadReq, attrHandle, attErrorReadNotPermitted)
}

func (a *att) handleWriteReq(handle, attrHandle uint16, data []byte) error {
	attr := a.findAttribute(attrHandle)
	if attr == nil {
		if _debug {
			println("att.handleWriteReq: attribute not found", attrHandle)
		}
		return a.sendError(handle, attOpWriteReq, attrHandle, attErrorAttrNotFound)
	}

	switch attr.typ {
	case attributeTypeCharacteristicValue:
		if _debug {
			println("att.handleWriteReq: writing characteristic value", attrHandle, hex.EncodeToString(data))
		}

		c := a.findCharacteristic(attr.parent)
		if c != nil && c.chr != nil {
			if _, err := c.chr.Write(data); err != nil {
				return a.sendError(handle, attOpWriteReq, attrHandle, attErrorWriteNotPermitted)
			}

			if err := a.hci.sendAclPkt(handle, attCID, []byte{attOpWriteResponse}); err != nil {
				return err
			}

			return nil
		}

	case attributeTypeDescriptor:
		if _debug {
			println("att.handleWriteReq: writing descriptor", attrHandle, hex.EncodeToString(data))
		}

		c := a.findCharacteristic(attr.parent)
		if c != nil && c.chr != nil {
			if err := c.chr.writeCCCD(binary.LittleEndian.Uint16(data)); err != nil {
				return a.sendError(handle, attOpWriteReq, attrHandle, attErrorWriteNotPermitted)
			}

			if err := a.hci.sendAclPkt(handle, attCID, []byte{attOpWriteResponse}); err != nil {
				return err
			}

			return nil

		}
	}

	return a.sendError(handle, attOpWriteReq, attrHandle, attErrorWriteNotPermitted)
}

func (a *att) clearResponse() {
	a.responded = false
	a.errored = false
	a.lastErrorOpcode = 0
	a.lastErrorHandle = 0
	a.lastErrorCode = 0
	a.value = []byte{}
}

func (a *att) waitUntilResponse() error {
	start := time.Now().UnixNano()
	for {
		if err := a.hci.poll(); err != nil {
			return err
		}

		switch {
		case a.responded:
			return nil

		default:
			// check for timeout
			if (time.Now().UnixNano()-start)/int64(time.Second) > 3 {
				break
			}

			time.Sleep(100 * time.Millisecond)
		}
	}

	return ErrATTTimeout
}

func (a *att) poll() error {
	a.busy.Lock()
	defer a.busy.Unlock()

	if err := a.hci.poll(); err != nil {
		return err
	}

	return nil
}

func (a *att) addConnection(handle uint16) {
	a.connections = append(a.connections, handle)
}

func (a *att) removeConnection(handle uint16) {
	for i := range a.connections {
		if a.connections[i] == handle {
			a.connections = append(a.connections[:i], a.connections[i+1:]...)
			return
		}
	}
}

func (a *att) addLocalAttribute(typ attributeType, parent uint16, uuid UUID, permissions CharacteristicPermissions, value []byte) uint16 {
	handle := a.lastHandle
	a.attributes = append(a.attributes,
		rawAttribute{
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

func (a *att) addLocalService(start, end uint16, uuid UUID) {
	a.localServices = append(a.localServices, rawService{
		startHandle: start,
		endHandle:   end,
		uuid:        uuid,
	})
}

func (a *att) addLocalCharacteristic(startHandle uint16, properties CharacteristicPermissions, valueHandle uint16, uuid UUID, chr *Characteristic) {
	a.characteristics = append(a.characteristics,
		rawCharacteristic{
			startHandle: startHandle,
			properties:  uint8(properties),
			valueHandle: valueHandle,
			uuid:        uuid,
			chr:         chr,
		})
}

func (a *att) findAttribute(hdl uint16) *rawAttribute {
	for i := range a.attributes {
		if a.attributes[i].handle == hdl {
			return &a.attributes[i]
		}
	}

	return nil
}

func (a *att) findCharacteristic(hdl uint16) *rawCharacteristic {
	for i := range a.characteristics {
		if a.characteristics[i].startHandle == hdl {
			return &a.characteristics[i]
		}
	}

	return nil
}
