package bluetooth

// SetConnectHandler sets a handler function to be called whenever the adaptor connects
// or disconnects. You must call this before you call adapter.Connect() for centrals
// or advertisement.Start() for peripherals in order for it to work.
func (a *Adapter) SetConnectHandler(c func(device Device, connected bool)) {
	a.connectHandler = c
}
