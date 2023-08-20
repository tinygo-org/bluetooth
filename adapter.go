package bluetooth

// Set this to true to print debug messages, for example for unknown events.
const debug = false

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

func (as *AdapterState) String() string {
	switch *as {
	case AdapterStatePoweredOff:
		return "PoweredOff"
	case AdapterStatePoweredOn:
		return "PoweredOn"
	case AdapterStateUnknown:
		return "Unknown"
	default:
		return "Unknown"
	}
}

// SetConnectHandler sets a handler function to be called whenever the adaptor connects
// or disconnects. You must call this before you call adaptor.Connect() for centrals
// or adaptor.Start() for peripherals in order for it to work.
func (a *Adapter) SetConnectHandler(c func(device Address, connected bool)) {
	a.connectHandler = c
}

// SetStateChangeHandler sets a handler function to be called whenever the adaptor's
// state changes.
// This is a no-op on bare metal.
func (a *Adapter) SetStateChangeHandler(c func(newState AdapterState)) {
	a.stateChangeHandler = c
}
