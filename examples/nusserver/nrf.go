// +build nrf

package main

import (
	"machine"
	"time"
)

var (
	serial = machine.UART0
	stdout = machine.UART0
)

func getchar() byte {
	for {
		// TODO: let ReadByte block instead of polling here.
		time.Sleep(1 * time.Millisecond)
		if stdout.Buffered() <= 0 {
			continue
		}
		ch, _ := stdout.ReadByte()
		if ch == 0 {
			continue
		}
		return ch
	}
}

func putchar(ch byte) {
	stdout.WriteByte(ch)
}

func initTerminal() {
}

func restoreTerminal() {
}
