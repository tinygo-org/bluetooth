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

	gattUnknownUUID        = 0x0000
	gattServiceUUID        = 0x2800
	gattCharacteristicUUID = 0x2803
	gattDescriptorUUID     = 0x2900
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

type rawCharacteristic struct {
	startHandle uint16
	properties  uint8
	valueHandle uint16
	uuid        UUID
}

type rawDescriptor struct {
	handle uint16
	uuid   UUID
}

type rawNotification struct {
	connectionHandle uint16
	handle           uint16
	data             []byte
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
}

func newATT(hci *hci) *att {
	return &att{
		hci:             hci,
		services:        []rawService{},
		characteristics: []rawCharacteristic{},
		value:           []byte{},
		notifications:   make(chan rawNotification, 32),
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

	case attOpFindInfoResponse:
		if _debug {
			println("att.handleData: attOpFindInfoResponse")
		}
		a.responded = true

		lengthPerDescriptor := int(buf[1])
		var uuid [16]byte

		for i := 2; i < len(buf); i += lengthPerDescriptor {
			d := rawDescriptor{
				handle: binary.LittleEndian.Uint16(buf[i:]),
			}
			switch lengthPerDescriptor - 2 {
			case 2:
				d.uuid = New16BitUUID(binary.LittleEndian.Uint16(buf[i+2:]))
			case 16:
				copy(uuid[:], buf[i+2:])
				slices.Reverse(uuid[:])
				d.uuid = NewUUID(uuid)
			}

			if _debug {
				println("att.handleData: descriptor", d.handle, d.uuid.String())
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

	case attOpReadByTypeResponse:
		if _debug {
			println("att.handleData: attOpReadByTypeResponse")
		}
		a.responded = true

		lengthPerCharacteristic := int(buf[1])
		var uuid [16]byte

		for i := 2; i < len(buf); i += lengthPerCharacteristic {
			c := rawCharacteristic{
				startHandle: binary.LittleEndian.Uint16(buf[i:]),
				properties:  buf[i+2],
				valueHandle: binary.LittleEndian.Uint16(buf[i+3:]),
			}
			switch lengthPerCharacteristic - 5 {
			case 2:
				c.uuid = New16BitUUID(binary.LittleEndian.Uint16(buf[i+5:]))
			case 16:
				copy(uuid[:], buf[i+5:])
				slices.Reverse(uuid[:])
				c.uuid = NewUUID(uuid)
			}

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

		// return generic services
		var response [14]byte
		response[0] = attOpReadByGroupResponse
		response[1] = 0x06 // length per service

		genericAccessService := rawService{
			startHandle: 0,
			endHandle:   1,
			uuid:        ServiceUUIDGenericAccess,
		}
		binary.LittleEndian.PutUint16(response[2:], genericAccessService.startHandle)
		binary.LittleEndian.PutUint16(response[4:], genericAccessService.endHandle)
		binary.LittleEndian.PutUint16(response[6:], genericAccessService.uuid.Get16Bit())

		genericAttributeService := rawService{
			startHandle: 2,
			endHandle:   5,
			uuid:        ServiceUUIDGenericAttribute,
		}
		binary.LittleEndian.PutUint16(response[8:], genericAttributeService.startHandle)
		binary.LittleEndian.PutUint16(response[10:], genericAttributeService.endHandle)
		binary.LittleEndian.PutUint16(response[12:], genericAttributeService.uuid.Get16Bit())

		if err := a.hci.sendAclPkt(handle, attCID, response[:]); err != nil {
			return err
		}

	case attOpReadByGroupResponse:
		if _debug {
			println("att.handleData: attOpReadByGroupResponse")
		}
		a.responded = true

		lengthPerService := int(buf[1])
		var uuid [16]byte

		for i := 2; i < len(buf); i += lengthPerService {
			service := rawService{
				startHandle: binary.LittleEndian.Uint16(buf[i:]),
				endHandle:   binary.LittleEndian.Uint16(buf[i+2:]),
			}
			switch lengthPerService - 4 {
			case 2:
				service.uuid = New16BitUUID(binary.LittleEndian.Uint16(buf[i+4:]))
			case 16:
				copy(uuid[:], buf[i+4:])
				slices.Reverse(uuid[:])
				service.uuid = NewUUID(uuid)
			}

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
