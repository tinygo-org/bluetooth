//go:build !windows && (!linux || baremetal)

package bluetooth

// Pair pairs and bonds this host with the connected peripheral. It is only
// supported on Windows and Linux for now.
func (d Device) Pair(params PairingParams) error {
	return errNotSupported
}

// Unpair removes this host's bond with the device. It is only supported on
// Windows and Linux for now.
func (d Device) Unpair() error {
	return errNotSupported
}
