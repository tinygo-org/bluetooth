package hci

import (
	"bytes"
	"encoding/binary"
	"testing"
)

// parseEvent hands one event packet straight to the event parser, so that a
// malformed one can be checked. Poll deliberately swallows a parse error and
// resets, which TestPollSwallowsAParseError covers.
func parseEvent(packet []byte) (*Stack, error) {
	s := newTestStack(&fakeTransport{})

	return s, s.HCI.handleEventData(packet[1:])
}

// pollOnce feeds the scripted packets to the stack and polls once.
func pollOnce(t *testing.T, packets ...[]byte) (*Stack, *fakeTransport, error) {
	t.Helper()

	f := &fakeTransport{rx: packets}
	s := newTestStack(f)

	return s, f, s.HCI.Poll()
}

func TestPollBracketsTheRead(t *testing.T) {
	_, f, err := pollOnce(t, commandComplete(OpCode(OGFHostCtl, OCFReset), 0))
	if err != nil {
		t.Fatalf("Poll: %v", err)
	}
	if f.starts != 1 || f.ends != 1 {
		t.Errorf("StartRead/EndRead called %d/%d times; want 1/1", f.starts, f.ends)
	}
}

func TestCommandComplete(t *testing.T) {
	opcode := OpCode(OGFInfoParam, OCFReadBDAddr)
	addr := []byte{1, 2, 3, 4, 5, 6}

	s, _, err := pollOnce(t, commandComplete(opcode, 0, addr...))
	if err != nil {
		t.Fatalf("Poll: %v", err)
	}
	if s.HCI.CommandStatus() != 0 {
		t.Errorf("CommandStatus = %d; want 0", s.HCI.CommandStatus())
	}
	// The response starts at the number of packets byte and covers the whole
	// parameter list.
	if got := s.HCI.CommandResponse(); len(got) == 0 {
		t.Error("CommandResponse is empty; want the event parameters")
	}
}

func TestCommandCompleteNoParameters(t *testing.T) {
	// An event with a zero parameter length must not index past the buffer.
	s, _, err := pollOnce(t, []byte{PacketEvent, EventCmdComplete, 0, 0, 0, 0})
	if err != nil {
		t.Fatalf("Poll: %v", err)
	}
	if len(s.HCI.CommandResponse()) != 0 {
		t.Errorf("CommandResponse = %v; want empty", s.HCI.CommandResponse())
	}
}

func TestTruncatedEvents(t *testing.T) {
	for _, tc := range []struct {
		name   string
		packet []byte
	}{
		{"event with no length byte", []byte{PacketEvent, EventCmdComplete}},
		{"command complete too short", []byte{PacketEvent, EventCmdComplete, 4, 1, 0x03}},
		{"command complete length past end", []byte{PacketEvent, EventCmdComplete, 200, 1, 0x03, 0x0c, 0}},
		{"command status too short", []byte{PacketEvent, EventCmdStatus, 4, 0, 1}},
		{"disconnect complete too short", []byte{PacketEvent, EventDisconnComplete, 4, 0, 1}},
		{"num completed packets too short", []byte{PacketEvent, EventNumCompPkts, 1, 1}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := parseEvent(tc.packet); err != ErrInvalidPacket {
				t.Errorf("handleEventData = %v; want ErrInvalidPacket", err)
			}
		})
	}
}

func TestNumCompletedPackets(t *testing.T) {
	t.Run("one handle", func(t *testing.T) {
		f := &fakeTransport{}
		s := newTestStack(f)
		s.HCI.pendingPkt = 5

		// numHandles, handle, count
		f.rx = [][]byte{event(EventNumCompPkts, 1, 0x40, 0x00, 0x02, 0x00)}
		if err := s.HCI.Poll(); err != nil {
			t.Fatalf("Poll: %v", err)
		}
		if s.HCI.pendingPkt != 3 {
			t.Errorf("pendingPkt = %d; want 3", s.HCI.pendingPkt)
		}
	})

	t.Run("three handles", func(t *testing.T) {
		f := &fakeTransport{}
		s := newTestStack(f)
		s.HCI.pendingPkt = 10

		f.rx = [][]byte{event(EventNumCompPkts, 3,
			0x40, 0x00, 0x01, 0x00,
			0x41, 0x00, 0x02, 0x00,
			0x42, 0x00, 0x03, 0x00)}
		if err := s.HCI.Poll(); err != nil {
			t.Fatalf("Poll: %v", err)
		}
		if s.HCI.pendingPkt != 4 {
			t.Errorf("pendingPkt = %d; want 4", s.HCI.pendingPkt)
		}
	})

	t.Run("count larger than pending does not wrap", func(t *testing.T) {
		f := &fakeTransport{}
		s := newTestStack(f)
		s.HCI.pendingPkt = 1

		f.rx = [][]byte{event(EventNumCompPkts, 1, 0x40, 0x00, 0x09, 0x00)}
		if err := s.HCI.Poll(); err != nil {
			t.Fatalf("Poll: %v", err)
		}
		if s.HCI.pendingPkt != 0 {
			t.Errorf("pendingPkt = %d; want 0", s.HCI.pendingPkt)
		}
	})

	t.Run("handle count overruns the event", func(t *testing.T) {
		// The count comes off the wire and claims more entries than are here.
		_, err := parseEvent(event(EventNumCompPkts, 4, 0x40, 0x00, 0x01, 0x00))
		if err != ErrInvalidPacket {
			t.Errorf("handleEventData = %v; want ErrInvalidPacket", err)
		}
	})
}

func TestAdvertisingReport(t *testing.T) {
	// numReports, type, addressType, address, dataLen, data..., rssi
	report := func(dataLen byte, data []byte, extra ...byte) []byte {
		params := []byte{1, 0x00, 0x01, 6, 5, 4, 3, 2, 1, dataLen}
		params = append(params, data...)
		params = append(params, extra...)

		return leEvent(LEMetaAdvertisingReport, params...)
	}

	t.Run("valid", func(t *testing.T) {
		s, _, err := pollOnce(t, report(3, []byte{0x02, 0x01, 0x06}, 0xc5))
		if err != nil {
			t.Fatalf("Poll: %v", err)
		}
		if !s.HCI.HasAdvertisement() {
			t.Fatal("HasAdvertisement = false; want true")
		}

		rep, ok := s.HCI.Advertisement()
		if !ok {
			t.Fatal("Advertisement reported not ok")
		}
		if rep.DataLen != 3 {
			t.Errorf("DataLen = %d; want 3", rep.DataLen)
		}
		if !bytes.Equal(rep.Data[:3], []byte{0x02, 0x01, 0x06}) {
			t.Errorf("Data = %v; want 02 01 06", rep.Data[:3])
		}
		if rep.Address != [6]byte{6, 5, 4, 3, 2, 1} {
			t.Errorf("Address = %v; want 06 05 04 03 02 01", rep.Address)
		}
		if rep.RSSI != -59 {
			t.Errorf("RSSI = %d; want -59", rep.RSSI)
		}
		if rep.AddressType != 1 {
			t.Errorf("AddressType = %d; want 1", rep.AddressType)
		}

		s.HCI.ClearAdvertisement()
		if s.HCI.HasAdvertisement() {
			t.Error("HasAdvertisement after ClearAdvertisement = true; want false")
		}
	})

	t.Run("data length 31 is allowed", func(t *testing.T) {
		s, _, err := pollOnce(t, report(31, make([]byte, 31), 0))
		if err != nil {
			t.Fatalf("Poll: %v", err)
		}
		if !s.HCI.HasAdvertisement() {
			t.Error("a 31 byte report was rejected; want accepted")
		}
	})

	t.Run("data length 32 is rejected", func(t *testing.T) {
		s, err := parseEvent(report(32, make([]byte, 32), 0))
		if err != ErrInvalidPacket {
			t.Errorf("handleEventData = %v; want ErrInvalidPacket", err)
		}
		if s.HCI.HasAdvertisement() {
			t.Error("a rejected report was reported; want not reported")
		}
	})

	t.Run("data length past the end of the event", func(t *testing.T) {
		// Claims 20 bytes of data but carries 3. Reading it would run off the
		// end of the packet.
		s, err := parseEvent(report(20, []byte{1, 2, 3}, 0xc5))
		if err != ErrInvalidPacket {
			t.Errorf("handleEventData = %v; want ErrInvalidPacket", err)
		}
		if s.HCI.HasAdvertisement() {
			t.Error("a rejected report was reported; want not reported")
		}
	})

	t.Run("report too short for the fixed part", func(t *testing.T) {
		_, err := parseEvent(leEvent(LEMetaAdvertisingReport, 1, 0x00, 0x01, 6, 5))
		if err != ErrInvalidPacket {
			t.Errorf("handleEventData = %v; want ErrInvalidPacket", err)
		}
	})

	t.Run("more than one report leaves the RSSI alone", func(t *testing.T) {
		// Multiple reports in one event are not handled yet, so the RSSI must
		// not be taken from the wrong offset.
		params := []byte{2, 0x00, 0x01, 6, 5, 4, 3, 2, 1, 1, 0x00, 0xc5}
		s, _, err := pollOnce(t, leEvent(LEMetaAdvertisingReport, params...))
		if err != nil {
			t.Fatalf("Poll: %v", err)
		}

		rep, _ := s.HCI.Advertisement()
		if rep.RSSI != 0 {
			t.Errorf("RSSI = %d; want 0 for a multi report event", rep.RSSI)
		}
	})
}

func TestConnectionComplete(t *testing.T) {
	// status, handle, role, addressType, address, interval, latency, timeout,
	// accuracy
	base := []byte{0x00, 0x40, 0x00, 0x01, 0x01, 6, 5, 4, 3, 2, 1,
		0x06, 0x00, 0x00, 0x00, 0xc8, 0x00, 0x00}

	t.Run("connection complete", func(t *testing.T) {
		s, _, err := pollOnce(t, leEvent(LEMetaConnComplete, base...))
		if err != nil {
			t.Fatalf("Poll: %v", err)
		}
		if !s.HCI.HasConnection() {
			t.Fatal("HasConnection = false; want true")
		}

		conn, _ := s.HCI.Connection()
		if conn.Handle != 0x0040 {
			t.Errorf("Handle = %#x; want 0x40", conn.Handle)
		}
		if conn.Role != 1 {
			t.Errorf("Role = %d; want 1", conn.Role)
		}
		if conn.Interval != 0x0006 {
			t.Errorf("Interval = %#x; want 0x6", conn.Interval)
		}
		if conn.Timeout != 0x00c8 {
			t.Errorf("Timeout = %#x; want 0xc8", conn.Timeout)
		}
		if conn.Address != [6]byte{6, 5, 4, 3, 2, 1} {
			t.Errorf("Address = %v; want 06 05 04 03 02 01", conn.Address)
		}
	})

	t.Run("enhanced connection complete reads the later offsets", func(t *testing.T) {
		// The enhanced variant carries the local and peer resolvable private
		// addresses before the interval, so the interval and the timeout sit
		// 12 bytes further along.
		params := []byte{0x00, 0x41, 0x00, 0x00, 0x01, 6, 5, 4, 3, 2, 1}
		params = append(params, make([]byte, 12)...) // the two extra addresses
		params = append(params, 0x10, 0x00, 0x00, 0x00, 0xd0, 0x07, 0x00, 0x00)

		s, _, err := pollOnce(t, leEvent(LEMetaEnhancedConnectionComplete, params...))
		if err != nil {
			t.Fatalf("Poll: %v", err)
		}

		conn, ok := s.HCI.Connection()
		if !ok {
			t.Fatal("no connection reported")
		}
		if conn.Handle != 0x0041 {
			t.Errorf("Handle = %#x; want 0x41", conn.Handle)
		}
		if conn.Interval != 0x0010 {
			t.Errorf("Interval = %#x; want 0x10", conn.Interval)
		}
		if conn.Timeout != 0x07d0 {
			t.Errorf("Timeout = %#x; want 0x7d0", conn.Timeout)
		}
	})

	t.Run("connection complete too short", func(t *testing.T) {
		_, err := parseEvent(leEvent(LEMetaConnComplete, base[:8]...))
		if err != ErrInvalidPacket {
			t.Errorf("handleEventData = %v; want ErrInvalidPacket", err)
		}
	})

	t.Run("enhanced connection complete too short", func(t *testing.T) {
		// Long enough for the plain variant, too short for the enhanced one.
		_, err := parseEvent(leEvent(LEMetaEnhancedConnectionComplete, base...))
		if err != ErrInvalidPacket {
			t.Errorf("handleEventData = %v; want ErrInvalidPacket", err)
		}
	})
}

func TestDisconnectionComplete(t *testing.T) {
	s, _, err := pollOnce(t, event(EventDisconnComplete, 0x00, 0x40, 0x00, ReasonRemoteUserTerminated))
	if err != nil {
		t.Fatalf("Poll: %v", err)
	}
	if !s.HCI.HasDisconnection() {
		t.Fatal("HasDisconnection = false; want true")
	}

	disc, _ := s.HCI.Disconnection()
	if disc.Handle != 0x0040 {
		t.Errorf("Handle = %#x; want 0x40", disc.Handle)
	}
	if disc.Reason != ReasonRemoteUserTerminated {
		t.Errorf("Reason = %#x; want %#x", disc.Reason, ReasonRemoteUserTerminated)
	}

	s.HCI.ClearConnection()
	if s.HCI.HasDisconnection() {
		t.Error("HasDisconnection after ClearConnection = true; want false")
	}
}

func TestUnknownEventGoesToTheHandler(t *testing.T) {
	const vendorEvent = 0xff

	t.Run("consumed", func(t *testing.T) {
		f := &fakeTransport{rx: [][]byte{event(vendorEvent, 1, 2, 3)}}
		s := newTestStack(f)

		var gotEvent uint8
		var gotParams []byte
		s.HCI.SetEventHandler(func(e uint8, params []byte) (bool, error) {
			gotEvent = e
			gotParams = append([]byte{}, params...)
			return true, nil
		})

		if err := s.HCI.Poll(); err != nil {
			t.Fatalf("Poll: %v", err)
		}
		if gotEvent != vendorEvent {
			t.Errorf("event = %#x; want %#x", gotEvent, vendorEvent)
		}
		if !bytes.Equal(gotParams, []byte{1, 2, 3}) {
			t.Errorf("params = %v; want 1 2 3", gotParams)
		}
	})

	t.Run("declined", func(t *testing.T) {
		f := &fakeTransport{rx: [][]byte{event(vendorEvent, 1)}}
		s := newTestStack(f)
		s.HCI.SetEventHandler(func(uint8, []byte) (bool, error) { return false, nil })

		// An event the handler declines is not an error, because an unknown
		// event was already ignored before the hook existed.
		if err := s.HCI.Poll(); err != nil {
			t.Errorf("Poll = %v; want nil", err)
		}
	})

	t.Run("no handler", func(t *testing.T) {
		if _, _, err := pollOnce(t, event(vendorEvent, 1)); err != nil {
			t.Errorf("Poll = %v; want nil", err)
		}
	})
}

func TestUnknownLEEventGoesToTheHandler(t *testing.T) {
	const vendorSubevent = 0x7f

	t.Run("consumed", func(t *testing.T) {
		f := &fakeTransport{rx: [][]byte{leEvent(vendorSubevent, 9, 8)}}
		s := newTestStack(f)

		var got uint8
		s.HCI.SetLEEventHandler(func(sub uint8, params []byte) (bool, error) {
			got = sub
			return true, nil
		})

		if err := s.HCI.Poll(); err != nil {
			t.Fatalf("Poll: %v", err)
		}
		if got != vendorSubevent {
			t.Errorf("subevent = %#x; want %#x", got, vendorSubevent)
		}
	})

	t.Run("declined", func(t *testing.T) {
		s := newTestStack(&fakeTransport{})
		s.HCI.SetLEEventHandler(func(uint8, []byte) (bool, error) { return false, nil })

		pkt := leEvent(vendorSubevent, 9)
		if err := s.HCI.handleEventData(pkt[1:]); err != ErrUnknownEvent {
			t.Errorf("handleEventData = %v; want ErrUnknownEvent", err)
		}
	})

	t.Run("no handler", func(t *testing.T) {
		if _, err := parseEvent(leEvent(vendorSubevent, 9)); err != ErrUnknownEvent {
			t.Errorf("handleEventData = %v; want ErrUnknownEvent", err)
		}
	})
}

func TestHardwareError(t *testing.T) {
	if _, err := parseEvent(event(EventHardwareError, 0x01)); err != ErrUnknownEvent {
		t.Errorf("handleEventData = %v; want ErrUnknownEvent", err)
	}
}

func TestUnknownPacketType(t *testing.T) {
	s := newTestStack(&fakeTransport{})
	copy(s.HCI.buf, []byte{0x09, 1, 2, 3, 4})
	s.HCI.end = 5
	s.HCI.pos = 5

	if _, err := s.HCI.processPacket(); err != ErrUnknown {
		t.Errorf("processPacket = %v; want ErrUnknown", err)
	}
}

func TestSynchronousDataIsSkipped(t *testing.T) {
	// BLE has no synchronous data, so such a packet is stepped over and the
	// next one is handled.
	sco := []byte{PacketSynchronousData, 0x40, 0x00, 2, 0xaa, 0xbb}
	s, _, err := pollOnce(t, sco, commandComplete(OpCode(OGFHostCtl, OCFReset), 0))
	if err != nil {
		t.Fatalf("Poll: %v", err)
	}
	_ = s
}

func TestACLPacketLongerThanTheBufferCanHold(t *testing.T) {
	// The ACL length comes off the wire. A length that no buffer could ever
	// hold has to be rejected rather than waited on.
	pkt := []byte{PacketACLData, 0x40, 0x00, 0xff, 0xff, 0, 0, 0, 0}
	s := newTestStack(&fakeTransport{})
	copy(s.HCI.buf, pkt)
	s.HCI.end = len(pkt)
	s.HCI.pos = len(pkt)

	if _, err := s.HCI.processPacket(); err != ErrInvalidPacket {
		t.Errorf("processPacket = %v; want ErrInvalidPacket", err)
	}
}

func TestResetWritesTheCommandAndWaitsForCompletion(t *testing.T) {
	opcode := OpCode(OGFHostCtl, OCFReset)
	f := &fakeTransport{rx: [][]byte{commandComplete(opcode, 0)}}
	s := newTestStack(f)

	if err := s.HCI.Reset(); err != nil {
		t.Fatalf("Reset: %v", err)
	}
	if len(f.tx) != 1 {
		t.Fatalf("wrote %d packets; want 1", len(f.tx))
	}

	// Command packet, opcode little endian, zero parameters.
	want := []byte{PacketCommand, 0x03, 0x0c, 0x00}
	if !bytes.Equal(f.tx[0], want) {
		t.Errorf("wrote %#v; want %#v", f.tx[0], want)
	}
}

func TestCommandWithoutAnAnswerTimesOut(t *testing.T) {
	f := &fakeTransport{}
	s := newTestStack(f)

	if err := s.HCI.Reset(); err != ErrTimeout {
		t.Errorf("Reset = %v; want ErrTimeout", err)
	}
}

func TestSendCommandPropagatesAWriteError(t *testing.T) {
	f := &fakeTransport{writeErr: errFakeRead}
	s := newTestStack(f)

	if err := s.HCI.Reset(); err != errFakeRead {
		t.Errorf("Reset = %v; want the write error", err)
	}
}

func TestSetRandomAddress(t *testing.T) {
	opcode := OpCode(OGFLECtrl, OCFLESetRandomAddress)
	f := &fakeTransport{rx: [][]byte{commandComplete(opcode, 0)}}
	s := newTestStack(f)

	mac := [6]byte{0xc0, 0x01, 0x02, 0x03, 0x04, 0x05}
	if err := s.HCI.SetRandomAddress(mac); err != nil {
		t.Fatalf("SetRandomAddress: %v", err)
	}

	want := append([]byte{PacketCommand, 0x05, 0x20, 0x06}, mac[:]...)
	if !bytes.Equal(f.tx[0], want) {
		t.Errorf("wrote %#v; want %#v", f.tx[0], want)
	}
	if !s.HCI.Address().IsRandom() {
		t.Error("Address().IsRandom() = false; want true")
	}
}

func TestScanEnableAndAdvertiseEnableBytes(t *testing.T) {
	t.Run("scan enable with duplicate filtering", func(t *testing.T) {
		opcode := OpCode(OGFLECtrl, OCFLESetScanEnable)
		f := &fakeTransport{rx: [][]byte{commandComplete(opcode, 0)}}
		s := newTestStack(f)

		if err := s.HCI.LESetScanEnable(true, true); err != nil {
			t.Fatalf("LESetScanEnable: %v", err)
		}

		want := []byte{PacketCommand, 0x0c, 0x20, 0x02, 0x01, 0x01}
		if !bytes.Equal(f.tx[0], want) {
			t.Errorf("wrote %#v; want %#v", f.tx[0], want)
		}
	})

	t.Run("advertise enable does not wait for a response", func(t *testing.T) {
		f := &fakeTransport{}
		s := newTestStack(f)

		// This command is sent without waiting, so it succeeds with nothing
		// scripted to answer it.
		if err := s.HCI.LESetAdvertiseEnable(true); err != nil {
			t.Fatalf("LESetAdvertiseEnable: %v", err)
		}

		want := []byte{PacketCommand, 0x0a, 0x20, 0x01, 0x01}
		if !bytes.Equal(f.tx[0], want) {
			t.Errorf("wrote %#v; want %#v", f.tx[0], want)
		}
	})
}

func TestStartDrainsTheTransport(t *testing.T) {
	f := &fakeTransport{rx: [][]byte{{1, 2, 3, 4}, {5, 6, 7, 8}}}
	s := newTestStack(f)

	if err := s.HCI.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if f.Buffered() != 0 {
		t.Errorf("Buffered after Start = %d; want 0", f.Buffered())
	}
}

func TestStartRejectsAnOversizedLeftover(t *testing.T) {
	// Start discards whatever is left over into the packet buffer, so an entry
	// larger than that buffer cannot be drained.
	f := &fakeTransport{rx: [][]byte{make([]byte, ReadBufferSize+8)}}
	s := newTestStack(f)

	if err := s.HCI.Start(); err != ErrInvalidPacket {
		t.Errorf("Start = %v; want ErrInvalidPacket", err)
	}
}

func TestTwoEventsInOneRead(t *testing.T) {
	// A stream transport can hand back two packets at once. The second must be
	// kept and handled on the next poll.
	first := event(EventDisconnComplete, 0x00, 0x40, 0x00, 0x13)
	second := commandComplete(OpCode(OGFHostCtl, OCFReset), 0)

	f := &fakeTransport{rx: [][]byte{append(append([]byte{}, first...), second...)}}
	s := newTestStack(f)

	if err := s.HCI.Poll(); err != nil {
		t.Fatalf("first Poll: %v", err)
	}
	if !s.HCI.HasDisconnection() {
		t.Error("first poll did not handle the disconnection")
	}

	if err := s.HCI.Poll(); err != nil {
		t.Fatalf("second Poll: %v", err)
	}
	if s.HCI.cmdCompleteOpcode != OpCode(OGFHostCtl, OCFReset) {
		t.Errorf("second poll did not handle the command complete")
	}
}

func TestPacketSplitAcrossTwoReads(t *testing.T) {
	// A stream transport can hand back half a packet. The stack keeps it and
	// finishes it on a later poll.
	full := commandComplete(OpCode(OGFHostCtl, OCFReset), 0)
	f := &fakeTransport{rx: [][]byte{full[:3], full[3:]}}
	s := newTestStack(f)

	if err := s.HCI.Poll(); err != nil {
		t.Fatalf("first Poll: %v", err)
	}
	if err := s.HCI.Poll(); err != nil {
		t.Fatalf("second Poll: %v", err)
	}
	if s.HCI.cmdCompleteOpcode != OpCode(OGFHostCtl, OCFReset) {
		t.Error("the split packet was not handled")
	}
}

func TestPacketTransportReadsWholePackets(t *testing.T) {
	f := &fakeTransport{packet: true, rx: [][]byte{
		commandComplete(OpCode(OGFHostCtl, OCFReset), 0),
	}}
	s := newTestStack(f)

	if err := s.HCI.Poll(); err != nil {
		t.Fatalf("Poll: %v", err)
	}
	if s.HCI.cmdCompleteOpcode != OpCode(OGFHostCtl, OCFReset) {
		t.Error("the packet was not handled")
	}
}

func TestOpCode(t *testing.T) {
	for _, tc := range []struct {
		ogf, ocf, want uint16
	}{
		{OGFHostCtl, OCFReset, 0x0c03},
		{OGFLECtrl, OCFLESetScanEnable, 0x200c},
		{OGFInfoParam, OCFReadBDAddr, 0x1009},
		{OGFVendor, 0x0001, 0xfc01},
	} {
		if got := OpCode(tc.ogf, tc.ocf); got != tc.want {
			t.Errorf("OpCode(%#x, %#x) = %#x; want %#x", tc.ogf, tc.ocf, got, tc.want)
		}
	}
}

func TestSendVendorCommand(t *testing.T) {
	f := &fakeTransport{}
	s := newTestStack(f)

	if err := s.HCI.SendVendorCommandWithoutResponse(0x0001, []byte{1, 2, 3}); err != nil {
		t.Fatalf("SendVendorCommandWithoutResponse: %v", err)
	}

	want := []byte{PacketCommand, 0x01, 0xfc, 0x03, 1, 2, 3}
	if !bytes.Equal(f.tx[0], want) {
		t.Errorf("wrote %#v; want %#v", f.tx[0], want)
	}
}

func TestPollSwallowsAParseError(t *testing.T) {
	// A malformed packet must not stop the caller's poll loop. Poll drops what
	// it has and returns nil, so that scanning survives a bad advertisement.
	bad := leEvent(LEMetaAdvertisingReport, 1, 0x00, 0x01, 6, 5, 4, 3, 2, 1, 32)
	good := commandComplete(OpCode(OGFHostCtl, OCFReset), 0)

	f := &fakeTransport{rx: [][]byte{bad, good}}
	s := newTestStack(f)

	if err := s.HCI.Poll(); err != nil {
		t.Errorf("Poll on a malformed packet = %v; want nil", err)
	}
	if s.HCI.HasAdvertisement() {
		t.Error("a malformed report was reported; want not reported")
	}

	// The stack recovers and handles the next packet.
	if err := s.HCI.Poll(); err != nil {
		t.Fatalf("Poll after recovery: %v", err)
	}
	if s.HCI.cmdCompleteOpcode != OpCode(OGFHostCtl, OCFReset) {
		t.Error("the packet after a malformed one was not handled")
	}
}

func TestPollPropagatesATransportError(t *testing.T) {
	f := &fakeTransport{rx: [][]byte{{1, 2, 3, 4}}, readErr: errFakeRead}
	s := newTestStack(f)

	if err := s.HCI.Poll(); err != errFakeRead {
		t.Errorf("Poll = %v; want the read error", err)
	}
}

func TestPollRejectsAPacketThatCanNeverFit(t *testing.T) {
	// A packet transport that offers more than the read buffer can hold, with
	// nothing already buffered to drain, can never be satisfied.
	f := &fakeTransport{packet: true, rx: [][]byte{make([]byte, ReadBufferSize+4)}}
	s := newTestStack(f)

	if err := s.HCI.Poll(); err != ErrInvalidPacket {
		t.Errorf("Poll = %v; want ErrInvalidPacket", err)
	}
}

func TestNoEventPanicsOnAShortPacket(t *testing.T) {
	// Every event code and every LE meta subevent, at every length up to 40
	// bytes. None of them may panic, whatever the content.
	events := []uint8{
		EventDisconnComplete, EventEncryptionChange, EventCmdComplete,
		EventCmdStatus, EventHardwareError, EventNumCompPkts,
		EventReturnLinkKeys, EventLEMeta, 0x00, 0xff,
	}
	subevents := []uint8{
		LEMetaConnComplete, LEMetaAdvertisingReport,
		LEMetaConnectionUpdateComplete, LEMetaReadRemoteUsedFeaturesComplete,
		LEMetaLongTermKeyRequest, LEMetaRemoteConnParamReq,
		LEMetaDataLengthChange, LEMetaReadLocalP256Complete,
		LEMetaGenerateDHKeyComplete, LEMetaEnhancedConnectionComplete,
		LEMetaDirectAdvertisingReport, 0x00, 0xff,
	}

	fill := func(buf []byte, from int) {
		for i := from; i < len(buf); i++ {
			buf[i] = 0xff
		}
	}

	for _, evt := range events {
		for n := 0; n <= 40; n++ {
			buf := make([]byte, n)
			if n > 0 {
				buf[0] = evt
			}
			fill(buf, 1)

			s := newTestStack(&fakeTransport{})
			_ = s.HCI.handleEventData(buf)
		}
	}

	for _, sub := range subevents {
		for n := 2; n <= 40; n++ {
			buf := make([]byte, n)
			buf[0] = EventLEMeta
			buf[1] = byte(n - 2)
			if n > 2 {
				buf[2] = sub
			}
			fill(buf, 3)

			s := newTestStack(&fakeTransport{})
			_ = s.HCI.handleEventData(buf)
		}
	}
}

func TestNoACLPacketPanicsOnAShortPayload(t *testing.T) {
	for _, cid := range []uint16{CIDATT, CIDSignaling, CIDSecurity, 0x0099} {
		for n := 0; n <= 24; n++ {
			buf := make([]byte, n)
			if n >= 8 {
				binary.LittleEndian.PutUint16(buf[0:], testConnHandle)
				binary.LittleEndian.PutUint16(buf[2:], uint16(n-4+4))
				binary.LittleEndian.PutUint16(buf[4:], uint16(n-8))
				binary.LittleEndian.PutUint16(buf[6:], cid)
				for i := 8; i < n; i++ {
					buf[i] = 0xff
				}
			}

			s := newTestStack(&fakeTransport{})
			_ = s.ATT.addConnection(testConnHandle)
			_ = s.HCI.handleACLData(buf)
		}
	}
}

func TestNoPacketPanicsThroughPoll(t *testing.T) {
	// Whole packets straight off a transport, at every length and with each
	// packet type byte.
	for _, typ := range []uint8{PacketCommand, PacketACLData,
		PacketSynchronousData, PacketEvent, PacketSecurity, 0x00, 0xff} {
		for n := 1; n <= 32; n++ {
			pkt := make([]byte, n)
			pkt[0] = typ
			for i := 1; i < n; i++ {
				pkt[i] = 0xff
			}

			s := newTestStack(&fakeTransport{rx: [][]byte{pkt}})
			_ = s.HCI.Poll()
		}
	}
}
