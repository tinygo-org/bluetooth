package hci

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
	"time"

	"tinygo.org/x/bluetooth/ble"
)

const (
	OGFCommandPos = 10

	OGFLinkCtl     = 0x01
	OGFHostCtl     = 0x03
	OGFInfoParam   = 0x04
	OGFStatusParam = 0x05
	OGFLECtrl      = 0x08

	// OGFLinkCtl
	OCFDisconnect = 0x0006

	// OGFHostCtl
	OCFSetEventMask = 0x0001
	OCFReset        = 0x0003

	// OGFInfoParam
	OCFReadLocalVersion = 0x0001
	OCFReadBDAddr       = 0x0009

	// OGFStatusParam
	OCFReadRSSI = 0x0005

	// OGFLECtrl
	OCFLEReadBufferSize           = 0x0002
	OCFLESetRandomAddress         = 0x0005
	OCFLESetAdvertisingParameters = 0x0006
	OCFLESetAdvertisingData       = 0x0008
	OCFLESetScanResponseData      = 0x0009
	OCFLESetAdvertiseEnable       = 0x000a
	OCFLESetScanParameters        = 0x000b
	OCFLESetScanEnable            = 0x000c
	OCFLECreateConn               = 0x000d
	OCFLECancelConn               = 0x000e
	OCFLEConnUpdate               = 0x0013
	OCFLEParamRequestReply        = 0x0020

	leCommandEncrypt                  = 0x0017
	leCommandRandom                   = 0x0018
	leCommandLongTermKeyReply         = 0x001A
	leCommandLongTermKeyNegativeReply = 0x001B
	leCommandReadLocalP256            = 0x0025
	leCommandGenerateDHKeyV1          = 0x0026
	leCommandGenerateDHKeyV2          = 0x005E

	LEMetaConnComplete                   = 0x01
	LEMetaAdvertisingReport              = 0x02
	LEMetaConnectionUpdateComplete       = 0x03
	LEMetaReadRemoteUsedFeaturesComplete = 0x04
	LEMetaLongTermKeyRequest             = 0x05
	LEMetaRemoteConnParamReq             = 0x06
	LEMetaDataLengthChange               = 0x07
	LEMetaReadLocalP256Complete          = 0x08
	LEMetaGenerateDHKeyComplete          = 0x09
	LEMetaEnhancedConnectionComplete     = 0x0A
	LEMetaDirectAdvertisingReport        = 0x0B

	PacketCommand         = 0x01
	PacketACLData         = 0x02
	PacketSynchronousData = 0x03
	PacketEvent           = 0x04
	PacketSecurity        = 0x06

	EventDisconnComplete  = 0x05
	EventEncryptionChange = 0x08
	EventCmdComplete      = 0x0e
	EventCmdStatus        = 0x0f
	EventHardwareError    = 0x10
	EventNumCompPkts      = 0x13
	EventReturnLinkKeys   = 0x15
	EventLEMeta           = 0x3e

	ReasonRemoteUserTerminated = 0x13
)

const (
	hciACLLenPos = 4
	hciEvtLenPos = 2

	CIDATT       = 0x0004
	bleCTL       = 0x0008
	CIDSignaling = 0x0005
	CIDSecurity  = 0x0006
)

var (
	ErrTimeout       = errors.New("bluetooth: HCI timeout")
	ErrUnknownEvent  = errors.New("bluetooth: HCI unknown event")
	ErrUnknown       = errors.New("bluetooth: HCI unknown error")
	ErrInvalidPacket = errors.New("bluetooth: HCI invalid packet")
	ErrHardware      = errors.New("bluetooth: HCI hardware error")
)

// AdvertisementReport is one LE Advertising Report. The advertisement data is
// a fixed array rather than a slice, so that a report does not go on the heap.
type AdvertisementReport struct {
	NumReports  uint8
	Type        uint8
	AddressType uint8
	Address     [6]uint8
	DataLen     uint8
	Data        [31]uint8
	RSSI        int8
}

// ConnectionComplete is an LE Connection Complete or LE Enhanced Connection
// Complete event.
type ConnectionComplete struct {
	Status      uint8
	Handle      uint16
	Role        uint8
	AddressType uint8
	Address     [6]uint8
	Interval    uint16
	Timeout     uint16
}

// Disconnection is a Disconnection Complete event.
type Disconnection struct {
	Handle uint16
	Reason uint8
}

type HCI struct {
	transport         Transport
	att               *ATT
	l2cap             *l2cap
	buf               []byte
	pos               int
	end               int
	writebuf          []byte
	address           ble.MACAddress
	cmdCompleteOpcode uint16
	cmdCompleteStatus uint8
	cmdResponse       []byte
	scanning          bool
	maxPkt            uint16
	pendingPkt        uint16

	// The most recent event of each kind, read back with the accessors below.
	// These are fields rather than callbacks because a callback for each event
	// costs about 2 kB of flash on TinyGo.
	advReport    AdvertisementReport
	advReported  bool
	connEvent    ConnectionComplete
	connected    bool
	disconnEvent Disconnection
	disconnected bool

	eventHandler   func(event uint8, params []byte) (bool, error)
	leEventHandler func(subevent uint8, params []byte) (bool, error)

	// The timeouts and the delay between polls. A test shortens them so that
	// a timeout path finishes at once. They are unexported, because the
	// defaults are the only supported setting.
	commandTimeout  time.Duration
	responseTimeout time.Duration
	retryDelay      time.Duration
}

// Advertisement returns the most recent LE advertising report, and whether one
// has arrived since the last call to ClearAdvertisement. The report is only
// valid until the next poll.
func (h *HCI) Advertisement() (*AdvertisementReport, bool) {
	return &h.advReport, h.advReported
}

// HasAdvertisement reports whether an advertising report has arrived since
// the last call to ClearAdvertisement.
func (h *HCI) HasAdvertisement() bool {
	return h.advReported
}

// ClearAdvertisement discards the stored advertising report.
func (h *HCI) ClearAdvertisement() {
	h.advReport = AdvertisementReport{}
	h.advReported = false
}

// HasConnection reports whether a connection complete event has arrived since
// the last call to ClearConnection.
func (h *HCI) HasConnection() bool {
	return h.connected
}

// HasDisconnection reports whether a disconnection event has arrived since the
// last call to ClearConnection.
func (h *HCI) HasDisconnection() bool {
	return h.disconnected
}

// Connection returns the most recent connection complete event, and whether
// one has arrived since the last call to ClearConnection.
func (h *HCI) Connection() (*ConnectionComplete, bool) {
	return &h.connEvent, h.connected
}

// Disconnection returns the most recent Disconnection event, and whether one
// has arrived since the last call to clearConnection.
func (h *HCI) Disconnection() (*Disconnection, bool) {
	return &h.disconnEvent, h.disconnected
}

// clearConnection discards the stored connection and Disconnection events.
func (h *HCI) ClearConnection() {
	h.connEvent = ConnectionComplete{}
	h.connected = false
	h.disconnEvent = Disconnection{}
	h.disconnected = false
}

// setEventHandler installs a handler for events that this package does not
// handle itself. The handler returns true when it consumed the event. This is
// the hook for controller specific events.
func (h *HCI) SetEventHandler(fn func(event uint8, params []byte) (bool, error)) {
	h.eventHandler = fn
}

// setLeEventHandler installs a handler for LE meta subevents that this package
// does not handle itself. The handler returns true when it consumed the
// subevent.
func (h *HCI) SetLEEventHandler(fn func(subevent uint8, params []byte) (bool, error)) {
	h.leEventHandler = fn
}

// Stack is the HCI, L2CAP and ATT layers wired together on one transport.
type Stack struct {
	HCI *HCI
	ATT *ATT
}

// NewStack builds the protocol stack on a transport. Call Start on the HCI
// layer to bring the controller up.
func NewStack(t Transport) *Stack {
	h := newHCI(t)
	a := newATT(h)
	h.att = a
	h.l2cap = newL2CAP(h)

	return &Stack{HCI: h, ATT: a}
}

// Address returns the controller address, as last read by ReadBdAddr or set by
// SetRandomAddress.
func (h *HCI) Address() ble.MACAddress {
	return h.address
}

// CommandResponse returns the parameters of the most recent Command Complete
// event. The slice points into the read buffer and is only valid until the
// next poll.
func (h *HCI) CommandResponse() []byte {
	return h.cmdResponse
}

// CommandStatus returns the status of the most recent Command Complete or
// Command Status event.
func (h *HCI) CommandStatus() uint8 {
	return h.cmdCompleteStatus
}

func newHCI(t Transport) *HCI {
	return &HCI{
		transport:       t,
		buf:             make([]byte, ReadBufferSize),
		writebuf:        make([]byte, 256),
		commandTimeout:  3 * time.Second,
		responseTimeout: 10 * time.Second,
		retryDelay:      5 * time.Millisecond,
	}
}

func (h *HCI) Start() error {
	h.transport.StartRead()
	defer h.transport.EndRead()

	for {
		available := h.transport.Buffered()
		if available == 0 {
			return nil
		}

		// Discard whatever is left over. The read goes into the packet buffer
		// rather than a small scratch buffer because a packet oriented
		// transport hands back a whole packet at a time and fails the read
		// outright when it does not fit.
		aligned := alignUp4(available)
		if aligned > len(h.buf) {
			return ErrInvalidPacket
		}
		if _, err := h.transport.Read(h.buf[:aligned]); err != nil {
			return err
		}
	}
}

func (h *HCI) Stop() error {
	return nil
}

func (h *HCI) Reset() error {
	return h.SendCommand(OGFHostCtl<<10 | OCFReset)
}

func (h *HCI) Poll() error {
	h.transport.StartRead()
	defer h.transport.EndRead()

	for {
		// noRoom records that data is waiting but does not fit alongside what
		// is already buffered.
		noRoom := false

		// perform read only if more data is available
		if available := h.transport.Buffered(); available > 0 {
			// Read in 4 byte aligned chunks. A packet oriented transport hands
			// back a whole packet at a time and fails the read outright when it
			// does not fit, so only read when there is room for all of it and
			// leave the rest until the buffer has been drained.
			aligned := alignUp4(available)
			switch {
			case h.end+aligned <= len(h.buf):
				n, err := h.transport.Read(h.buf[h.end : h.end+aligned])
				if err != nil {
					return err
				}
				h.end += n
			case h.end == 0:
				// There is nothing to drain, so this can never be read.
				if debug {
					println("hci poll packet too large:", available)
				}

				return ErrInvalidPacket
			default:
				noRoom = true
			}
		}

		if h.end == 0 {
			return nil
		}

		// do processing
		h.pos = h.end
		done, err := h.processPacket()
		switch {
		case err == ErrInvalidPacket || err == ErrUnknown || err == ErrUnknownEvent:
			if debug {
				println("hci poll unknown packet:", err.Error(), hex.EncodeToString(h.buf[:h.pos]))
			}

			h.pos = 0
			h.end = 0

			time.Sleep(h.retryDelay)
		case err != nil:
			// some other error, so return
			h.pos = 0
			h.end = 0

			return err
		case done:
			switch {
			case h.end > h.pos:
				// move remaining data to start of buffer
				copy(h.buf[:], h.buf[h.pos:h.end])
				h.end -= h.pos
			default:
				h.end = 0
			}

			h.pos = 0
			return nil
		case noRoom:
			// The buffer holds an incomplete packet and there is no room left
			// to read the rest of it, so it can never be completed.
			if debug {
				println("hci poll buffer overflow", hex.EncodeToString(h.buf[:h.end]))
			}
			h.pos = 0
			h.end = 0

			time.Sleep(h.retryDelay)
		case h.transport.Buffered() == 0:
			// Incomplete packet with nothing more to read for now. Keep it and
			// pick up where we left off on the next poll.
			return nil
		}
	}
}

func (h *HCI) processPacket() (bool, error) {
	switch h.buf[0] {
	case PacketACLData:
		if h.pos > hciACLLenPos {
			pktlen := int(binary.LittleEndian.Uint16(h.buf[3:5]))

			// Total size of the packet, including the leading packet type
			// byte. The length comes off the wire, so it may be larger than
			// the read buffer can ever hold.
			pktTotal := hciACLLenPos + pktlen + 1
			if pktTotal > MaxPacketSize {
				return true, ErrInvalidPacket
			}

			switch {
			case h.end < pktTotal:
				// need to read more data
				return false, nil
			case h.pos >= pktTotal:
				if debug {
					println("hci acl data recv:", h.pos, hex.EncodeToString(h.buf[:pktTotal]))
				}

				h.pos = pktTotal
				return true, h.handleACLData(h.buf[1:h.pos])
			}
		}

	case PacketEvent:
		if h.pos > hciEvtLenPos {
			pktlen := int(h.buf[hciEvtLenPos])

			pktTotal := hciEvtLenPos + pktlen + 1
			if pktTotal > MaxPacketSize {
				return true, ErrInvalidPacket
			}

			switch {
			case h.end < pktTotal:
				// need to read more data
				return false, nil
			case h.pos >= pktTotal:
				if debug {
					println("hci event data recv:", h.pos, hex.EncodeToString(h.buf[:pktTotal]))
				}

				h.pos = pktTotal
				return true, h.handleEventData(h.buf[1:h.pos])
			}
		}

	case PacketSynchronousData:
		// not supported by BLE, so ignore
		if h.pos > 3 {
			pktlen := int(h.buf[3])
			if debug {
				println("hci synchronous data recv:", h.pos, pktlen, hex.EncodeToString(h.buf[:1+3+pktlen]))
			}

			// move to next packet
			h.pos = 1 + 3 + pktlen

			return true, nil
		}

	default:
		if debug {
			println("unknown packet data recv:", h.pos, h.end, hex.EncodeToString(h.buf[:h.pos]))
		}
		return true, ErrUnknown
	}

	return false, nil
}

func (h *HCI) ReadBdAddr() error {
	if err := h.SendCommand(OGFInfoParam<<OGFCommandPos | OCFReadBDAddr); err != nil {
		return err
	}

	if len(h.cmdResponse) < 7 {
		return ErrInvalidPacket
	}

	copy(h.address.MAC[:], h.cmdResponse[:7])

	return nil
}

func (h *HCI) SetRandomAddress(mac ble.MAC) error {
	if err := h.SendCommandWithParams(OGFLECtrl<<OGFCommandPos|OCFLESetRandomAddress, mac[:]); err != nil {
		return err
	}

	copy(h.address.MAC[:], mac[:])
	h.address.SetRandom(true)

	return nil
}

func (h *HCI) SetEventMask(eventMask uint64) error {
	var b [8]byte
	binary.LittleEndian.PutUint64(b[:], eventMask)
	return h.SendCommandWithParams(OGFHostCtl<<OGFCommandPos|OCFSetEventMask, b[:])
}

func (h *HCI) SetLEEventMask(eventMask uint64) error {
	var b [8]byte
	binary.LittleEndian.PutUint64(b[:], eventMask)
	return h.SendCommandWithParams(OGFLECtrl<<OGFCommandPos|0x01, b[:])
}

func (h *HCI) ReadLEBufferSize() error {
	if err := h.SendCommand(OGFLECtrl<<OGFCommandPos | OCFLEReadBufferSize); err != nil {
		return err
	}

	pktLen := binary.LittleEndian.Uint16(h.buf[0:])
	h.maxPkt = uint16(h.buf[2])

	// pkt len must be at least 27 bytes
	if pktLen < 27 {
		pktLen = 27
	}

	if err := h.att.SetMaxMTU(pktLen); err != nil {
		return err
	}

	return nil
}

func (h *HCI) LESetScanEnable(enabled, duplicates bool) error {
	h.scanning = enabled

	var data [2]byte
	if enabled {
		data[0] = 1
	}
	if duplicates {
		data[1] = 1
	}

	return h.SendCommandWithParams(OGFLECtrl<<OGFCommandPos|OCFLESetScanEnable, data[:])
}

func (h *HCI) LESetScanParameters(typ uint8, interval, window uint16, ownBdaddrType, filter uint8) error {
	var data [7]byte
	data[0] = typ
	binary.LittleEndian.PutUint16(data[1:], interval)
	binary.LittleEndian.PutUint16(data[3:], window)
	data[5] = ownBdaddrType
	data[6] = filter

	return h.SendCommandWithParams(OGFLECtrl<<OGFCommandPos|OCFLESetScanParameters, data[:])
}

func (h *HCI) LESetAdvertiseEnable(enabled bool) error {
	var data [1]byte
	if enabled {
		data[0] = 1
	}

	return h.SendWithoutResponse(OGFLECtrl<<OGFCommandPos|OCFLESetAdvertiseEnable, data[:])
}

func (h *HCI) LESetAdvertisingParameters(minInterval, maxInterval uint16,
	advType, ownBdaddrType uint8,
	directBdaddrType uint8, directBdaddr [6]byte,
	chanMap, filter uint8) error {

	var b [15]byte
	binary.LittleEndian.PutUint16(b[0:], minInterval)
	binary.LittleEndian.PutUint16(b[2:], maxInterval)
	b[4] = advType
	b[5] = ownBdaddrType
	b[6] = directBdaddrType
	copy(b[7:], directBdaddr[:])
	b[13] = chanMap
	b[14] = filter

	return h.SendCommandWithParams(OGFLECtrl<<OGFCommandPos|OCFLESetAdvertisingParameters, b[:])
}

func (h *HCI) LESetAdvertisingData(data []byte) error {
	var b [32]byte
	b[0] = byte(len(data))
	copy(b[1:], data)

	return h.SendCommandWithParams(OGFLECtrl<<OGFCommandPos|OCFLESetAdvertisingData, b[:])
}

func (h *HCI) LESetScanResponseData(data []byte) error {
	var b [32]byte
	b[0] = byte(len(data))
	copy(b[1:], data)

	return h.SendCommandWithParams(OGFLECtrl<<OGFCommandPos|OCFLESetScanResponseData, b[:])
}

func (h *HCI) LECreateConn(interval, window uint16,
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

	return h.SendCommandWithParams(OGFLECtrl<<OGFCommandPos|OCFLECreateConn, b[:])
}

func (h *HCI) LECancelConn() error {
	return h.SendCommand(OGFLECtrl<<OGFCommandPos | OCFLECancelConn)
}

func (h *HCI) LEConnUpdate(handle uint16, minInterval, maxInterval,
	latency, supervisionTimeout uint16) error {

	var b [14]byte
	binary.LittleEndian.PutUint16(b[0:], handle)
	binary.LittleEndian.PutUint16(b[2:], minInterval)
	binary.LittleEndian.PutUint16(b[4:], maxInterval)
	binary.LittleEndian.PutUint16(b[6:], latency)
	binary.LittleEndian.PutUint16(b[8:], supervisionTimeout)
	binary.LittleEndian.PutUint16(b[10:], 0x0004)
	binary.LittleEndian.PutUint16(b[12:], 0x0006)

	return h.SendCommandWithParams(OGFLECtrl<<OGFCommandPos|OCFLEConnUpdate, b[:])
}

func (h *HCI) Disconnect(handle uint16) error {
	var b [3]byte
	binary.LittleEndian.PutUint16(b[0:], handle)
	b[2] = ReasonRemoteUserTerminated

	return h.SendCommandWithParams(OGFLinkCtl<<OGFCommandPos|OCFDisconnect, b[:])
}

func (h *HCI) SendCommand(opcode uint16) error {
	return h.SendCommandWithParams(opcode, []byte{})
}

func (h *HCI) SendCommandWithParams(opcode uint16, params []byte) error {
	if debug {
		println("hci send command", opcode, hex.EncodeToString(params))
	}

	h.writebuf[0] = PacketCommand
	binary.LittleEndian.PutUint16(h.writebuf[1:], opcode)
	h.writebuf[3] = byte(len(params))
	copy(h.writebuf[4:], params)

	if _, err := h.write(h.writebuf[:4+len(params)]); err != nil {
		return err
	}

	h.cmdCompleteOpcode = 0xffff
	h.cmdCompleteStatus = 0xff

	start := time.Now()
	for h.cmdCompleteOpcode != opcode {
		if err := h.Poll(); err != nil {
			return err
		}

		if time.Since(start) > h.commandTimeout {
			return ErrTimeout
		}
	}

	return nil
}

func (h *HCI) SendWithoutResponse(opcode uint16, params []byte) error {
	if debug {
		println("hci send without response command", opcode, hex.EncodeToString(params))
	}

	h.writebuf[0] = PacketCommand
	binary.LittleEndian.PutUint16(h.writebuf[1:], opcode)
	h.writebuf[3] = byte(len(params))
	copy(h.writebuf[4:], params)

	if _, err := h.write(h.writebuf[:4+len(params)]); err != nil {
		return err
	}

	h.cmdCompleteOpcode = 0xffff
	h.cmdCompleteStatus = 0xff

	return nil
}

func (h *HCI) SendACLPacket(handle, cid uint16, data []byte) error {
	h.writebuf[0] = PacketACLData
	binary.LittleEndian.PutUint16(h.writebuf[1:], handle)
	binary.LittleEndian.PutUint16(h.writebuf[3:], uint16(len(data)+4))
	binary.LittleEndian.PutUint16(h.writebuf[5:], uint16(len(data)))
	binary.LittleEndian.PutUint16(h.writebuf[7:], cid)

	copy(h.writebuf[9:], data)

	if debug {
		println("hci send acl data", handle, cid, hex.EncodeToString(h.writebuf[:9+len(data)]))
	}

	if _, err := h.write(h.writebuf[:9+len(data)]); err != nil {
		return err
	}

	h.pendingPkt++

	return nil
}

func (h *HCI) write(buf []byte) (int, error) {
	return h.transport.Write(buf)
}

type aclDataHeader struct {
	handle uint16
	dlen   uint16
	len    uint16
	cid    uint16
}

func (h *HCI) handleACLData(buf []byte) error {
	// The ACL header is 4 bytes, followed by a 4 byte L2CAP header.
	if len(buf) < 8 {
		return ErrInvalidPacket
	}

	aclHdr := aclDataHeader{
		handle: binary.LittleEndian.Uint16(buf[0:]),
		dlen:   binary.LittleEndian.Uint16(buf[2:]),
		len:    binary.LittleEndian.Uint16(buf[4:]),
		cid:    binary.LittleEndian.Uint16(buf[6:]),
	}

	aclFlags := (aclHdr.handle & 0xf000) >> 12
	if aclHdr.dlen < 4 || aclHdr.dlen-4 != aclHdr.len {
		return errors.New("fragmented packet")
	}

	// The L2CAP length comes off the wire, so check the payload is really
	// present rather than trusting it. Computed as an int, since the uint16
	// arithmetic would wrap around.
	end := 8 + int(aclHdr.len)
	if end > len(buf) {
		if debug {
			println("invalid acl payload length", aclHdr.len, len(buf))
		}
		return ErrInvalidPacket
	}

	switch aclHdr.cid {
	case CIDATT:
		if aclFlags == 0x01 {
			// TODO: use buffered packet
			if debug {
				println("WARNING: att.handleACLData needs buffered packet")
			}
			return h.att.handleData(aclHdr.handle&0x0fff, buf[8:end])
		} else {
			return h.att.handleData(aclHdr.handle&0x0fff, buf[8:end])
		}
	case CIDSignaling:
		if debug {
			println("signaling cid", aclHdr.cid, hex.EncodeToString(buf))
		}

		return h.l2cap.handleData(aclHdr.handle&0x0fff, buf[8:end])

	default:
		if debug {
			println("unknown acl data cid", aclHdr.cid)
		}
	}

	return nil
}

func (h *HCI) handleEventData(buf []byte) error {
	// Every event has at least an event code and a parameter length byte.
	if len(buf) < 2 {
		return ErrInvalidPacket
	}

	evt := buf[0]
	plen := buf[1]

	switch evt {
	case EventDisconnComplete:
		if debug {
			println("EventDisconnComplete")
		}

		if len(buf) < 5 {
			return ErrInvalidPacket
		}

		handle := binary.LittleEndian.Uint16(buf[3:])
		h.att.removeConnection(handle)
		h.l2cap.removeConnection(handle)

		h.disconnEvent = Disconnection{Handle: handle}
		// The reason follows the handle. A well formed event always carries
		// it, but the length comes off the wire.
		if len(buf) > 5 {
			h.disconnEvent.Reason = buf[5]
		}
		h.disconnected = true

		return h.LESetAdvertiseEnable(true)

	case EventEncryptionChange:
		if debug {
			println("EventEncryptionChange")
		}

	case EventCmdComplete:
		if len(buf) < 6 || int(plen)+2 > len(buf) {
			return ErrInvalidPacket
		}

		h.cmdCompleteOpcode = binary.LittleEndian.Uint16(buf[3:])
		h.cmdCompleteStatus = buf[5]
		if plen > 0 {
			h.cmdResponse = buf[1 : plen+2]
		} else {
			h.cmdResponse = buf[:0]
		}

		if debug {
			println("EventCmdComplete", h.cmdCompleteOpcode, h.cmdCompleteStatus)
		}

		return nil

	case EventCmdStatus:
		if len(buf) < 6 {
			return ErrInvalidPacket
		}

		h.cmdCompleteStatus = buf[2]
		h.cmdCompleteOpcode = binary.LittleEndian.Uint16(buf[4:])
		if debug {
			println("EventCmdStatus", h.cmdCompleteOpcode, h.cmdCompleteOpcode, h.cmdCompleteStatus)
		}

		h.cmdResponse = buf[:0]

		return nil

	case EventNumCompPkts:
		if debug {
			println("EventNumCompPkts", hex.EncodeToString(buf))
		}
		if len(buf) < 3 {
			return ErrInvalidPacket
		}

		// count of handles
		c := buf[2]
		pkts := uint16(0)

		// The handle count comes off the wire, so make sure the event is
		// actually long enough to hold that many entries.
		if 5+(int(c)-1)*4+2 > len(buf) {
			return ErrInvalidPacket
		}

		for i := byte(0); i < c; i++ {
			pkts += binary.LittleEndian.Uint16(buf[5+int(i)*4:])
		}

		if pkts > 0 && h.pendingPkt > pkts {
			h.pendingPkt -= pkts
		} else {
			h.pendingPkt = 0
		}

		if debug {
			println("EventNumCompPkts", pkts, h.pendingPkt)
		}

		return nil

	case EventLEMeta:
		if debug {
			println("EventLEMeta")
		}

		// An LE meta event has at least a subevent code.
		if len(buf) < 3 {
			return ErrInvalidPacket
		}

		switch buf[2] {
		case LEMetaConnComplete, LEMetaEnhancedConnectionComplete:
			if debug {
				if buf[2] == LEMetaConnComplete {
					println("LEMetaConnComplete", hex.EncodeToString(buf))
				} else {
					println("LEMetaEnhancedConnectionComplete", hex.EncodeToString(buf))
				}
			}

			// The enhanced variant carries two extra addresses before the
			// connection interval, so it needs a longer event.
			minLen := 20
			if buf[2] == LEMetaEnhancedConnectionComplete {
				minLen = 32
			}
			if len(buf) < minLen {
				if debug {
					println("invalid connection complete length", len(buf))
				}
				return ErrInvalidPacket
			}

			h.connEvent = ConnectionComplete{
				Status:      buf[3],
				Handle:      binary.LittleEndian.Uint16(buf[4:]),
				Role:        buf[6],
				AddressType: buf[7],
			}
			copy(h.connEvent.Address[0:], buf[8:14])

			switch buf[2] {
			case LEMetaConnComplete:
				h.connEvent.Interval = binary.LittleEndian.Uint16(buf[14:])
				h.connEvent.Timeout = binary.LittleEndian.Uint16(buf[18:])
			case LEMetaEnhancedConnectionComplete:
				h.connEvent.Interval = binary.LittleEndian.Uint16(buf[26:])
				h.connEvent.Timeout = binary.LittleEndian.Uint16(buf[30:])
			}
			h.connected = true

			h.att.addConnection(h.connEvent.Handle)
			if err := h.l2cap.addConnection(h.connEvent.Handle, h.connEvent.Role,
				h.connEvent.Interval, h.connEvent.Timeout); err != nil {
				return err
			}

			return h.LESetAdvertiseEnable(false)

		case LEMetaAdvertisingReport:
			// Validate the whole report before the handler runs, so a
			// truncated one is never reported. The fixed part is 13 bytes (up
			// to and including the data length), followed by the
			// advertisement data and one RSSI byte.
			if len(buf) < 13 {
				if debug {
					println("invalid advertising report length", len(buf))
				}
				return ErrInvalidPacket
			}

			eirLength := buf[12]

			// Note: the length must be computed as an int. Doing the
			// arithmetic on the uint8 eirLength would wrap around.
			if eirLength > 31 || 13+int(eirLength)+1 > len(buf) {
				if debug {
					println("invalid packet length", eirLength, len(buf))
				}
				return ErrInvalidPacket
			}

			h.advReport = AdvertisementReport{
				NumReports:  buf[3],
				Type:        buf[4],
				AddressType: buf[5],
				DataLen:     eirLength,
			}
			copy(h.advReport.Address[0:], buf[6:12])
			copy(h.advReport.Data[0:eirLength], buf[13:13+eirLength])

			// TODO: handle multiple reports
			if h.advReport.NumReports == 0x01 {
				h.advReport.RSSI = int8(buf[13+int(eirLength)])
			}
			h.advReported = true

			if debug {
				println("LEMetaAdvertisingReport", plen, h.advReport.NumReports,
					h.advReport.Type, h.advReport.AddressType, h.advReport.DataLen)
			}

			return nil

		case LEMetaLongTermKeyRequest:
			if debug {
				println("LEMetaLongTermKeyRequest")
			}

		case LEMetaRemoteConnParamReq:
			if debug {
				println("LEMetaRemoteConnParamReq")
			}

			if len(buf) < 13 {
				return ErrInvalidPacket
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

			return h.SendWithoutResponse(OGFLECtrl<<10|OCFLEParamRequestReply, b[:])

		case LEMetaConnectionUpdateComplete:
			if debug {
				println("LEMetaConnectionUpdateComplete")
			}

		case LEMetaReadLocalP256Complete:
			if debug {
				println("LEMetaReadLocalP256Complete")
			}

		case LEMetaGenerateDHKeyComplete:
			if debug {
				println("LEMetaGenerateDHKeyComplete")
			}

		case LEMetaDataLengthChange:
			if debug {
				println("LEMetaDataLengthChange")
			}

		default:
			if debug {
				println("unknown metaevent", buf[2], buf[3], buf[4], buf[5])
			}

			if h.leEventHandler != nil {
				handled, err := h.leEventHandler(buf[2], buf[3:])
				if err != nil {
					return err
				}
				if handled {
					return nil
				}
			}

			return ErrUnknownEvent
		}
	case EventHardwareError:
		if debug {
			println("EventHardwareError", hex.EncodeToString(buf))
		}

		return ErrUnknownEvent

	default:
		if h.eventHandler != nil {
			handled, err := h.eventHandler(evt, buf[2:])
			if err != nil {
				return err
			}
			if handled {
				return nil
			}
		}
	}

	return nil
}
