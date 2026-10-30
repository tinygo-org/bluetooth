package bluetooth

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
)

func TestNotificationsInOrder(t *testing.T) {
	const charPath = dbus.ObjectPath("/org/bluez/hci0/dev_00_11_22_33_44_55/service0010/char0011")
	const count = 1000

	socket := filepath.Join(t.TempDir(), "system_bus_socket")
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	t.Setenv("DBUS_SYSTEM_BUS_ADDRESS", "unix:path="+socket)

	serverDone := make(chan error, 1)
	go func() {
		serverDone <- serveNotifications(listener, charPath, count)
	}()

	bus, err := systemBus()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { bus.Close() })
	c := &DeviceCharacteristic{
		adapter:        &Adapter{bus: bus},
		characteristic: bus.Object("org.bluez", charPath),
	}
	got := make(chan int, count)
	err = c.EnableNotifications(func(buf []byte) {
		got <- int(binary.LittleEndian.Uint16(buf))
	})
	if err != nil {
		t.Fatal(err)
	}

	for i := 0; i < count; i++ {
		select {
		case b := <-got:
			if b != i {
				t.Fatalf("notification %d has value %d", i, b)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("timeout waiting for notification %d", i)
		}
	}

	bus.Close()
	if err := <-serverDone; err != nil {
		t.Error(err)
	}
}

// serveNotifications acts as the bus and BlueZ for one client. It answers
// every method call and, after StartNotify, sends count Value changes.
func serveNotifications(listener net.Listener, charPath dbus.ObjectPath, count int) error {
	conn, err := listener.Accept()
	if err != nil {
		return err
	}
	defer conn.Close()
	r, err := acceptBusAuth(conn)
	if err != nil {
		return err
	}
	for {
		call, err := dbus.DecodeMessage(r)
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		if call.Type != dbus.TypeMethodCall || call.Flags&dbus.FlagNoReplyExpected != 0 {
			continue
		}
		var body []interface{}
		member, _ := call.Headers[dbus.FieldMember].Value().(string)
		if member == "Hello" {
			body = []interface{}{":1.1"}
		}
		if err := writeMessage(conn, dbus.TypeMethodReply, map[dbus.HeaderField]dbus.Variant{
			dbus.FieldReplySerial: dbus.MakeVariant(call.Serial()),
		}, body...); err != nil {
			return err
		}
		if member != "StartNotify" {
			continue
		}
		for i := 0; i < count; i++ {
			changed := map[string]dbus.Variant{"Value": dbus.MakeVariant(binary.LittleEndian.AppendUint16(nil, uint16(i)))}
			if err := writeMessage(conn, dbus.TypeSignal, map[dbus.HeaderField]dbus.Variant{
				dbus.FieldPath:      dbus.MakeVariant(charPath),
				dbus.FieldInterface: dbus.MakeVariant("org.freedesktop.DBus.Properties"),
				dbus.FieldMember:    dbus.MakeVariant("PropertiesChanged"),
			}, "org.bluez.GattCharacteristic1", changed, []string{}); err != nil {
				return err
			}
		}
	}
}

// acceptBusAuth runs the server side of the D-Bus EXTERNAL authentication.
// See https://dbus.freedesktop.org/doc/dbus-specification.html#auth-protocol
func acceptBusAuth(conn net.Conn) (*bufio.Reader, error) {
	r := bufio.NewReader(conn)
	if b, err := r.ReadByte(); err != nil || b != 0 {
		return nil, fmt.Errorf("auth: expected a nul byte, got %d (%v)", b, err)
	}
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return nil, err
		}
		var reply string
		switch {
		case line == "AUTH\r\n":
			reply = "REJECTED EXTERNAL\r\n"
		case strings.HasPrefix(line, "AUTH EXTERNAL"):
			reply = "OK 0123456789abcdef0123456789abcdef\r\n"
		case line == "NEGOTIATE_UNIX_FD\r\n":
			reply = "ERROR\r\n"
		case line == "BEGIN\r\n":
			return r, nil
		default:
			return nil, fmt.Errorf("auth: unexpected line %q", line)
		}
		if _, err := conn.Write([]byte(reply)); err != nil {
			return nil, err
		}
	}
}

func writeMessage(conn net.Conn, typ dbus.Type, headers map[dbus.HeaderField]dbus.Variant, body ...interface{}) error {
	if len(body) != 0 {
		headers[dbus.FieldSignature] = dbus.MakeVariant(dbus.SignatureOf(body...))
	}
	msg := &dbus.Message{Type: typ, Headers: headers, Body: body}
	return msg.EncodeTo(conn, binary.LittleEndian)
}
