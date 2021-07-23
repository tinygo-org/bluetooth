// +build wioterminal

package bluetooth

// Adapter is a dummy adapter: it represents the connection to the (only)
// SoftDevice on the chip.
type Adapter struct {
	isDefault         bool
	scanning          bool
	charWriteHandlers []charWriteHandler

	connectHandler func(device Addresser, connected bool)
}

// DefaultAdapter is the default adapter on the current system. On Nordic chips,
// it will return the SoftDevice interface.
//
// Make sure to call Enable() before using it to initialize the adapter.
var DefaultAdapter = &Adapter{isDefault: true,
	connectHandler: func(device Addresser, connected bool) {
		return
	}}

// Enable configures the BLE stack. It must be called before any
// Bluetooth-related calls (unless otherwise indicated).
func (a *Adapter) Enable() error {
	return nil
}
