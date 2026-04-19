package bluetooth

// BLEAdapter is the shared interface that all platform-specific Adapter types must implement.
type BLEAdapter interface {
	Connect(address Address, params ConnectionParams) (Device, error)
	Enable() error
	Scan(callback func(*Adapter, ScanResult)) (err error)
	SetConnectHandler(c func(device Device, connected bool))
	StopScan() error
}

// SetConnectHandler sets a handler function to be called whenever the adapter connects
// or disconnects. You must call this before you call adapter.Connect() for centrals
// or advertisement.Start() for peripherals in order for it to work.
func (a *Adapter) SetConnectHandler(c func(device Device, connected bool)) {
	a.connectHandler = c
}
