//go:build !baremetal

package bluetooth

// L2CAPConn represents an L2CAP Connection-Oriented Channel connection.
type L2CAPConn struct{}

var _ L2CAPChannel = (*L2CAPConn)(nil)

// Read is not yet implemented on this platform.
func (c *L2CAPConn) Read(p []byte) (int, error) {
	return 0, errNotYetImplmented
}

// Write is not yet implemented on this platform.
func (c *L2CAPConn) Write(p []byte) (int, error) {
	return 0, errNotYetImplmented
}

// Close is not yet implemented on this platform.
func (c *L2CAPConn) Close() error {
	return errNotYetImplmented
}

// PSM is not yet implemented on this platform.
func (c *L2CAPConn) PSM() L2CAPPSM {
	return 0
}

// OpenL2CAPChannel is not yet implemented on this platform.
func (d Device) OpenL2CAPChannel(psm L2CAPPSM) (*L2CAPConn, error) {
	return nil, errNotYetImplmented
}
