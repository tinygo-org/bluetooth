package hci

import (
	"errors"
	"testing"
	"time"
)

// errFakeRead is returned by a fake transport told to fail a read.
var errFakeRead = errors.New("fake transport read error")

// fakeTransport serves scripted packets and records what was written. Each
// entry in rx is one thing the controller offers, so a stream transport hands
// out its bytes as they fit and a packet transport hands out a whole entry or
// nothing.
type fakeTransport struct {
	// rx holds what the controller has to offer, front first.
	rx [][]byte

	// tx holds each write, in order.
	tx [][]byte

	// packet makes the transport packet oriented, like the CYW43439 SDIO
	// ring. It refuses a read that cannot take the whole rounded up entry and
	// rounds its copy up to 4 bytes.
	packet bool

	// readErr, when set, fails the next read.
	readErr error

	// writeErr, when set, fails the next write.
	writeErr error

	starts, ends int
}

func (f *fakeTransport) StartRead() { f.starts++ }
func (f *fakeTransport) EndRead()   { f.ends++ }

func (f *fakeTransport) Buffered() int {
	if len(f.rx) == 0 {
		return 0
	}

	return len(f.rx[0])
}

func (f *fakeTransport) Read(buf []byte) (int, error) {
	if f.readErr != nil {
		err := f.readErr
		f.readErr = nil
		return 0, err
	}

	if len(f.rx) == 0 {
		return 0, nil
	}

	front := f.rx[0]

	if f.packet {
		// A packet transport copies the whole entry, rounded up to 4 bytes, so
		// it fails outright when the destination is too small.
		if len(buf) < alignUp4(len(front)) {
			return 0, errFakeRead
		}

		n := copy(buf, front)
		f.rx = f.rx[1:]

		return n, nil
	}

	n := copy(buf, front)
	if n == len(front) {
		f.rx = f.rx[1:]
	} else {
		f.rx[0] = front[n:]
	}

	return n, nil
}

func (f *fakeTransport) Write(buf []byte) (int, error) {
	if f.writeErr != nil {
		err := f.writeErr
		f.writeErr = nil
		return 0, err
	}

	f.tx = append(f.tx, append([]byte{}, buf...))

	return len(buf), nil
}

// newTestStack returns a stack on a fake transport, with the timeouts short
// enough that a timeout path finishes at once.
func newTestStack(t *fakeTransport) *Stack {
	s := NewStack(t)
	s.HCI.commandTimeout = time.Millisecond
	s.HCI.responseTimeout = time.Millisecond
	s.HCI.retryDelay = 0

	return s
}

// event wraps event parameters in an HCI event packet.
func event(code uint8, params ...byte) []byte {
	return append([]byte{PacketEvent, code, byte(len(params))}, params...)
}

// leEvent wraps LE meta subevent parameters in an HCI event packet.
func leEvent(subevent uint8, params ...byte) []byte {
	return event(EventLEMeta, append([]byte{subevent}, params...)...)
}

// commandComplete is a Command Complete event for an opcode, with a status and
// any return parameters.
func commandComplete(opcode uint16, status uint8, ret ...byte) []byte {
	params := []byte{1, byte(opcode), byte(opcode >> 8), status}

	return event(EventCmdComplete, append(params, ret...)...)
}

func TestFakeTransportStream(t *testing.T) {
	f := &fakeTransport{rx: [][]byte{{1, 2, 3, 4, 5}}}

	buf := make([]byte, 3)
	n, err := f.Read(buf)
	if err != nil || n != 3 {
		t.Fatalf("first read = %d, %v; want 3, nil", n, err)
	}
	if f.Buffered() != 2 {
		t.Errorf("Buffered after partial read = %d; want 2", f.Buffered())
	}

	n, err = f.Read(buf)
	if err != nil || n != 2 {
		t.Fatalf("second read = %d, %v; want 2, nil", n, err)
	}
	if f.Buffered() != 0 {
		t.Errorf("Buffered after full read = %d; want 0", f.Buffered())
	}
}

func TestFakeTransportPacket(t *testing.T) {
	f := &fakeTransport{packet: true, rx: [][]byte{{1, 2, 3, 4, 5}}}

	// 5 bytes round up to 8, so a 5 byte destination is not enough.
	if _, err := f.Read(make([]byte, 5)); err == nil {
		t.Error("read into a short buffer succeeded; want an error")
	}

	n, err := f.Read(make([]byte, 8))
	if err != nil || n != 5 {
		t.Fatalf("read = %d, %v; want 5, nil", n, err)
	}
}

func TestAlignUp4(t *testing.T) {
	for _, tc := range []struct{ in, want int }{
		{0, 0}, {1, 4}, {3, 4}, {4, 4}, {5, 8}, {255, 256}, {256, 256},
	} {
		if got := alignUp4(tc.in); got != tc.want {
			t.Errorf("alignUp4(%d) = %d; want %d", tc.in, got, tc.want)
		}
	}
}

func TestReadBufferSizeHoldsLargestPacket(t *testing.T) {
	// The read buffer must be able to hold the largest packet that can
	// legitimately arrive, plus whatever a transport adds on top.
	if ReadBufferSize < MaxPacketSize+TransportOverhead {
		t.Errorf("ReadBufferSize = %d; too small for MaxPacketSize %d plus overhead %d",
			ReadBufferSize, MaxPacketSize, TransportOverhead)
	}
}
