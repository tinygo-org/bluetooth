package bluetooth

// Set this to true to print debug messages, for example for unknown events.
const debug = false

// SetEventHandler sets the callback that gets called on incoming events.
//
// Warning: must only be called when the Bluetooth stack has not yet been
// initialized!
func (a *Adapter) SetEventHandler(handler func(Event)) {
	a.handler = handler
}

// Event is a global Bluetooth stack event.
type Event interface{}
