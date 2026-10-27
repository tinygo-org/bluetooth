package bluetooth

// disconnectLocalHost is the HCI code for a connection this side ended.
// See Bluetooth Core Specification v5.4, Vol 1, Part F, Table 1.1.
const disconnectLocalHost DisconnectReason = 0x16

// disconnectError returns the error for the reason of a disconnection event.
// It is nil when the local host ended the connection, like on macOS.
func disconnectError(reason uint8) error {
	if DisconnectReason(reason) == disconnectLocalHost {
		return nil
	}
	return DisconnectReason(reason)
}
