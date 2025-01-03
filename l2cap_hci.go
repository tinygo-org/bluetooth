//go:build ninafw || hci || cyw43439

package bluetooth

import (
	"encoding/binary"
	"encoding/hex"
)

const (
	connectionParamUpdateRequest  = 0x12
	connectionParamUpdateResponse = 0x13
)

type l2capConnectionParamReqPkt struct {
	minInterval uint16
	maxInterval uint16
	latency     uint16
	timeout     uint16
}

func (l *l2capConnectionParamReqPkt) Write(buf []byte) (int, error) {
	l.minInterval = binary.LittleEndian.Uint16(buf[0:])
	l.maxInterval = binary.LittleEndian.Uint16(buf[2:])
	l.latency = binary.LittleEndian.Uint16(buf[4:])
	l.timeout = binary.LittleEndian.Uint16(buf[6:])

	return 8, nil
}

func (l *l2capConnectionParamReqPkt) Read(p []byte) (int, error) {
	binary.LittleEndian.PutUint16(p[0:], l.minInterval)
	binary.LittleEndian.PutUint16(p[2:], l.maxInterval)
	binary.LittleEndian.PutUint16(p[4:], l.latency)
	binary.LittleEndian.PutUint16(p[6:], l.timeout)

	return 8, nil
}

type l2capConnectionParamResponsePkt struct {
	code       uint8
	identifier uint8
	length     uint16
	value      uint16
}

func (l *l2capConnectionParamResponsePkt) Read(p []byte) (int, error) {
	p[0] = l.code
	p[1] = l.identifier
	binary.LittleEndian.PutUint16(p[2:], l.length)
	binary.LittleEndian.PutUint16(p[4:], l.value)

	return 6, nil
}

type l2cap struct {
	hci *hci
}

func newL2CAP(hci *hci) *l2cap {
	return &l2cap{
		hci: hci,
	}
}

func (l *l2cap) addConnection(handle uint16, role uint8, interval, timeout uint16) error {
	if role != 0x01 {
		return nil
	}

	if timeout == 0 {
		timeout = 0x00c8
	}

	var b [12]byte
	b[0] = connectionParamUpdateRequest
	b[1] = 0x01
	binary.LittleEndian.PutUint16(b[2:], 8)
	binary.LittleEndian.PutUint16(b[4:], interval)
	binary.LittleEndian.PutUint16(b[6:], interval)
	binary.LittleEndian.PutUint16(b[8:], 0)
	binary.LittleEndian.PutUint16(b[10:], timeout)

	return l.sendReq(handle, b[:])
}

func (l *l2cap) removeConnection(handle uint16) error {
	return nil
}

func (l *l2cap) handleData(handle uint16, buf []byte) error {
	code := buf[0]
	identifier := buf[1]
	//length := binary.LittleEndian.Uint16(buf[2:4])

	if debug {
		println("l2cap.handleData:", handle, "data:", hex.EncodeToString(buf))
	}

	// TODO: check length

	switch code {
	case connectionParamUpdateRequest:
		return l.handleParameterUpdateRequest(handle, identifier, buf[4:])

	case connectionParamUpdateResponse:
		return l.handleParameterUpdateResponse(handle, identifier, buf[4:])
	}

	return nil
}

func (l *l2cap) handleParameterUpdateRequest(connectionHandle uint16, identifier uint8, data []byte) error {
	if debug {
		println("l2cap.handleParameterUpdateRequest:", connectionHandle, "identifier:", identifier, "data:", hex.EncodeToString(data))
	}

	req := l2capConnectionParamReqPkt{}
	req.Write(data)

	// TODO: check against min/max

	resp := l2capConnectionParamResponsePkt{
		code:       connectionParamUpdateResponse,
		identifier: identifier,
		length:     2,
		value:      0,
	}

	var b [6]byte
	resp.Read(b[:])

	if err := l.sendReq(connectionHandle, b[:]); err != nil {
		return err
	}

	return nil
}

func (l *l2cap) handleParameterUpdateResponse(connectionHandle uint16, identifier uint8, data []byte) error {
	if debug {
		println("l2cap.handleParameterUpdateResponse:", connectionHandle, "data:", hex.EncodeToString(data))
	}

	// for now do nothing
	return nil
}

func (l *l2cap) sendReq(handle uint16, data []byte) error {
	if debug {
		println("l2cap.sendReq:", handle, "data:", hex.EncodeToString(data))
	}

	return l.hci.sendAclPkt(handle, signalingCID, data)
}
