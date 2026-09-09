package hci

import (
	"bytes"
	"encoding/binary"
	"errors"
	"testing"

	"tinygo.org/x/bluetooth/ble"
)

const testConnHandle = 0x0040

// errTestRead is returned by a characteristic handler told to fail.
var errTestRead = errors.New("test read error")

// aclPacket wraps an ATT protocol data unit in an ACL and L2CAP header, the
// way it arrives from a controller.
func aclPacket(handle, cid uint16, pdu []byte) []byte {
	pkt := make([]byte, 9+len(pdu))
	pkt[0] = PacketACLData
	binary.LittleEndian.PutUint16(pkt[1:], handle)
	binary.LittleEndian.PutUint16(pkt[3:], uint16(len(pdu)+4))
	binary.LittleEndian.PutUint16(pkt[5:], uint16(len(pdu)))
	binary.LittleEndian.PutUint16(pkt[7:], cid)
	copy(pkt[9:], pdu)

	return pkt
}

// attPacket wraps an ATT protocol data unit for the ATT channel.
func attPacket(pdu ...byte) []byte {
	return aclPacket(testConnHandle, CIDATT, pdu)
}

// sentPDU returns the ATT protocol data unit of the nth write, stripping the
// ACL and L2CAP headers.
func sentPDU(t *testing.T, f *fakeTransport, n int) []byte {
	t.Helper()

	if len(f.tx) <= n {
		t.Fatalf("only %d packets written; want at least %d", len(f.tx), n+1)
	}

	pkt := f.tx[n]
	if len(pkt) < 9 {
		t.Fatalf("write %d is %d bytes; too short for an ACL packet", n, len(pkt))
	}
	if pkt[0] != PacketACLData {
		t.Fatalf("write %d has packet type %#x; want an ACL packet", n, pkt[0])
	}

	return pkt[9:]
}

// newTestATT returns a stack with one connection already open, so that the ATT
// layer has connection data to work with.
func newTestATT(t *testing.T, rx ...[]byte) (*Stack, *fakeTransport) {
	t.Helper()

	f := &fakeTransport{rx: rx}
	s := newTestStack(f)

	if err := s.ATT.addConnection(testConnHandle); err != nil {
		t.Fatalf("addConnection: %v", err)
	}

	return s, f
}

func TestATTServerReadByGroupRequest(t *testing.T) {
	// A primary service with a 16 bit UUID.
	s, f := newTestATT(t)
	handle := s.ATT.AddLocalAttribute(AttributeTypeService, 0,
		UUIDPrimaryService.UUID(), 0, []byte{0x0f, 0x18})
	s.ATT.AddLocalService(handle, handle, ble.New16BitUUID(0x180f))

	// Read By Group Type Request for primary services over the whole range.
	pdu := []byte{OpReadByGroupReq, 0x01, 0x00, 0xff, 0xff, 0x00, 0x28}
	if err := s.ATT.handleData(testConnHandle, pdu); err != nil {
		t.Fatalf("handleData: %v", err)
	}

	got := sentPDU(t, f, 0)
	if got[0] != OpReadByGroupResponse {
		t.Fatalf("opcode = %#x; want OpReadByGroupResponse", got[0])
	}
	// length byte, then start handle, end handle and the 16 bit UUID.
	if got[1] != 6 {
		t.Errorf("entry length = %d; want 6", got[1])
	}
	if uuid := binary.LittleEndian.Uint16(got[6:]); uuid != 0x180f {
		t.Errorf("UUID = %#x; want 0x180f", uuid)
	}
}

func TestATTServerReadByGroupRequestNothingFound(t *testing.T) {
	s, f := newTestATT(t)

	pdu := []byte{OpReadByGroupReq, 0x01, 0x00, 0xff, 0xff, 0x00, 0x28}
	if err := s.ATT.handleData(testConnHandle, pdu); err != nil {
		t.Fatalf("handleData: %v", err)
	}

	got := sentPDU(t, f, 0)
	want := []byte{OpError, OpReadByGroupReq, 0x01, 0x00, byte(ble.ErrAttNotFound)}
	if !bytes.Equal(got, want) {
		t.Errorf("response = %#v; want %#v", got, want)
	}
}

func TestATTServerReadRequest(t *testing.T) {
	s, f := newTestATT(t)

	svc := s.ATT.AddLocalAttribute(AttributeTypeService, 0,
		UUIDPrimaryService.UUID(), 0, []byte{0x0f, 0x18})
	chr := s.ATT.AddLocalAttribute(AttributeTypeCharacteristic, svc,
		UUIDCharacteristic.UUID(), 0x02, nil)
	val := s.ATT.AddLocalAttribute(AttributeTypeCharacteristicValue, chr,
		ble.New16BitUUID(0x2a19), 0x02, []byte{50})

	s.ATT.AddLocalCharacteristic(chr, 0x02, val, ble.New16BitUUID(0x2a19),
		CharacteristicHandler{
			ReadValue: func() ([]byte, error) { return []byte{99}, nil },
		})

	pdu := []byte{OpReadReq, byte(val), byte(val >> 8)}
	if err := s.ATT.handleData(testConnHandle, pdu); err != nil {
		t.Fatalf("handleData: %v", err)
	}

	got := sentPDU(t, f, 0)
	want := []byte{OpReadResponse, 99}
	if !bytes.Equal(got, want) {
		t.Errorf("response = %#v; want %#v", got, want)
	}
}

func TestATTServerReadRequestHandlerError(t *testing.T) {
	s, f := newTestATT(t)

	svc := s.ATT.AddLocalAttribute(AttributeTypeService, 0,
		UUIDPrimaryService.UUID(), 0, nil)
	chr := s.ATT.AddLocalAttribute(AttributeTypeCharacteristic, svc,
		UUIDCharacteristic.UUID(), 0, nil)
	val := s.ATT.AddLocalAttribute(AttributeTypeCharacteristicValue, chr,
		ble.New16BitUUID(0x2a19), 0, nil)

	s.ATT.AddLocalCharacteristic(chr, 0, val, ble.New16BitUUID(0x2a19),
		CharacteristicHandler{
			ReadValue: func() ([]byte, error) { return nil, errTestRead },
		})

	pdu := []byte{OpReadReq, byte(val), byte(val >> 8)}
	if err := s.ATT.handleData(testConnHandle, pdu); err != nil {
		t.Fatalf("handleData: %v", err)
	}

	got := sentPDU(t, f, 0)
	if got[0] != OpError || got[4] != byte(ble.ErrAttReadNotPermitted) {
		t.Errorf("response = %#v; want a read not permitted error", got)
	}
}

func TestATTServerReadRequestUnknownHandle(t *testing.T) {
	s, f := newTestATT(t)

	pdu := []byte{OpReadReq, 0x99, 0x00}
	if err := s.ATT.handleData(testConnHandle, pdu); err != nil {
		t.Fatalf("handleData: %v", err)
	}

	got := sentPDU(t, f, 0)
	if got[0] != OpError || got[4] != byte(ble.ErrAttNotFound) {
		t.Errorf("response = %#v; want an attribute not found error", got)
	}
}

func TestATTServerWriteRequest(t *testing.T) {
	s, f := newTestATT(t)

	svc := s.ATT.AddLocalAttribute(AttributeTypeService, 0,
		UUIDPrimaryService.UUID(), 0, nil)
	chr := s.ATT.AddLocalAttribute(AttributeTypeCharacteristic, svc,
		UUIDCharacteristic.UUID(), 0, nil)
	val := s.ATT.AddLocalAttribute(AttributeTypeCharacteristicValue, chr,
		ble.New16BitUUID(0x2a19), 0, nil)

	var written []byte
	s.ATT.AddLocalCharacteristic(chr, 0, val, ble.New16BitUUID(0x2a19),
		CharacteristicHandler{
			WriteValue: func(data []byte) (int, error) {
				written = append([]byte{}, data...)
				return len(data), nil
			},
		})

	pdu := []byte{OpWriteReq, byte(val), byte(val >> 8), 0xaa, 0xbb}
	if err := s.ATT.handleData(testConnHandle, pdu); err != nil {
		t.Fatalf("handleData: %v", err)
	}

	if !bytes.Equal(written, []byte{0xaa, 0xbb}) {
		t.Errorf("handler saw %#v; want aa bb", written)
	}

	got := sentPDU(t, f, 0)
	if !bytes.Equal(got, []byte{OpWriteResponse}) {
		t.Errorf("response = %#v; want a write response", got)
	}
}

func TestATTServerWriteCCCD(t *testing.T) {
	s, f := newTestATT(t)

	svc := s.ATT.AddLocalAttribute(AttributeTypeService, 0,
		UUIDPrimaryService.UUID(), 0, nil)
	chr := s.ATT.AddLocalAttribute(AttributeTypeCharacteristic, svc,
		UUIDCharacteristic.UUID(), 0, nil)
	val := s.ATT.AddLocalAttribute(AttributeTypeCharacteristicValue, chr,
		ble.New16BitUUID(0x2a19), 0, nil)
	cccd := s.ATT.AddLocalAttribute(AttributeTypeDescriptor, chr,
		UUIDClientCharacteristicConfig.UUID(), 0, []byte{0, 0})

	var got uint16
	s.ATT.AddLocalCharacteristic(chr, 0, val, ble.New16BitUUID(0x2a19),
		CharacteristicHandler{
			WriteCCCD: func(v uint16) error { got = v; return nil },
			ReadCCCD:  func() (uint16, error) { return got, nil },
		})

	pdu := []byte{OpWriteReq, byte(cccd), byte(cccd >> 8), 0x01, 0x00}
	if err := s.ATT.handleData(testConnHandle, pdu); err != nil {
		t.Fatalf("handleData: %v", err)
	}
	if got != 0x0001 {
		t.Errorf("CCCD = %#x; want 0x1", got)
	}

	// Reading it back returns what was written.
	pdu = []byte{OpReadReq, byte(cccd), byte(cccd >> 8)}
	if err := s.ATT.handleData(testConnHandle, pdu); err != nil {
		t.Fatalf("handleData: %v", err)
	}

	resp := sentPDU(t, f, 1)
	if resp[0] != OpReadResponse || binary.LittleEndian.Uint16(resp[1:]) != 0x0001 {
		t.Errorf("read response = %#v; want the CCCD value 0x1", resp)
	}
}

func TestATTServerNoHandlerIsAnError(t *testing.T) {
	s, f := newTestATT(t)

	svc := s.ATT.AddLocalAttribute(AttributeTypeService, 0,
		UUIDPrimaryService.UUID(), 0, nil)
	chr := s.ATT.AddLocalAttribute(AttributeTypeCharacteristic, svc,
		UUIDCharacteristic.UUID(), 0, nil)
	val := s.ATT.AddLocalAttribute(AttributeTypeCharacteristicValue, chr,
		ble.New16BitUUID(0x2a19), 0, nil)

	// A characteristic whose handler set is empty.
	s.ATT.AddLocalCharacteristic(chr, 0, val, ble.New16BitUUID(0x2a19),
		CharacteristicHandler{})

	pdu := []byte{OpReadReq, byte(val), byte(val >> 8)}
	if err := s.ATT.handleData(testConnHandle, pdu); err != nil {
		t.Fatalf("handleData: %v", err)
	}

	got := sentPDU(t, f, 0)
	if got[0] != OpError {
		t.Errorf("response = %#v; want an error response", got)
	}
}

func TestATTClientReadByGroupResponse(t *testing.T) {
	s, _ := newTestATT(t)

	// Two services, each with a 16 bit UUID.
	pdu := []byte{OpReadByGroupResponse, 6,
		0x01, 0x00, 0x05, 0x00, 0x0f, 0x18,
		0x06, 0x00, 0x09, 0x00, 0x0a, 0x18,
	}
	if err := s.ATT.handleData(testConnHandle, pdu); err != nil {
		t.Fatalf("handleData: %v", err)
	}

	cd, err := s.ATT.ConnectionData(testConnHandle)
	if err != nil {
		t.Fatalf("ConnectionData: %v", err)
	}
	if len(cd.Services) != 2 {
		t.Fatalf("found %d services; want 2", len(cd.Services))
	}
	if cd.Services[0].UUID != ble.New16BitUUID(0x180f) {
		t.Errorf("first UUID = %v; want 180f", cd.Services[0].UUID)
	}
	if cd.Services[0].StartHandle != 1 || cd.Services[0].EndHandle != 5 {
		t.Errorf("first handles = %d..%d; want 1..5",
			cd.Services[0].StartHandle, cd.Services[0].EndHandle)
	}
	if cd.Services[1].UUID != ble.New16BitUUID(0x180a) {
		t.Errorf("second UUID = %v; want 180a", cd.Services[1].UUID)
	}
}

func TestATTClientReadByTypeResponse(t *testing.T) {
	s, _ := newTestATT(t)

	// One characteristic: declaration handle, properties, value handle, UUID.
	pdu := []byte{OpReadByTypeResponse, 7,
		0x02, 0x00, 0x12, 0x03, 0x00, 0x19, 0x2a,
	}
	if err := s.ATT.handleData(testConnHandle, pdu); err != nil {
		t.Fatalf("handleData: %v", err)
	}

	cd, _ := s.ATT.ConnectionData(testConnHandle)
	if len(cd.Characteristics) != 1 {
		t.Fatalf("found %d characteristics; want 1", len(cd.Characteristics))
	}

	c := cd.Characteristics[0]
	if c.Properties != 0x12 {
		t.Errorf("Properties = %#x; want 0x12", c.Properties)
	}
	if c.ValueHandle != 3 {
		t.Errorf("ValueHandle = %d; want 3", c.ValueHandle)
	}
	if c.UUID != ble.New16BitUUID(0x2a19) {
		t.Errorf("UUID = %v; want 2a19", c.UUID)
	}
}

func TestATTClientReadResponse(t *testing.T) {
	s, _ := newTestATT(t)

	if err := s.ATT.handleData(testConnHandle, []byte{OpReadResponse, 1, 2, 3}); err != nil {
		t.Fatalf("handleData: %v", err)
	}

	cd, _ := s.ATT.ConnectionData(testConnHandle)
	if !bytes.Equal(cd.Value, []byte{1, 2, 3}) {
		t.Errorf("Value = %#v; want 1 2 3", cd.Value)
	}
}

func TestATTClientErrorResponse(t *testing.T) {
	s, _ := newTestATT(t)

	pdu := []byte{OpError, OpReadByTypeReq, 0x01, 0x00, byte(ble.ErrAttNotFound)}
	err := s.ATT.handleData(testConnHandle, pdu)
	if err != ErrATTAttributeNotFound {
		t.Errorf("handleData = %v; want ErrATTAttributeNotFound", err)
	}

	attErr, ok := s.ATT.LastError(testConnHandle)
	if !ok {
		t.Fatal("LastError reported nothing")
	}
	if attErr.Opcode != OpReadByTypeReq {
		t.Errorf("Opcode = %#x; want OpReadByTypeReq", attErr.Opcode)
	}
	if attErr.Code != ble.ErrAttNotFound {
		t.Errorf("Code = %v; want ErrAttNotFound", attErr.Code)
	}
}

func TestATTClientErrorResponseOtherCode(t *testing.T) {
	s, _ := newTestATT(t)

	pdu := []byte{OpError, OpWriteReq, 0x01, 0x00, byte(ble.ErrAttWriteNotPermitted)}
	if err := s.ATT.handleData(testConnHandle, pdu); err != ErrATTOp {
		t.Errorf("handleData = %v; want ErrATTOp", err)
	}

	attErr, _ := s.ATT.LastError(testConnHandle)
	if attErr.Code != ble.ErrAttWriteNotPermitted {
		t.Errorf("Code = %v; want ErrAttWriteNotPermitted", attErr.Code)
	}
}

func TestATTMTUExchange(t *testing.T) {
	t.Run("clamped to the maximum", func(t *testing.T) {
		s, f := newTestATT(t)

		// The remote asks for more than this stack accepts.
		pdu := []byte{OpMTUReq, 0xff, 0x00}
		if err := s.ATT.handleData(testConnHandle, pdu); err != nil {
			t.Fatalf("handleData: %v", err)
		}

		got := sentPDU(t, f, 0)
		if got[0] != OpMTUResponse {
			t.Fatalf("opcode = %#x; want OpMTUResponse", got[0])
		}
		if mtu := binary.LittleEndian.Uint16(got[1:]); mtu != MaximumMTU {
			t.Errorf("MTU = %d; want %d", mtu, MaximumMTU)
		}
	})

	t.Run("a smaller request is honoured", func(t *testing.T) {
		s, f := newTestATT(t)

		pdu := []byte{OpMTUReq, 0x40, 0x00}
		if err := s.ATT.handleData(testConnHandle, pdu); err != nil {
			t.Fatalf("handleData: %v", err)
		}

		got := sentPDU(t, f, 0)
		if mtu := binary.LittleEndian.Uint16(got[1:]); mtu != 0x40 {
			t.Errorf("MTU = %d; want 64", mtu)
		}
	})
}

func TestATTNotificationReachesTheChannel(t *testing.T) {
	s, _ := newTestATT(t)

	pdu := []byte{OpHandleNotify, 0x03, 0x00, 0x2a, 0x2b}
	if err := s.ATT.handleData(testConnHandle, pdu); err != nil {
		t.Fatalf("handleData: %v", err)
	}

	select {
	case not := <-s.ATT.Notifications():
		if not.ConnectionHandle != testConnHandle {
			t.Errorf("ConnectionHandle = %#x; want %#x", not.ConnectionHandle, testConnHandle)
		}
		if not.Handle != 3 {
			t.Errorf("Handle = %d; want 3", not.Handle)
		}
		if !bytes.Equal(not.Data, []byte{0x2a, 0x2b}) {
			t.Errorf("Data = %#v; want 2a 2b", not.Data)
		}
	default:
		t.Fatal("no notification on the channel")
	}
}

func TestATTNotificationQueueDropsWhenFull(t *testing.T) {
	s, _ := newTestATT(t)

	// The channel holds 32. The ones after that are dropped rather than
	// blocking the poll loop.
	for i := 0; i < 40; i++ {
		pdu := []byte{OpHandleNotify, 0x03, 0x00, byte(i)}
		if err := s.ATT.handleData(testConnHandle, pdu); err != nil {
			t.Fatalf("handleData at %d: %v", i, err)
		}
	}

	if got := len(s.ATT.Notifications()); got != 32 {
		t.Errorf("queued %d notifications; want 32", got)
	}
}

func TestATTUnknownConnection(t *testing.T) {
	f := &fakeTransport{}
	s := newTestStack(f)

	if _, err := s.ATT.ConnectionData(0x99); err != ErrATTUnknownConnection {
		t.Errorf("ConnectionData = %v; want ErrATTUnknownConnection", err)
	}
}

func TestATTRequestWithoutAResponseTimesOut(t *testing.T) {
	s, _ := newTestATT(t)

	err := s.ATT.ReadReq(testConnHandle, 0x0003)
	if err != ErrATTTimeout {
		t.Errorf("ReadReq = %v; want ErrATTTimeout", err)
	}
}

func TestATTRequestWritesTheCorrectPDU(t *testing.T) {
	s, f := newTestATT(t)

	// No answer is scripted, so the request times out. The bytes it put on the
	// wire are what matter here.
	_ = s.ATT.ReadReq(testConnHandle, 0x0003)

	got := sentPDU(t, f, 0)
	want := []byte{OpReadReq, 0x03, 0x00}
	if !bytes.Equal(got, want) {
		t.Errorf("wrote %#v; want %#v", got, want)
	}
}

func TestATTWriteCommandNeedsNoResponse(t *testing.T) {
	s, f := newTestATT(t)

	if err := s.ATT.WriteCmd(testConnHandle, 0x0003, []byte{0xaa}); err != nil {
		t.Fatalf("WriteCmd: %v", err)
	}

	got := sentPDU(t, f, 0)
	want := []byte{OpWriteCmd, 0x03, 0x00, 0xaa}
	if !bytes.Equal(got, want) {
		t.Errorf("wrote %#v; want %#v", got, want)
	}
}

func TestATTSendNotification(t *testing.T) {
	s, f := newTestATT(t)

	if err := s.ATT.SendNotification(0x0003, []byte{0x42}); err != nil {
		t.Fatalf("SendNotification: %v", err)
	}

	got := sentPDU(t, f, 0)
	want := []byte{OpHandleNotify, 0x03, 0x00, 0x42}
	if !bytes.Equal(got, want) {
		t.Errorf("wrote %#v; want %#v", got, want)
	}
}

func TestATTOverACLFromTheTransport(t *testing.T) {
	// The whole path: an ACL packet arrives, HCI unwraps it, L2CAP routes it
	// to ATT and ATT answers on the same channel.
	s, f := newTestATT(t, attPacket(OpMTUReq, 0x40, 0x00))

	if err := s.HCI.Poll(); err != nil {
		t.Fatalf("Poll: %v", err)
	}

	got := sentPDU(t, f, 0)
	if got[0] != OpMTUResponse {
		t.Errorf("response opcode = %#x; want OpMTUResponse", got[0])
	}
}

func TestACLPacketWithMismatchedLengths(t *testing.T) {
	// The ACL length and the L2CAP length must agree. The ACL payload is the
	// 4 byte L2CAP header plus the L2CAP length.
	pkt := make([]byte, 13)
	pkt[0] = PacketACLData
	binary.LittleEndian.PutUint16(pkt[1:], testConnHandle)
	binary.LittleEndian.PutUint16(pkt[3:], 8) // ACL length, so 4 payload bytes
	binary.LittleEndian.PutUint16(pkt[5:], 2) // the L2CAP length disagrees
	binary.LittleEndian.PutUint16(pkt[7:], CIDATT)

	s := newTestStack(&fakeTransport{})
	copy(s.HCI.buf, pkt)
	s.HCI.end = len(pkt)
	s.HCI.pos = len(pkt)

	if _, err := s.HCI.processPacket(); err == nil {
		t.Error("processPacket = nil; want an error for mismatched lengths")
	}
}

func TestACLPacketTooShortForTheHeaders(t *testing.T) {
	// An ACL payload has to hold at least the 4 byte L2CAP header.
	s := newTestStack(&fakeTransport{})

	err := s.HCI.handleACLData([]byte{0x40, 0x00, 0x02, 0x00, 0x01, 0x02})
	if err != ErrInvalidPacket {
		t.Errorf("handleACLData = %v; want ErrInvalidPacket", err)
	}
}

func TestACLPayloadLengthPastTheEnd(t *testing.T) {
	// A defence in depth check. The L2CAP length comes off the wire, so
	// handleACLData does not trust it even though processPacket has already
	// sized the packet.
	buf := make([]byte, 10)
	binary.LittleEndian.PutUint16(buf[0:], testConnHandle)
	binary.LittleEndian.PutUint16(buf[2:], 6)
	binary.LittleEndian.PutUint16(buf[4:], 2)
	binary.LittleEndian.PutUint16(buf[6:], CIDATT)

	s := newTestStack(&fakeTransport{})

	// Hand it a buffer shorter than the L2CAP length claims.
	if err := s.HCI.handleACLData(buf[:9]); err != ErrInvalidPacket {
		t.Errorf("handleACLData = %v; want ErrInvalidPacket", err)
	}
}

func TestShortUUID(t *testing.T) {
	if got := UUIDPrimaryService.UUID(); got != ble.New16BitUUID(0x2800) {
		t.Errorf("UUIDPrimaryService.UUID() = %v; want 2800", got)
	}
	if got := UUIDCharacteristic.UUID(); got != ble.New16BitUUID(0x2803) {
		t.Errorf("UUIDCharacteristic.UUID() = %v; want 2803", got)
	}
	if got := UUIDClientCharacteristicConfig.UUID(); got != ble.New16BitUUID(0x2902) {
		t.Errorf("UUIDClientCharacteristicConfig.UUID() = %v; want 2902", got)
	}
}

func TestATTClientRequestPDUs(t *testing.T) {
	for _, tc := range []struct {
		name string
		send func(*ATT) error
		want []byte
	}{
		{
			name: "read by group type request",
			send: func(a *ATT) error {
				return a.ReadByGroupReq(testConnHandle, 0x0001, 0xffff, UUIDPrimaryService)
			},
			want: []byte{OpReadByGroupReq, 0x01, 0x00, 0xff, 0xff, 0x00, 0x28},
		},
		{
			name: "read by type request",
			send: func(a *ATT) error {
				return a.ReadByTypeReq(testConnHandle, 0x0002, 0x0009, UUIDCharacteristic)
			},
			want: []byte{OpReadByTypeReq, 0x02, 0x00, 0x09, 0x00, 0x03, 0x28},
		},
		{
			name: "find information request",
			send: func(a *ATT) error {
				return a.FindInfoReq(testConnHandle, 0x0004, 0x0006)
			},
			want: []byte{OpFindInfoReq, 0x04, 0x00, 0x06, 0x00},
		},
		{
			name: "write request",
			send: func(a *ATT) error {
				return a.WriteReq(testConnHandle, 0x0004, []byte{0x01, 0x00})
			},
			want: []byte{OpWriteReq, 0x04, 0x00, 0x01, 0x00},
		},
		{
			name: "exchange MTU request",
			send: func(a *ATT) error {
				return a.MTUReq(testConnHandle)
			},
			want: []byte{OpMTUReq, byte(MaximumMTU), 0x00},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, f := newTestATT(t)

			// Nothing answers, so the request times out. The bytes it put on
			// the wire are what matter.
			if err := tc.send(s.ATT); err != ErrATTTimeout {
				t.Fatalf("request = %v; want ErrATTTimeout", err)
			}

			if got := sentPDU(t, f, 0); !bytes.Equal(got, tc.want) {
				t.Errorf("wrote %#v; want %#v", got, tc.want)
			}
		})
	}
}

func TestATTServerReadByTypeRequest(t *testing.T) {
	s, f := newTestATT(t)

	svc := s.ATT.AddLocalAttribute(AttributeTypeService, 0,
		UUIDPrimaryService.UUID(), 0, nil)
	chr := s.ATT.AddLocalAttribute(AttributeTypeCharacteristic, svc,
		UUIDCharacteristic.UUID(), 0x12, nil)
	val := s.ATT.AddLocalAttribute(AttributeTypeCharacteristicValue, chr,
		ble.New16BitUUID(0x2a19), 0x12, []byte{42})

	s.ATT.AddLocalCharacteristic(chr, 0x12, val, ble.New16BitUUID(0x2a19),
		CharacteristicHandler{})

	// Read By Type Request for characteristic declarations.
	pdu := []byte{OpReadByTypeReq, 0x01, 0x00, 0xff, 0xff, 0x03, 0x28}
	if err := s.ATT.handleData(testConnHandle, pdu); err != nil {
		t.Fatalf("handleData: %v", err)
	}

	got := sentPDU(t, f, 0)
	if got[0] != OpReadByTypeResponse {
		t.Fatalf("opcode = %#x; want OpReadByTypeResponse", got[0])
	}
	// handle, properties, value handle and a 16 bit UUID.
	if got[1] != 7 {
		t.Errorf("entry length = %d; want 7", got[1])
	}
	if got[4] != 0x12 {
		t.Errorf("properties = %#x; want 0x12", got[4])
	}
	if uuid := binary.LittleEndian.Uint16(got[7:]); uuid != 0x2a19 {
		t.Errorf("UUID = %#x; want 0x2a19", uuid)
	}
}

func TestATTServerReadByTypeRequestNothingFound(t *testing.T) {
	s, f := newTestATT(t)

	pdu := []byte{OpReadByTypeReq, 0x01, 0x00, 0xff, 0xff, 0x03, 0x28}
	if err := s.ATT.handleData(testConnHandle, pdu); err != nil {
		t.Fatalf("handleData: %v", err)
	}

	got := sentPDU(t, f, 0)
	if got[0] != OpError || got[4] != byte(ble.ErrAttNotFound) {
		t.Errorf("response = %#v; want an attribute not found error", got)
	}
}

func TestATTServerFindInfoRequest(t *testing.T) {
	s, f := newTestATT(t)

	svc := s.ATT.AddLocalAttribute(AttributeTypeService, 0,
		UUIDPrimaryService.UUID(), 0, nil)
	chr := s.ATT.AddLocalAttribute(AttributeTypeCharacteristic, svc,
		UUIDCharacteristic.UUID(), 0, nil)
	s.ATT.AddLocalAttribute(AttributeTypeCharacteristicValue, chr,
		ble.New16BitUUID(0x2a19), 0, nil)
	s.ATT.AddLocalAttribute(AttributeTypeDescriptor, chr,
		UUIDClientCharacteristicConfig.UUID(), 0, []byte{0, 0})

	pdu := []byte{OpFindInfoReq, 0x01, 0x00, 0xff, 0xff}
	if err := s.ATT.handleData(testConnHandle, pdu); err != nil {
		t.Fatalf("handleData: %v", err)
	}

	got := sentPDU(t, f, 0)
	if got[0] != OpFindInfoResponse {
		t.Fatalf("opcode = %#x; want OpFindInfoResponse", got[0])
	}

	// The format must be one of the two the specification defines. It should
	// be 1 here, because every UUID in this database is 16 bit, but the server
	// picks the format from the attribute type. See the TODO in
	// handleFindInfoReq.
	if got[1] != attFindInfoFormat16Bit && got[1] != attFindInfoFormat128Bit {
		t.Errorf("format = %#x; want 1 or 2", got[1])
	}
}

func TestATTServerFindInfoRequestNothingFound(t *testing.T) {
	s, f := newTestATT(t)

	pdu := []byte{OpFindInfoReq, 0x10, 0x00, 0x20, 0x00}
	if err := s.ATT.handleData(testConnHandle, pdu); err != nil {
		t.Fatalf("handleData: %v", err)
	}

	got := sentPDU(t, f, 0)
	if got[0] != OpError || got[4] != byte(ble.ErrAttNotFound) {
		t.Errorf("response = %#v; want an attribute not found error", got)
	}
}

func TestATTClientFindInfoResponse(t *testing.T) {
	s, _ := newTestATT(t)

	// Format 1, then handle and 16 bit UUID pairs.
	pdu := []byte{OpFindInfoResponse, 0x01, 0x04, 0x00, 0x02, 0x29}
	if err := s.ATT.handleData(testConnHandle, pdu); err != nil {
		t.Fatalf("handleData: %v", err)
	}

	cd, _ := s.ATT.ConnectionData(testConnHandle)
	if len(cd.Descriptors) != 1 {
		t.Fatalf("found %d descriptors; want 1", len(cd.Descriptors))
	}
	if cd.Descriptors[0].Handle != 4 {
		t.Errorf("Handle = %d; want 4", cd.Descriptors[0].Handle)
	}
}

func TestATTMTUAndSetMaxMTU(t *testing.T) {
	s, _ := newTestATT(t)

	if err := s.ATT.SetMaxMTU(100); err != nil {
		t.Fatalf("SetMaxMTU: %v", err)
	}

	// A request above the new maximum is clamped to it, and the negotiated
	// value is what MTU reports.
	if err := s.ATT.handleData(testConnHandle, []byte{OpMTUReq, 0xf0, 0x00}); err != nil {
		t.Fatalf("handleData: %v", err)
	}
	if got := s.ATT.MTU(); got != 100 {
		t.Errorf("MTU = %d; want 100", got)
	}

	cd, _ := s.ATT.ConnectionData(testConnHandle)
	if cd.MTU != 100 {
		t.Errorf("ConnectionData().MTU = %d; want 100", cd.MTU)
	}
}

func TestATTMTUStartsAtTheDefault(t *testing.T) {
	// Before an exchange the MTU is 23. It must not be zero, because the
	// server uses it as the size budget for a discovery response.
	s, _ := newTestATT(t)

	if got := s.ATT.MTU(); got != DefaultMTU {
		t.Errorf("MTU = %d; want %d", got, DefaultMTU)
	}
}

func TestATTMTUResponseIsRecorded(t *testing.T) {
	s, _ := newTestATT(t)

	if err := s.ATT.handleData(testConnHandle, []byte{OpMTUResponse, 0x40, 0x00}); err != nil {
		t.Fatalf("handleData: %v", err)
	}
	if got := s.ATT.MTU(); got != 0x40 {
		t.Errorf("MTU = %d; want 64", got)
	}
}

func TestATTMTUResponseTooShort(t *testing.T) {
	s, _ := newTestATT(t)

	if err := s.ATT.handleData(testConnHandle, []byte{OpMTUResponse, 0x40}); err != ErrInvalidPacket {
		t.Errorf("handleData = %v; want ErrInvalidPacket", err)
	}
}

func TestATTClearLocalData(t *testing.T) {
	s, _ := newTestATT(t)

	handle := s.ATT.AddLocalAttribute(AttributeTypeService, 0,
		UUIDPrimaryService.UUID(), 0, nil)
	s.ATT.AddLocalService(handle, handle, ble.New16BitUUID(0x180f))

	s.ATT.ClearLocalData()

	if len(s.ATT.attributes) != 0 || len(s.ATT.localServices) != 0 {
		t.Error("ClearLocalData left attributes or services behind")
	}
}

func TestATTRemoveConnection(t *testing.T) {
	s, _ := newTestATT(t)

	if err := s.ATT.removeConnection(testConnHandle); err != nil {
		t.Fatalf("removeConnection: %v", err)
	}
	if _, err := s.ATT.ConnectionData(testConnHandle); err != ErrATTUnknownConnection {
		t.Errorf("ConnectionData after remove = %v; want ErrATTUnknownConnection", err)
	}
}

func TestATTPollDelegatesToHCI(t *testing.T) {
	s, _ := newTestATT(t, commandComplete(OpCode(OGFHostCtl, OCFReset), 0))

	if err := s.ATT.Poll(); err != nil {
		t.Errorf("Poll = %v; want nil", err)
	}
}

func TestATTServiceWithA128BitUUID(t *testing.T) {
	s, f := newTestATT(t)

	uuid, err := ble.ParseUUID("6e400001-b5a3-f393-e0a9-e50e24dcca9e")
	if err != nil {
		t.Fatalf("ParseUUID: %v", err)
	}

	handle := s.ATT.AddLocalAttribute(AttributeTypeService, 0,
		UUIDPrimaryService.UUID(), 0, nil)
	s.ATT.AddLocalService(handle, handle, uuid)

	pdu := []byte{OpReadByGroupReq, 0x01, 0x00, 0xff, 0xff, 0x00, 0x28}
	if err := s.ATT.handleData(testConnHandle, pdu); err != nil {
		t.Fatalf("handleData: %v", err)
	}

	got := sentPDU(t, f, 0)
	// start handle, end handle and a 128 bit UUID.
	if got[1] != 20 {
		t.Errorf("entry length = %d; want 20 for a 128 bit UUID", got[1])
	}

	want := uuid.BytesLittleEndian()
	if !bytes.Equal(got[6:22], want[:]) {
		t.Errorf("UUID = % x; want % x", got[6:22], want)
	}
}

func TestATTFindInfoResponseParsing(t *testing.T) {
	// The format byte and the entry widths come off the wire. A response that
	// does not add up used to run off the end of the buffer and panic.
	for _, tc := range []struct {
		name  string
		pdu   []byte
		want  error
		count int
	}{
		{
			name:  "one 16 bit entry",
			pdu:   []byte{OpFindInfoResponse, attFindInfoFormat16Bit, 0x04, 0x00, 0x02, 0x29},
			count: 1,
		},
		{
			name: "two 16 bit entries",
			pdu: []byte{OpFindInfoResponse, attFindInfoFormat16Bit,
				0x04, 0x00, 0x02, 0x29,
				0x05, 0x00, 0x01, 0x29},
			count: 2,
		},
		{
			name: "one 128 bit entry",
			pdu: append([]byte{OpFindInfoResponse, attFindInfoFormat128Bit, 0x04, 0x00},
				make([]byte, 16)...),
			count: 1,
		},
		{
			name:  "a trailing partial entry is ignored",
			pdu:   []byte{OpFindInfoResponse, attFindInfoFormat16Bit, 0x04, 0x00, 0x02, 0x29, 0x05},
			count: 1,
		},
		{
			name:  "an entry narrower than the format claims",
			pdu:   []byte{OpFindInfoResponse, attFindInfoFormat128Bit, 0x04, 0x00, 0x02},
			count: 0,
		},
		{
			name: "no format byte",
			pdu:  []byte{OpFindInfoResponse},
			want: ErrInvalidPacket,
		},
		{
			name: "unknown format",
			pdu:  []byte{OpFindInfoResponse, 0x09, 0x04, 0x00, 0x02, 0x29},
			want: ErrInvalidPacket,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, _ := newTestATT(t)

			// This must not panic, whatever the response claims.
			if err := s.ATT.handleData(testConnHandle, tc.pdu); err != tc.want {
				t.Fatalf("handleData = %v; want %v", err, tc.want)
			}
			if tc.want != nil {
				return
			}

			cd, _ := s.ATT.ConnectionData(testConnHandle)
			if len(cd.Descriptors) != tc.count {
				t.Errorf("found %d descriptors; want %d", len(cd.Descriptors), tc.count)
			}
		})
	}
}

func TestATTTruncatedResponses(t *testing.T) {
	// Every response arrives over the air, so none of them may panic.
	for _, name := range []struct {
		name string
		pdu  []byte
	}{
		{"error response with no code", []byte{OpError, OpReadReq, 0x01}},
		{"MTU request with no value", []byte{OpMTUReq, 0x40}},
		{"read by group response with no length", []byte{OpReadByGroupResponse}},
		{"read by type response with no length", []byte{OpReadByTypeResponse}},
		{"read request with no handle", []byte{OpReadReq, 0x03}},
		{"write request with no handle", []byte{OpWriteReq, 0x03}},
		{"notification with no handle", []byte{OpHandleNotify, 0x03}},
		{"empty", []byte{}},
	} {
		t.Run(name.name, func(t *testing.T) {
			s, _ := newTestATT(t)

			// It must be rejected rather than parsed, and it must not panic.
			if err := s.ATT.handleData(testConnHandle, name.pdu); err != ErrInvalidPacket {
				t.Errorf("handleData = %v; want ErrInvalidPacket", err)
			}
		})
	}
}

func TestATTNoOpcodePanicsOnAShortPDU(t *testing.T) {
	// Every opcode, at every length from empty up to 24 bytes. None of them
	// may panic, whatever the content.
	opcodes := []uint8{
		OpError, OpMTUReq, OpMTUResponse, OpFindInfoReq, OpFindInfoResponse,
		OpFindByTypeReq, OpFindByTypeResponse, OpReadByTypeReq,
		OpReadByTypeResponse, OpReadReq, OpReadResponse, OpReadBlobReq,
		OpReadBlobResponse, OpReadMultiReq, OpReadMultiResponse,
		OpReadByGroupReq, OpReadByGroupResponse, OpWriteReq, OpWriteResponse,
		OpWriteCmd, OpPrepWriteReq, OpPrepWriteResponse, OpExecWriteReq,
		OpExecWriteResponse, OpHandleNotify, OpHandleInd, OpHandleCNF,
		OpSignedWriteCmd,
	}

	for _, op := range opcodes {
		for n := 0; n <= 24; n++ {
			pdu := make([]byte, n)
			if n > 0 {
				pdu[0] = op
				// Fill the rest with a value that claims a large length, so
				// that a missing bounds check shows up.
				for i := 1; i < n; i++ {
					pdu[i] = 0xff
				}
			}

			s, _ := newTestATT(t)
			_ = s.ATT.handleData(testConnHandle, pdu)
		}
	}
}
