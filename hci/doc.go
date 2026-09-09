// Package hci implements the Bluetooth Low Energy host side of the Host
// Controller Interface, together with the L2CAP and ATT layers above it.
//
// A Stack drives a controller over a Transport. The transport carries HCI
// packets and is the only hardware dependent part, so a board provides one and
// this package stays portable.
//
// Poll reads from the transport and dispatches one packet. Every call that
// waits for a response from the controller polls until it arrives, so the
// caller must not poll from more than one goroutine.
//
// SetEventHandler and SetLEEventHandler receive the events that this package
// does not handle. A controller with vendor specific events uses them, and
// SendVendorCommand sends the matching commands.
//
// This package is not yet stable. Its API can change until the module reaches
// version 1.0.
package hci
