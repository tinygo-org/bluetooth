// +build wioterminal

package bluetooth

// Scan starts a BLE scan. It is stopped by a call to StopScan. A common pattern
// is to cancel the scan when a particular device has been found.
//
// The callback is run on the same goroutine as the Scan function when using a
// SoftDevice.
func (a *Adapter) Scan(callback func(*Adapter, ScanResult)) error {
	return nil
}
