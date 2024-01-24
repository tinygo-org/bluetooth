//go:build ninafw && ninafw_machine_init

package bluetooth

import (
	"machine"
)

func init() {
	AdapterConfig = NINAConfig{
		UART:            machine.UART_NINA,
		CS:              machine.NINA_CS,
		ACK:             machine.NINA_ACK,
		GPIO0:           machine.NINA_GPIO0,
		RESETN:          machine.NINA_RESETN,
		CTS:             machine.NINA_CTS,
		RTS:             machine.NINA_RTS,
		BaudRate:        machine.NINA_BAUDRATE,
		ResetInverted:   machine.NINA_RESET_INVERTED,
		SoftFlowControl: machine.NINA_SOFT_FLOWCONTROL,
	}
}
