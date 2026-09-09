//go:build cyw43439

package bluetooth

// ocfSetBTMACAddr is a Broadcom vendor command that sets the controller
// address. See the CYW43439 datasheet.
const ocfSetBTMACAddr = 0x0001

// SetBdAddr sets the address of the controller.
func (a *Adapter) SetBdAddr(address Address) error {
	// Reverse the byte order as per spec
	var mac [6]byte
	for i := range address.MACAddress.MAC {
		mac[i] = address.MACAddress.MAC[len(address.MACAddress.MAC)-1-i]
	}

	return a.hci.SendVendorCommandWithoutResponse(ocfSetBTMACAddr, mac[:])
}
