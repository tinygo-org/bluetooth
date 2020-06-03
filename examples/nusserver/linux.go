// +build linux,!baremetal

package main

import (
	"os"

	"golang.org/x/crypto/ssh/terminal"
)

var (
	stdout        = os.Stdout
	terminalState *terminal.State
)

func getchar() byte {
	var b [1]byte
	os.Stdin.Read(b[:])
	return b[0]
}

func putchar(ch byte) {
	b := [1]byte{ch}
	os.Stdout.Write(b[:])
}

func initTerminal() {
	terminalState, _ = terminal.MakeRaw(0)
}

func restoreTerminal() {
	terminal.Restore(0, terminalState)
}
