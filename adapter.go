package bluetooth

// AdapterState represents the state of the adaptor.
type AdapterState int

const (
	// AdapterStatePoweredOff is the state of the adaptor when it is powered off.
	AdapterStatePoweredOff = AdapterState(iota)
	// AdapterStatePoweredOn is the state of the adaptor when it is powered on.
	AdapterStatePoweredOn
	// AdapterStateUnknown is the state of the adaptor when it is unknown.
	AdapterStateUnknown
)

// BLEAdapter is the shared interface that all platform-specific Adapter types must implement.
type BLEAdapter interface {
	Connect(address Address, params ConnectionParams) (Device, error)
	Enable() error
	Reset() error
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
