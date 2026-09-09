package hci

import (
	"bytes"
	"encoding/binary"
	"testing"
)

// signalingPDU returns the L2CAP signalling payload of the nth write.
func signalingPDU(t *testing.T, f *fakeTransport, n int) []byte {
	t.Helper()

	pkt := f.tx[n]
	if cid := binary.LittleEndian.Uint16(pkt[7:]); cid != CIDSignaling {
		t.Fatalf("write %d has CID %#x; want the signalling channel", n, cid)
	}

	return pkt[9:]
}

func TestL2CAPParameterUpdateRequestIsAnswered(t *testing.T) {
	f := &fakeTransport{}
	s := newTestStack(f)

	// code, identifier, length, then min and max interval, latency, timeout.
	pdu := []byte{connectionParamUpdateRequest, 0x05, 0x08, 0x00,
		0x06, 0x00, 0x0c, 0x00, 0x00, 0x00, 0xc8, 0x00}

	if err := s.HCI.l2cap.handleData(testConnHandle, pdu); err != nil {
		t.Fatalf("handleData: %v", err)
	}

	got := signalingPDU(t, f, 0)
	// code, identifier echoed, length 2, result 0 for accepted.
	want := []byte{connectionParamUpdateResponse, 0x05, 0x02, 0x00, 0x00, 0x00}
	if !bytes.Equal(got, want) {
		t.Errorf("response = %#v; want %#v", got, want)
	}
}

func TestL2CAPParameterUpdateResponseIsAccepted(t *testing.T) {
	f := &fakeTransport{}
	s := newTestStack(f)

	pdu := []byte{connectionParamUpdateResponse, 0x05, 0x02, 0x00, 0x00, 0x00}
	if err := s.HCI.l2cap.handleData(testConnHandle, pdu); err != nil {
		t.Fatalf("handleData: %v", err)
	}
	if len(f.tx) != 0 {
		t.Errorf("wrote %d packets in reply to a response; want 0", len(f.tx))
	}
}

func TestL2CAPUnknownCodeIsIgnored(t *testing.T) {
	f := &fakeTransport{}
	s := newTestStack(f)

	if err := s.HCI.l2cap.handleData(testConnHandle, []byte{0x01, 0x05, 0x00, 0x00}); err != nil {
		t.Errorf("handleData = %v; want nil", err)
	}
	if len(f.tx) != 0 {
		t.Errorf("wrote %d packets for an unknown code; want 0", len(f.tx))
	}
}

func TestL2CAPAddConnection(t *testing.T) {
	t.Run("the peripheral asks for its parameters", func(t *testing.T) {
		f := &fakeTransport{}
		s := newTestStack(f)

		if err := s.HCI.l2cap.addConnection(testConnHandle, 0x01, 0x0006, 0x00c8); err != nil {
			t.Fatalf("addConnection: %v", err)
		}

		got := signalingPDU(t, f, 0)
		if got[0] != connectionParamUpdateRequest {
			t.Fatalf("code = %#x; want a parameter update request", got[0])
		}
		if interval := binary.LittleEndian.Uint16(got[4:]); interval != 0x0006 {
			t.Errorf("min interval = %#x; want 0x6", interval)
		}
		if timeout := binary.LittleEndian.Uint16(got[10:]); timeout != 0x00c8 {
			t.Errorf("timeout = %#x; want 0xc8", timeout)
		}
	})

	t.Run("a zero timeout falls back to the default", func(t *testing.T) {
		f := &fakeTransport{}
		s := newTestStack(f)

		if err := s.HCI.l2cap.addConnection(testConnHandle, 0x01, 0x0006, 0); err != nil {
			t.Fatalf("addConnection: %v", err)
		}

		got := signalingPDU(t, f, 0)
		if timeout := binary.LittleEndian.Uint16(got[10:]); timeout != 0x00c8 {
			t.Errorf("timeout = %#x; want the default 0xc8", timeout)
		}
	})

	t.Run("the central sends nothing", func(t *testing.T) {
		f := &fakeTransport{}
		s := newTestStack(f)

		if err := s.HCI.l2cap.addConnection(testConnHandle, 0x00, 0x0006, 0x00c8); err != nil {
			t.Fatalf("addConnection: %v", err)
		}
		if len(f.tx) != 0 {
			t.Errorf("the central wrote %d packets; want 0", len(f.tx))
		}
	})
}

func TestL2CAPParameterRequestTooShort(t *testing.T) {
	f := &fakeTransport{}
	s := newTestStack(f)

	// The request needs 8 bytes of parameters and carries 4.
	pdu := []byte{connectionParamUpdateRequest, 0x05, 0x08, 0x00, 0x06, 0x00, 0x0c, 0x00}
	if err := s.HCI.l2cap.handleData(testConnHandle, pdu); err != errInvalidPayloadLength {
		t.Errorf("handleData = %v; want errInvalidPayloadLength", err)
	}
}

func TestL2CAPOverACLFromTheTransport(t *testing.T) {
	// A signalling packet arrives from the controller and is answered.
	pdu := []byte{connectionParamUpdateRequest, 0x05, 0x08, 0x00,
		0x06, 0x00, 0x0c, 0x00, 0x00, 0x00, 0xc8, 0x00}

	f := &fakeTransport{rx: [][]byte{aclPacket(testConnHandle, CIDSignaling, pdu)}}
	s := newTestStack(f)

	if err := s.HCI.Poll(); err != nil {
		t.Fatalf("Poll: %v", err)
	}

	got := signalingPDU(t, f, 0)
	if got[0] != connectionParamUpdateResponse {
		t.Errorf("code = %#x; want a parameter update response", got[0])
	}
}

func TestUnknownCIDIsIgnored(t *testing.T) {
	// A channel this stack does not use must not be an error.
	f := &fakeTransport{rx: [][]byte{aclPacket(testConnHandle, 0x0099, []byte{1, 2, 3, 4})}}
	s := newTestStack(f)

	if err := s.HCI.Poll(); err != nil {
		t.Errorf("Poll = %v; want nil", err)
	}
}

func TestNoL2CAPPacketPanicsOnAShortPayload(t *testing.T) {
	for _, code := range []uint8{connectionParamUpdateRequest,
		connectionParamUpdateResponse, 0x00, 0xff} {
		for n := 0; n <= 20; n++ {
			buf := make([]byte, n)
			if n > 0 {
				buf[0] = code
			}
			for i := 1; i < n; i++ {
				buf[i] = 0xff
			}

			s := newTestStack(&fakeTransport{})
			_ = s.HCI.l2cap.handleData(testConnHandle, buf)
		}
	}
}
