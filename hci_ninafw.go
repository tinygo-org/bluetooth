//go:build ninafw

package bluetooth

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
	"machine"
	"time"
)

const (
	ogfCommandPos = 10

	ogfLinkCtl     = 0x01
	ogfHostCtl     = 0x03
	ogfInfoParam   = 0x04
	ogfStatusParam = 0x05
	ogfLECtrl      = 0x08

	// ogfLinkCtl
	ocfDisconnect = 0x0006

	// ogfHostCtl
	ocfSetEventMask = 0x0001
	ocfReset        = 0x0003

	// ogfInfoParam
	ocfReadLocalVersion = 0x0001
	ocfReadBDAddr       = 0x0009

	// ogfStatusParam
	ocfReadRSSI = 0x0005

	// ogfLECtrl
	ocfLEReadBufferSize           = 0x0002
	ocfLESetRandomAddress         = 0x0005
	ocfLESetAdvertisingParameters = 0x0006
	ocfLESetAdvertisingData       = 0x0008
	ocfLESetScanResponseData      = 0x0009
	ocfLESetAdvertiseEnable       = 0x000a
	ocfLESetScanParameters        = 0x000b
	ocfLESetScanEnable            = 0x000c
	ocfLECreateConn               = 0x000d
	ocfLECancelConn               = 0x000e
	ocfLEConnUpdate               = 0x0013
	ocfLEParamRequestReply        = 0x0020

	leCommandEncrypt                  = 0x0017
	leCommandRandom                   = 0x0018
	leCommandLongTermKeyReply         = 0x001A
	leCommandLongTermKeyNegativeReply = 0x001B
	leCommandReadLocalP256            = 0x0025
	leCommandGenerateDHKeyV1          = 0x0026
	leCommandGenerateDHKeyV2          = 0x005E

	leMetaEventConnComplete                   = 0x01
	leMetaEventAdvertisingReport              = 0x02
	leMetaEventConnectionUpdateComplete       = 0x03
	leMetaEventReadRemoteUsedFeaturesComplete = 0x04
	leMetaEventLongTermKeyRequest             = 0x05
	leMetaEventRemoteConnParamReq             = 0x06
	leMetaEventDataLengthChange               = 0x07
	leMetaEventReadLocalP256Complete          = 0x08
	leMetaEventGenerateDHKeyComplete          = 0x09
	leMetaEventEnhancedConnectionComplete     = 0x0A
	leMetaEventDirectAdvertisingReport        = 0x0B

	hciCommandPkt  = 0x01
	hciACLDataPkt  = 0x02
	hciEventPkt    = 0x04
	hciSecurityPkt = 0x06

	evtDisconnComplete  = 0x05
	evtEncryptionChange = 0x08
	evtCmdComplete      = 0x0e
	evtCmdStatus        = 0x0f
	evtHardwareError    = 0x10
	evtNumCompPkts      = 0x13
	evtReturnLinkKeys   = 0x15
	evtLEMetaEvent      = 0x3e

	hciOEUserEndedConnection = 0x13
)

const (
	hciACLLenPos = 4
	hciEvtLenPos = 2
)

var (
	ErrHCITimeout       = errors.New("bluetooth: HCI timeout")
	ErrHCIUnknownEvent  = errors.New("bluetooth: HCI unknown event")
	ErrHCIUnknown       = errors.New("bluetooth: HCI unknown error")
	ErrHCIInvalidPacket = errors.New("bluetooth: HCI invalid packet")
	ErrHCIHardware      = errors.New("bluetooth: HCI hardware error")
)

type leAdvertisingReport struct {
	reported                        bool
	numReports, typ, peerBdaddrType uint8
	peerBdaddr                      [6]uint8
	eirLength                       uint8
	eirData                         [31]uint8
	rssi                            int8
}

type leConnectData struct {
	connected      bool
	status         uint8
	handle         uint16
	role           uint8
	peerBdaddrType uint8
	peerBdaddr     [6]uint8
}

type hci struct {
	uart              *machine.UART
	att               *att
	buf               []byte
	address           [6]byte
	cmdCompleteOpcode uint16
	cmdCompleteStatus uint8
	cmdResponse       []byte
	scanning          bool
	advData           leAdvertisingReport
	connectData       leConnectData
}

func newHCI(uart *machine.UART) *hci {
	return &hci{uart: uart,
		buf: make([]byte, 256),
	}
}

func (h *hci) start() error {
	for h.uart.Buffered() > 0 {
		h.uart.ReadByte()
	}

	return nil
}

func (h *hci) stop() error {
	return nil
}

func (h *hci) reset() error {
	return h.sendCommand(ogfHostCtl<<10 | ocfReset)
}

func (h *hci) poll() error {
	i := 0
	for h.uart.Buffered() > 0 {
		data, _ := h.uart.ReadByte()
		h.buf[i] = data

		done, err := h.processPacket(i)
		switch {
		case err == ErrHCIUnknown || err == ErrHCIInvalidPacket || err == ErrHCIUnknownEvent:
			if _debug {
				println("hci error:", err.Error())
			}
			i = 0
			time.Sleep(5 * time.Millisecond)
		case err != nil:
			return err
		case done:
			return nil
		default:
			i++
			time.Sleep(1 * time.Millisecond)
		}
	}

	return nil
}

func (h *hci) processPacket(i int) (bool, error) {
	switch h.buf[0] {
	case hciACLDataPkt:
		if i > hciACLLenPos {
			pktlen := int(binary.LittleEndian.Uint16(h.buf[3:5]))
			switch {
			case pktlen > len(h.buf):
				return true, ErrHCIInvalidPacket
			case i >= (hciACLLenPos + pktlen):
				if _debug {
					println("hci acl data:", i, hex.EncodeToString(h.buf[:1+hciACLLenPos+pktlen]))
				}
				return true, h.handleACLData(h.buf[1 : 1+hciACLLenPos+pktlen])
			}
		}

	case hciEventPkt:
		if i > hciEvtLenPos {
			pktlen := int(h.buf[hciEvtLenPos])

			switch {
			case pktlen > len(h.buf):
				return true, ErrHCIInvalidPacket
			case i >= (hciEvtLenPos + pktlen):
				if _debug {
					println("hci event data:", i, hex.EncodeToString(h.buf[:1+hciEvtLenPos+pktlen]))
				}
				return true, h.handleEventData(h.buf[1 : 1+hciEvtLenPos+pktlen])
			}
		}

	default:
		if _debug {
			println("unknown packet data:", h.buf[0])
		}
		return true, ErrHCIUnknown
	}

	return false, nil
}

func (h *hci) readBdAddr() error {
	if err := h.sendCommand(ogfInfoParam<<ogfCommandPos | ocfReadBDAddr); err != nil {
		return err
	}

	copy(h.address[:], h.cmdResponse[:7])

	return nil
}

func (h *hci) setEventMask(eventMask uint64) error {
	var b [8]byte
	binary.LittleEndian.PutUint64(b[:], eventMask)
	return h.sendCommandWithParams(ogfHostCtl<<ogfCommandPos|ocfSetEventMask, b[:])
}

func (h *hci) setLeEventMask(eventMask uint64) error {
	var b [8]byte
	binary.LittleEndian.PutUint64(b[:], eventMask)
	return h.sendCommandWithParams(ogfLECtrl<<ogfCommandPos|0x01, b[:])
}

func (h *hci) leSetScanEnable(enabled, duplicates bool) error {
	h.scanning = enabled

	var data [2]byte
	if enabled {
		data[0] = 1
	}
	if duplicates {
		data[1] = 1
	}

	return h.sendCommandWithParams(ogfLECtrl<<ogfCommandPos|ocfLESetScanEnable, data[:])
}

func (h *hci) leSetScanParameters(typ uint8, interval, window uint16, ownBdaddrType, filter uint8) error {
	var data [7]byte
	data[0] = typ
	binary.LittleEndian.PutUint16(data[1:], interval)
	binary.LittleEndian.PutUint16(data[3:], window)
	data[5] = ownBdaddrType
	data[6] = filter

	return h.sendCommandWithParams(ogfLECtrl<<ogfCommandPos|ocfLESetScanParameters, data[:])
}

func (h *hci) leSetAdvertiseEnable(enabled bool) error {
	var data [1]byte
	if enabled {
		data[0] = 1
	}

	return h.sendCommandWithParams(ogfLECtrl<<ogfCommandPos|ocfLESetAdvertiseEnable, data[:])
}

func (h *hci) leCreateConn(interval, window uint16,
	initiatorFilter, peerBdaddrType uint8, peerBdaddr [6]byte, ownBdaddrType uint8,
	minInterval, maxInterval, latency, supervisionTimeout,
	minCeLength, maxCeLength uint16) error {

	var b [25]byte
	binary.LittleEndian.PutUint16(b[0:], interval)
	binary.LittleEndian.PutUint16(b[2:], window)
	b[4] = initiatorFilter
	b[5] = peerBdaddrType
	copy(b[6:], peerBdaddr[:])
	b[12] = ownBdaddrType
	binary.LittleEndian.PutUint16(b[13:], minInterval)
	binary.LittleEndian.PutUint16(b[15:], maxInterval)
	binary.LittleEndian.PutUint16(b[17:], latency)
	binary.LittleEndian.PutUint16(b[19:], supervisionTimeout)
	binary.LittleEndian.PutUint16(b[21:], minCeLength)
	binary.LittleEndian.PutUint16(b[23:], maxCeLength)

	return h.sendCommandWithParams(ogfLECtrl<<ogfCommandPos|ocfLECreateConn, b[:])
}

func (h *hci) leCancelConn() error {
	return h.sendCommand(ogfLECtrl<<ogfCommandPos | ocfLECancelConn)
}

func (h *hci) disconnect(handle uint16) error {
	var b [3]byte
	binary.LittleEndian.PutUint16(b[0:], handle)
	b[2] = hciOEUserEndedConnection

	return h.sendCommandWithParams(ogfLinkCtl<<ogfCommandPos|ocfDisconnect, b[:])
}

func (h *hci) sendCommand(opcode uint16) error {
	return h.sendCommandWithParams(opcode, []byte{})
}

func (h *hci) sendCommandWithParams(opcode uint16, params []byte) error {
	if _debug {
		println("hci send command", opcode, hex.EncodeToString(params))
	}

	h.buf[0] = hciCommandPkt
	binary.LittleEndian.PutUint16(h.buf[1:], opcode)
	h.buf[3] = byte(len(params))
	copy(h.buf[4:], params)

	if _, err := h.uart.Write(h.buf[:4+len(params)]); err != nil {
		return err
	}

	h.cmdCompleteOpcode = 0xffff
	h.cmdCompleteStatus = 0xff

	start := time.Now().UnixNano()
	for h.cmdCompleteOpcode != opcode {
		if err := h.poll(); err != nil {
			return err
		}

		if (time.Now().UnixNano()-start)/int64(time.Second) > 3 {
			return ErrHCITimeout
		}
	}

	return nil
}

func (h *hci) sendAclPkt(handle uint16, cid uint8, data []byte) error {
	h.buf[0] = hciACLDataPkt
	binary.LittleEndian.PutUint16(h.buf[1:], handle)
	binary.LittleEndian.PutUint16(h.buf[3:], uint16(len(data)+4))
	binary.LittleEndian.PutUint16(h.buf[5:], uint16(len(data)))
	binary.LittleEndian.PutUint16(h.buf[7:], uint16(cid))

	copy(h.buf[9:], data)

	if _debug {
		println("hci send acl data", handle, cid, hex.EncodeToString(h.buf[:9+len(data)]))
	}

	if _, err := h.uart.Write(h.buf[:9+len(data)]); err != nil {
		return err
	}

	return nil
}

type aclDataHeader struct {
	handle uint16
	dlen   uint16
	len    uint16
	cid    uint16
}

func (h *hci) handleACLData(buf []byte) error {
	aclHdr := aclDataHeader{
		handle: binary.LittleEndian.Uint16(buf[0:]),
		dlen:   binary.LittleEndian.Uint16(buf[2:]),
		len:    binary.LittleEndian.Uint16(buf[4:]),
		cid:    binary.LittleEndian.Uint16(buf[6:]),
	}

	aclFlags := (aclHdr.handle & 0xf000) >> 12
	if aclHdr.dlen-4 != aclHdr.len {
		return errors.New("fragmented packet")
	}

	switch aclHdr.cid {
	case attCID:
		if aclFlags == 0x01 {
			// TODO: use buffered packet
			if _debug {
				println("WARNING: att.handleACLData needs buffered packet")
			}
			return h.att.handleData(aclHdr.handle&0x0fff, buf[8:aclHdr.len+8])
		} else {
			return h.att.handleData(aclHdr.handle&0x0fff, buf[8:aclHdr.len+8])
		}
	default:
		if _debug {
			println("unknown acl data cid", aclHdr.cid)
		}
	}

	return nil
}

func (h *hci) handleEventData(buf []byte) error {
	evt := buf[0]
	plen := buf[1]

	switch evt {
	case evtDisconnComplete:
		if _debug {
			println("evtDisconnComplete")
		}
		// TODO: something with this data?
		// status := buf[2]
		// handle := buf[3] | (buf[4] << 8)
		// reason := buf[5]
		// ATT.removeConnection(disconnComplete->handle, disconnComplete->reason);
		// L2CAPSignaling.removeConnection(disconnComplete->handle, disconnComplete->reason);

		return h.leSetAdvertiseEnable(true)

	case evtEncryptionChange:
		if _debug {
			println("evtEncryptionChange")
		}

	case evtCmdComplete:
		h.cmdCompleteOpcode = binary.LittleEndian.Uint16(buf[3:])
		h.cmdCompleteStatus = buf[5]
		if plen > 0 {
			h.cmdResponse = buf[1 : plen+2]
		} else {
			h.cmdResponse = buf[:0]
		}

		if _debug {
			println("evtCmdComplete", h.cmdCompleteOpcode, h.cmdCompleteStatus)
		}

		return nil

	case evtCmdStatus:
		h.cmdCompleteStatus = buf[2]
		h.cmdCompleteOpcode = binary.LittleEndian.Uint16(buf[4:])
		if _debug {
			println("evtCmdStatus", h.cmdCompleteOpcode, h.cmdCompleteOpcode, h.cmdCompleteStatus)
		}

		h.cmdResponse = buf[:0]

		return nil

	case evtNumCompPkts:
		if _debug {
			println("evtNumCompPkts")
		}
	case evtLEMetaEvent:
		if _debug {
			println("evtLEMetaEvent")
		}

		switch buf[2] {
		case leMetaEventConnComplete, leMetaEventEnhancedConnectionComplete:
			if _debug {
				println("leMetaEventConnComplete")
			}

			h.connectData.connected = true
			h.connectData.status = buf[3]
			h.connectData.handle = binary.LittleEndian.Uint16(buf[4:])
			h.connectData.role = buf[6]
			h.connectData.peerBdaddrType = buf[7]
			copy(h.connectData.peerBdaddr[0:], buf[8:])

			return nil

		case leMetaEventAdvertisingReport:
			h.advData.reported = true
			h.advData.numReports = buf[3]
			h.advData.typ = buf[4]
			h.advData.peerBdaddrType = buf[5]
			copy(h.advData.peerBdaddr[0:], buf[6:])
			h.advData.eirLength = buf[12]
			h.advData.rssi = 0
			if _debug {
				println("leMetaEventAdvertisingReport", plen, h.advData.numReports,
					h.advData.typ, h.advData.peerBdaddrType, h.advData.eirLength)
			}

			if int(13+h.advData.eirLength+1) > len(buf) || h.advData.eirLength > 31 {
				if _debug {
					println("invalid packet length", h.advData.eirLength, len(buf))
				}
				return ErrHCIInvalidPacket
			}
			copy(h.advData.eirData[0:h.advData.eirLength], buf[13:13+h.advData.eirLength])

			// TODO: handle multiple reports
			if h.advData.numReports == 0x01 {
				h.advData.rssi = int8(buf[int(13+h.advData.eirLength)])
			}

			return nil

		case leMetaEventLongTermKeyRequest:
			if _debug {
				println("leMetaEventLongTermKeyRequest")
			}

		case leMetaEventRemoteConnParamReq:
			if _debug {
				println("leMetaEventRemoteConnParamReq")
			}

			connectionHandle := binary.LittleEndian.Uint16(buf[3:])
			intervalMin := binary.LittleEndian.Uint16(buf[5:])
			intervalMax := binary.LittleEndian.Uint16(buf[7:])
			latency := binary.LittleEndian.Uint16(buf[9:])
			timeOut := binary.LittleEndian.Uint16(buf[11:])

			var b [14]byte
			binary.LittleEndian.PutUint16(b[0:], connectionHandle)
			binary.LittleEndian.PutUint16(b[2:], intervalMin)
			binary.LittleEndian.PutUint16(b[4:], intervalMax)
			binary.LittleEndian.PutUint16(b[6:], latency)
			binary.LittleEndian.PutUint16(b[8:], timeOut)
			binary.LittleEndian.PutUint16(b[10:], 0x000F)
			binary.LittleEndian.PutUint16(b[12:], 0x0FFF)

			return h.sendCommandWithParams(ogfLECtrl<<10|ocfLEParamRequestReply, b[:])

		case leMetaEventConnectionUpdateComplete:
			if _debug {
				println("leMetaEventConnectionUpdateComplete")
			}

		case leMetaEventReadLocalP256Complete:
			if _debug {
				println("leMetaEventReadLocalP256Complete")
			}

		case leMetaEventGenerateDHKeyComplete:
			if _debug {
				println("leMetaEventGenerateDHKeyComplete")
			}

		default:
			if _debug {
				println("unknown metaevent", buf[2], buf[3], buf[4], buf[5])
			}

			h.clearAdvData()
			return ErrHCIUnknownEvent
		}
	case evtHardwareError:
		return ErrHCIUnknownEvent
	}

	return nil
}

func (h *hci) clearAdvData() error {
	h.advData.reported = false
	h.advData.numReports = 0
	h.advData.typ = 0
	h.advData.peerBdaddrType = 0
	h.advData.peerBdaddr = [6]uint8{}
	h.advData.eirLength = 0
	h.advData.eirData = [31]uint8{}
	h.advData.rssi = 0

	return nil
}

func (h *hci) clearConnectData() error {
	h.connectData.connected = false
	h.connectData.status = 0
	h.connectData.handle = 0
	h.connectData.role = 0
	h.connectData.peerBdaddrType = 0
	h.connectData.peerBdaddr = [6]uint8{}

	return nil
}
