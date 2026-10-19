//go:build !windows && !softdevice && (!linux || baremetal)

package bluetooth

// EnterPasskey answers PairingParams.PasskeyEntryHandler. It is not supported
// on this platform.
func (d Device) EnterPasskey(passkey string) error {
	return errNotSupported
}

// ConfirmPasskey answers PairingParams.PasskeyComparisonHandler. It is not
// supported on this platform.
func (d Device) ConfirmPasskey(match bool) error {
	return errNotSupported
}
