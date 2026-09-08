package bluetooth

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

// DCSupplyStage selects a regulator stage. The two stages are in series,
// because REG0 supplies VDD from VDDH and VDD is the input to REG1.
// See nRF52840 Product Specification v1.11 section 5.4, Power management.
type DCSupplyStage uint8

const (
	// DCSupplyMain is the REG1 stage, which supplies the core from VDD.
	DCSupplyMain DCSupplyStage = iota

	// DCSupplyHighVoltage is the REG0 stage, which needs VDDH power. The gain
	// is small unless VDDH is much higher than VDD.
	DCSupplyHighVoltage
)
