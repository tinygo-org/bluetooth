package hci

// OGFVendor is the opcode group for controller specific commands. See the
// Bluetooth Core Specification, Section 5.4.1 of Part E.
const OGFVendor = 0x3f

// OpCode returns the opcode for an opcode group and an opcode command field.
func OpCode(ogf, ocf uint16) uint16 {
	return ogf<<OGFCommandPos | ocf
}

// SendVendorCommand sends a controller specific command and waits for the
// command complete event.
func (h *HCI) SendVendorCommand(ocf uint16, params []byte) error {
	return h.SendCommandWithParams(OpCode(OGFVendor, ocf), params)
}

// SendVendorCommandWithoutResponse sends a controller specific command and
// does not wait for a command complete event.
func (h *HCI) SendVendorCommandWithoutResponse(ocf uint16, params []byte) error {
	return h.SendWithoutResponse(OpCode(OGFVendor, ocf), params)
}
