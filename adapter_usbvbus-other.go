//go:build !(softdevice && (s140v6 || s140v7))

package bluetooth

// USBVBusPresent reports whether USB bus power (VBUS) is present. Only the
// nRF52840 with a SoftDevice has this function, so this is not supported.
func (a *Adapter) USBVBusPresent() (bool, error) {
	return false, errNotSupported
}
