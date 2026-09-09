package hci_test

import (
	"tinygo.org/x/bluetooth/hci"
)

// nullTransport is the shape a board supplies. A real one reads from and
// writes to the controller, over a UART or a packet interface.
type nullTransport struct{}

func (nullTransport) StartRead()                 {}
func (nullTransport) EndRead()                   {}
func (nullTransport) Buffered() int              { return 0 }
func (nullTransport) Read(p []byte) (int, error) { return 0, nil }
func (nullTransport) Write(p []byte) (int, error) {
	return len(p), nil
}

// ExampleNewStack brings a controller up over a transport.
func ExampleNewStack() {
	stack := hci.NewStack(nullTransport{})

	if err := stack.HCI.Start(); err != nil {
		return
	}
	if err := stack.HCI.Reset(); err != nil {
		return
	}
	if err := stack.HCI.SetEventMask(0x3FFFFFFFFFFFFFFF); err != nil {
		return
	}
	if err := stack.HCI.SetLEEventMask(0x00000000000003FF); err != nil {
		return
	}

	// The caller polls for events, and the stack dispatches them.
	_ = stack.HCI.Poll()
}

// ExampleHCI_SetEventHandler consumes a controller specific event that this
// package does not handle.
func ExampleHCI_SetEventHandler() {
	stack := hci.NewStack(nullTransport{})

	stack.HCI.SetEventHandler(func(event uint8, params []byte) (bool, error) {
		if event != 0xff {
			return false, nil
		}

		// Handle the vendor event here.
		return true, nil
	})

	// A matching vendor command goes out the same way.
	_ = stack.HCI.SendVendorCommandWithoutResponse(0x0001, []byte{0x01})
}
