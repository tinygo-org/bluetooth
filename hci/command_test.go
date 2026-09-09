package hci

import (
	"bytes"
	"encoding/binary"
	"testing"

	"tinygo.org/x/bluetooth/ble"
)

// answering returns a stack whose transport answers the given opcode with a
// successful Command Complete event, so that a command that waits completes.
func answering(opcode uint16, ret ...byte) (*Stack, *fakeTransport) {
	f := &fakeTransport{rx: [][]byte{commandComplete(opcode, 0, ret...)}}

	return newTestStack(f), f
}

// wantCommand checks the opcode and the parameters of the nth written command.
func wantCommand(t *testing.T, f *fakeTransport, n int, opcode uint16, params ...byte) {
	t.Helper()

	if len(f.tx) <= n {
		t.Fatalf("only %d packets written; want at least %d", len(f.tx), n+1)
	}

	want := append([]byte{PacketCommand, byte(opcode), byte(opcode >> 8), byte(len(params))}, params...)
	if !bytes.Equal(f.tx[n], want) {
		t.Errorf("wrote %#v; want %#v", f.tx[n], want)
	}
}

func TestSetEventMask(t *testing.T) {
	opcode := OpCode(OGFHostCtl, OCFSetEventMask)
	s, f := answering(opcode)

	if err := s.HCI.SetEventMask(0x3FFFFFFFFFFFFFFF); err != nil {
		t.Fatalf("SetEventMask: %v", err)
	}
	wantCommand(t, f, 0, opcode, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0x3f)
}

func TestSetLEEventMask(t *testing.T) {
	// The LE Set Event Mask opcode is 0x01 in the LE controller group.
	opcode := OpCode(OGFLECtrl, 0x01)
	s, f := answering(opcode)

	if err := s.HCI.SetLEEventMask(0x00000000000003FF); err != nil {
		t.Fatalf("SetLEEventMask: %v", err)
	}
	wantCommand(t, f, 0, opcode, 0xff, 0x03, 0, 0, 0, 0, 0, 0)
}

func TestReadBdAddr(t *testing.T) {
	opcode := OpCode(OGFInfoParam, OCFReadBDAddr)

	t.Run("valid", func(t *testing.T) {
		// The return parameter is the address, least significant byte first.
		s, _ := answering(opcode, 0x11, 0x22, 0x33, 0x44, 0x55, 0x66)

		if err := s.HCI.ReadBdAddr(); err != nil {
			t.Fatalf("ReadBdAddr: %v", err)
		}

		want := ble.MAC{0x11, 0x22, 0x33, 0x44, 0x55, 0x66}
		if got := s.HCI.Address().MAC; got != want {
			t.Errorf("Address().MAC = %v; want %v", got, want)
		}
	})

	t.Run("response too short", func(t *testing.T) {
		s, _ := answering(opcode, 0x11, 0x22)

		if err := s.HCI.ReadBdAddr(); err != ErrInvalidPacket {
			t.Errorf("ReadBdAddr = %v; want ErrInvalidPacket", err)
		}
	})
}

func TestLESetScanParameters(t *testing.T) {
	opcode := OpCode(OGFLECtrl, OCFLESetScanParameters)
	s, f := answering(opcode)

	// active scanning, 80 ms interval, 30 ms window, public own address, no
	// filter
	if err := s.HCI.LESetScanParameters(0x01, 0x0080, 0x0030, 0x00, 0x00); err != nil {
		t.Fatalf("LESetScanParameters: %v", err)
	}
	wantCommand(t, f, 0, opcode, 0x01, 0x80, 0x00, 0x30, 0x00, 0x00, 0x00)
}

func TestLESetAdvertisingParameters(t *testing.T) {
	opcode := OpCode(OGFLECtrl, OCFLESetAdvertisingParameters)
	s, f := answering(opcode)

	err := s.HCI.LESetAdvertisingParameters(0x00a0, 0x00a0, 0x00, 0x00,
		0x00, [6]byte{}, 0x07, 0x00)
	if err != nil {
		t.Fatalf("LESetAdvertisingParameters: %v", err)
	}
	wantCommand(t, f, 0, opcode,
		0xa0, 0x00, // min interval
		0xa0, 0x00, // max interval
		0x00,             // advertising type
		0x00,             // own address type
		0x00,             // direct address type
		0, 0, 0, 0, 0, 0, // direct address
		0x07, // channel map
		0x00, // filter policy
	)
}

func TestLESetAdvertisingData(t *testing.T) {
	opcode := OpCode(OGFLECtrl, OCFLESetAdvertisingData)
	s, f := answering(opcode)

	if err := s.HCI.LESetAdvertisingData([]byte{0x02, 0x01, 0x06}); err != nil {
		t.Fatalf("LESetAdvertisingData: %v", err)
	}

	// The command always carries 32 bytes: the significant length and a
	// 31 byte padded field.
	params := make([]byte, 32)
	params[0] = 3
	copy(params[1:], []byte{0x02, 0x01, 0x06})
	wantCommand(t, f, 0, opcode, params...)
}

func TestLESetScanResponseData(t *testing.T) {
	opcode := OpCode(OGFLECtrl, OCFLESetScanResponseData)
	s, f := answering(opcode)

	if err := s.HCI.LESetScanResponseData([]byte{0x05, 0x09, 'a', 'b', 'c', 'd'}); err != nil {
		t.Fatalf("LESetScanResponseData: %v", err)
	}

	params := make([]byte, 32)
	params[0] = 6
	copy(params[1:], []byte{0x05, 0x09, 'a', 'b', 'c', 'd'})
	wantCommand(t, f, 0, opcode, params...)
}

func TestLECreateConn(t *testing.T) {
	opcode := OpCode(OGFLECtrl, OCFLECreateConn)
	s, f := answering(opcode)

	peer := [6]byte{6, 5, 4, 3, 2, 1}
	err := s.HCI.LECreateConn(0x0060, 0x0030, 0x00, 0x00, peer, 0x00,
		0x0006, 0x000c, 0x0000, 0x00c8, 0x0004, 0x0006)
	if err != nil {
		t.Fatalf("LECreateConn: %v", err)
	}

	params := []byte{
		0x60, 0x00, // scan interval
		0x30, 0x00, // scan window
		0x00,             // initiator filter
		0x00,             // peer address type
		6, 5, 4, 3, 2, 1, // peer address
		0x00,       // own address type
		0x06, 0x00, // min interval
		0x0c, 0x00, // max interval
		0x00, 0x00, // latency
		0xc8, 0x00, // supervision timeout
		0x04, 0x00, // min connection event length
		0x06, 0x00, // max connection event length
	}
	wantCommand(t, f, 0, opcode, params...)
}

func TestLECancelConn(t *testing.T) {
	opcode := OpCode(OGFLECtrl, OCFLECancelConn)
	s, f := answering(opcode)

	if err := s.HCI.LECancelConn(); err != nil {
		t.Fatalf("LECancelConn: %v", err)
	}
	wantCommand(t, f, 0, opcode)
}

func TestLEConnUpdate(t *testing.T) {
	opcode := OpCode(OGFLECtrl, OCFLEConnUpdate)
	s, f := answering(opcode)

	if err := s.HCI.LEConnUpdate(0x0040, 0x0006, 0x000c, 0x0000, 0x00c8); err != nil {
		t.Fatalf("LEConnUpdate: %v", err)
	}

	got := f.tx[0]
	if binary.LittleEndian.Uint16(got[1:]) != opcode {
		t.Errorf("opcode = %#x; want %#x", binary.LittleEndian.Uint16(got[1:]), opcode)
	}
	if handle := binary.LittleEndian.Uint16(got[4:]); handle != 0x0040 {
		t.Errorf("handle = %#x; want 0x40", handle)
	}
}

func TestDisconnect(t *testing.T) {
	opcode := OpCode(OGFLinkCtl, OCFDisconnect)
	s, f := answering(opcode)

	if err := s.HCI.Disconnect(0x0040); err != nil {
		t.Fatalf("Disconnect: %v", err)
	}
	wantCommand(t, f, 0, opcode, 0x40, 0x00, ReasonRemoteUserTerminated)
}

func TestReadLEBufferSize(t *testing.T) {
	opcode := OpCode(OGFLECtrl, OCFLEReadBufferSize)

	t.Run("a small packet length is raised to the minimum", func(t *testing.T) {
		// The specification requires at least 27 bytes.
		s, _ := answering(opcode, 0x10, 0x00, 0x04)

		if err := s.HCI.ReadLEBufferSize(); err != nil {
			t.Fatalf("ReadLEBufferSize: %v", err)
		}
		if s.ATT.maxMTU != 27 {
			t.Errorf("maxMTU = %d; want 27", s.ATT.maxMTU)
		}
		if s.HCI.maxPkt != 4 {
			t.Errorf("maxPkt = %d; want 4", s.HCI.maxPkt)
		}
	})

	t.Run("a length in range is used as it is", func(t *testing.T) {
		s, _ := answering(opcode, 0xf0, 0x00, 0x08)

		if err := s.HCI.ReadLEBufferSize(); err != nil {
			t.Fatalf("ReadLEBufferSize: %v", err)
		}
		if s.ATT.maxMTU != 240 {
			t.Errorf("maxMTU = %d; want 240", s.ATT.maxMTU)
		}
	})

	t.Run("a large packet length is clamped to the maximum MTU", func(t *testing.T) {
		// The response buffers are sized for MaximumMTU, so a controller that
		// offers more must not raise the MTU past it.
		s, _ := answering(opcode, 0x00, 0x04, 0x08)

		if err := s.HCI.ReadLEBufferSize(); err != nil {
			t.Fatalf("ReadLEBufferSize: %v", err)
		}
		if s.ATT.maxMTU != MaximumMTU {
			t.Errorf("maxMTU = %d; want %d", s.ATT.maxMTU, MaximumMTU)
		}
	})

	t.Run("response too short", func(t *testing.T) {
		s, _ := answering(opcode, 0x10)

		if err := s.HCI.ReadLEBufferSize(); err != ErrInvalidPacket {
			t.Errorf("ReadLEBufferSize = %v; want ErrInvalidPacket", err)
		}
	})
}

func TestSendVendorCommandWaitsForCompletion(t *testing.T) {
	opcode := OpCode(OGFVendor, 0x0001)
	s, f := answering(opcode)

	if err := s.HCI.SendVendorCommand(0x0001, []byte{0xaa}); err != nil {
		t.Fatalf("SendVendorCommand: %v", err)
	}
	wantCommand(t, f, 0, opcode, 0xaa)
}

func TestStop(t *testing.T) {
	s := newTestStack(&fakeTransport{})

	if err := s.HCI.Stop(); err != nil {
		t.Errorf("Stop = %v; want nil", err)
	}
}
