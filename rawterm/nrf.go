//go:build nrf

package rawterm

import (
	"machine"
	"time"
)

var serial = machine.UART0

// Getchar returns a single character from stdin, or a serial input. Newlines
// are encoded with a single LF ('\n').
func Getchar() byte {
	for {
		// TODO: let ReadByte block instead of polling here.
		time.Sleep(1 * time.Millisecond)
		if serial.Buffered() <= 0 {
			continue
		}
		ch, _ := serial.ReadByte()
		if ch == 0 {
			continue
		}
		if ch == '\r' {
			ch = '\n'
		}
		return ch
	}
}

// Putchar writes a single character to the terminal. Newlines are expected to
// be encoded as LF symbols ('\n').
func Putchar(ch byte) {
	if ch == '\n' {
		serial.WriteByte('\r')
	}
	serial.WriteByte(ch)
}

// Configure initializes the terminal for use by raw reading/writing (using
// Getchar/Putchar). It must be restored after use with Restore. You can do this
// with the following code:
//
//	rawterm.Configure()
//	defer rawterm.Restore()
//	// use raw terminal features
func Configure() {
}

// Restore restores the state to before a call to Configure. It must be called
// after a call to Configure to restore the terminal state, and must only be
// called after a call to Configure.
func Restore() {
}
