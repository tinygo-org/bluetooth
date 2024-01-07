//go:build ninafw && ninafw_featherwing_init

package bluetooth

import (
	"machine"
)

func init() {
	AdapterConfig = NINAConfig{
		UART:            machine.DefaultUART,
		CS:              machine.D13,
		ACK:             machine.D11,
		GPIO0:           machine.D10,
		RESETN:          machine.D12,
		CTS:             machine.D11, // same as ACK
		RTS:             machine.D10, // same as GPIO0
		BaudRate:        115200,
		ResetInverted:   true,
		SoftFlowControl: true,
	}
}
