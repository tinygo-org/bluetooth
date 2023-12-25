//go:build (linux && !baremetal) || darwin

// Package rawterm provides some sort of raw terminal interface, both on hosted
// systems and baremetal. It is intended only for use by examples.
//
// Newlines are always LF (not CR or CRLF). While terminals generally use a
// different format (CR when pressing the enter key and CRLF for newline) the
// format returned by Getchar and expected as input by Putchar is a single LF
// as newline symbol.
package rawterm

import (
	"os"

	"golang.org/x/crypto/ssh/terminal"
)

var terminalState *terminal.State

// Getchar returns a single character from stdin, or a serial input. Newlines
// are encoded with a single LF ('\n').
func Getchar() byte {
	var b [1]byte
	os.Stdin.Read(b[:])
	if b[0] == '\r' {
		return '\n'
	}
	return b[0]
}

// Putchar writes a single character to the terminal. Newlines are expected to
// be encoded as LF symbols ('\n').
func Putchar(ch byte) {
	if ch == '\n' {
		// Terminals expect CRLF.
		Putchar('\r')
	}
	b := [1]byte{ch}
	os.Stdout.Write(b[:])
}

// Configure initializes the terminal for use by raw reading/writing (using
// Getchar/Putchar). It must be restored after use with Restore. You can do this
// with the following code:
//
//	rawterm.Configure()
//	defer rawterm.Restore()
//	// use raw terminal features
func Configure() {
	terminalState, _ = terminal.MakeRaw(0)
}

// Restore restores the state to before a call to Configure. It must be called
// after a call to Configure to restore the terminal state, and must only be
// called after a call to Configure.
func Restore() {
	terminal.Restore(0, terminalState)
}
