//go:build !(softdevice && (s113v7 || s132v6 || s140v6 || s140v7))

package bluetooth

// EnableDCSupply turns the DC/DC converter of a stage on or off. Only the nRF52
// series with a SoftDevice has these regulators, so this is not supported.
func (a *Adapter) EnableDCSupply(stage DCSupplyStage, enable bool) error {
	return errNotSupported
}
