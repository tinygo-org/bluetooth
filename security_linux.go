//go:build !baremetal

package bluetooth

import (
	"errors"
	"fmt"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/godbus/dbus/v5"
)

var (
	// agentID makes each pairing agent's object path unique.
	agentID uint64

	// pairingAgents maps a device object path to the agent pairing it.
	pairingAgents sync.Map

	errDBusRejected = dbus.NewError("org.bluez.Error.Rejected", nil)
)

// Pair pairs and bonds this host with the connected peripheral, and blocks until
// pairing ends. It returns nil on success, also when the device was already paired.
func (d Device) Pair(params PairingParams) (err error) {
	defer func() {
		if params.PairingCompleteHandler != nil {
			params.PairingCompleteHandler(d, err)
		}
	}()

	agent := &pairingAgent{device: d, params: params}
	if _, loaded := pairingAgents.LoadOrStore(d.device.Path(), agent); loaded {
		return errors.New("bluetooth: a pairing with this device is already in progress")
	}
	defer pairingAgents.Delete(d.device.Path())
	defer agent.cancel()

	// BlueZ uses the agent registered by the application that calls Pair.
	// https://git.kernel.org/pub/scm/bluetooth/bluez.git/tree/doc/org.bluez.Device.rst
	path := dbus.ObjectPath(fmt.Sprintf("/org/tinygo/bluetooth/agent%d", atomic.AddUint64(&agentID, 1)))
	if err := d.adapter.bus.Export(agent, path, "org.bluez.Agent1"); err != nil {
		return fmt.Errorf("bluetooth: export pairing agent: %w", err)
	}
	defer d.adapter.bus.Export(nil, path, "org.bluez.Agent1")

	manager := d.adapter.bus.Object("org.bluez", "/org/bluez")
	err = manager.Call("org.bluez.AgentManager1.RegisterAgent", 0, path, params.IOCapabilities.bluezCapability()).Err
	if err != nil {
		return fmt.Errorf("bluetooth: register pairing agent: %w", err)
	}
	defer manager.Call("org.bluez.AgentManager1.UnregisterAgent", 0, path)

	// BlueZ 5.72 never answers Pair if bonding ends during the discovery it runs
	// on connect. See device_bonding_complete in bluez src/device.c.
	d.waitServicesResolved(servicesResolvedTimeout)

	err = d.device.Call("org.bluez.Device1.Pair", 0).Err
	if err, ok := err.(dbus.Error); ok && err.Name == "org.bluez.Error.AlreadyExists" {
		return nil
	}
	if err != nil {
		return fmt.Errorf("bluetooth: pairing failed: %w", err)
	}
	return nil
}

// Unpair removes this host's bond with the device, and returns nil if it is not
// paired. BlueZ also forgets the device, so scan for it again before Connect.
func (d Device) Unpair() error {
	err := d.adapter.adapter.Call("org.bluez.Adapter1.RemoveDevice", 0, d.device.Path()).Err
	if err, ok := err.(dbus.Error); ok && err.Name == "org.bluez.Error.DoesNotExist" {
		return nil
	}
	if err != nil {
		return fmt.Errorf("bluetooth: unpairing failed: %w", err)
	}
	return nil
}

// EnterPasskey answers PairingParams.PasskeyEntryHandler with the 6 digit
// passkey shown by the peripheral. An empty passkey rejects the pairing.
func (d Device) EnterPasskey(passkey string) error {
	agent, ok := pairingAgents.Load(d.device.Path())
	if !ok {
		return errNoPairingRequest
	}
	if passkey == "" {
		return agent.(*pairingAgent).reply(pairingAnswer{})
	}
	if !isValidPasskey(passkey) {
		return errInvalidPasskey
	}
	n, _ := strconv.ParseUint(passkey, 10, 32)
	return agent.(*pairingAgent).reply(pairingAnswer{passkey: uint32(n), accept: true})
}

// ConfirmPasskey answers PairingParams.PasskeyComparisonHandler. match reports
// whether both devices show the same passkey.
func (d Device) ConfirmPasskey(match bool) error {
	agent, ok := pairingAgents.Load(d.device.Path())
	if !ok {
		return errNoPairingRequest
	}
	return agent.(*pairingAgent).reply(pairingAnswer{accept: match})
}

// servicesResolvedTimeout bounds how long Pair waits for that discovery.
const servicesResolvedTimeout = 10 * time.Second

// waitServicesResolved waits until BlueZ has discovered the device's
// services, the device disconnects, or timeout passes.
func (d Device) waitServicesResolved(timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resolved, err := d.device.GetProperty("org.bluez.Device1.ServicesResolved")
		if err != nil {
			return
		}
		if v, ok := resolved.Value().(bool); !ok || v {
			return
		}
		if connected, err := d.Connected(); err != nil || !connected {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// bluezCapability returns the BlueZ agent capability for these IO
// capabilities.
func (io IOCapabilities) bluezCapability() string {
	switch io {
	case IOCapsDisplayOnly:
		return "DisplayOnly"
	case IOCapsDisplayYesNo:
		return "DisplayYesNo"
	case IOCapsKeyboardOnly:
		return "KeyboardOnly"
	case IOCapsKeyboardDisplay:
		return "KeyboardDisplay"
	}
	return "NoInputNoOutput"
}

// pairingAnswer is the application's answer to a passkey request.
type pairingAnswer struct {
	passkey uint32
	accept  bool
}

// pairingAgent implements org.bluez.Agent1 for one Pair call. Its methods may
// block until the application calls EnterPasskey or ConfirmPasskey.
type pairingAgent struct {
	device Device
	params PairingParams

	mu        sync.Mutex
	answer    chan pairingAnswer // set while a request waits for the application
	displayed bool
}

// Release is called when BlueZ unregisters the agent.
func (a *pairingAgent) Release() *dbus.Error {
	return nil
}

// RequestPinCode asks for a BR/EDR legacy PIN code, which LE pairing doesn't
// use.
func (a *pairingAgent) RequestPinCode(device dbus.ObjectPath) (string, *dbus.Error) {
	return "", errDBusRejected
}

// DisplayPinCode shows a BR/EDR legacy PIN code, which LE pairing doesn't use.
func (a *pairingAgent) DisplayPinCode(device dbus.ObjectPath, pincode string) *dbus.Error {
	return errDBusRejected
}

// RequestPasskey asks for the passkey shown by the peripheral.
func (a *pairingAgent) RequestPasskey(device dbus.ObjectPath) (uint32, *dbus.Error) {
	if a.params.PasskeyEntryHandler == nil {
		return 0, errDBusRejected
	}
	answer := a.wait()
	a.params.PasskeyEntryHandler(a.device)
	if ans, ok := <-answer; ok && ans.accept {
		return ans.passkey, nil
	}
	return 0, errDBusRejected
}

// DisplayPasskey shows the passkey to type on the peripheral. BlueZ calls it
// again for each typed key, but the handler runs only once.
func (a *pairingAgent) DisplayPasskey(device dbus.ObjectPath, passkey uint32, entered uint16) *dbus.Error {
	a.mu.Lock()
	first := !a.displayed
	a.displayed = true
	a.mu.Unlock()
	if first && a.params.PasskeyDisplayHandler != nil {
		a.params.PasskeyDisplayHandler(a.device, fmt.Sprintf("%06d", passkey))
	}
	return nil
}

// RequestConfirmation asks whether both devices show the same passkey.
func (a *pairingAgent) RequestConfirmation(device dbus.ObjectPath, passkey uint32) *dbus.Error {
	if a.params.PasskeyComparisonHandler == nil {
		return errDBusRejected
	}
	answer := a.wait()
	a.params.PasskeyComparisonHandler(a.device, fmt.Sprintf("%06d", passkey))
	if ans, ok := <-answer; ok && ans.accept {
		return nil
	}
	return errDBusRejected
}

// RequestAuthorization asks to accept a Just Works pairing.
func (a *pairingAgent) RequestAuthorization(device dbus.ObjectPath) *dbus.Error {
	return nil
}

// AuthorizeService asks whether the device may use a service.
func (a *pairingAgent) AuthorizeService(device dbus.ObjectPath, uuid string) *dbus.Error {
	return nil
}

// Cancel is called when a request is cancelled, for example on a timeout.
func (a *pairingAgent) Cancel() *dbus.Error {
	a.cancel()
	return nil
}

// wait returns the channel on which the application's answer arrives. It is
// buffered so that a handler may answer before it returns.
func (a *pairingAgent) wait() chan pairingAnswer {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.answer = make(chan pairingAnswer, 1)
	return a.answer
}

func (a *pairingAgent) reply(ans pairingAnswer) error {
	a.mu.Lock()
	answer := a.answer
	a.answer = nil
	a.mu.Unlock()
	if answer == nil {
		return errNoPairingRequest
	}
	answer <- ans
	return nil
}

// cancel rejects the waiting request, if any.
func (a *pairingAgent) cancel() {
	a.mu.Lock()
	answer := a.answer
	a.answer = nil
	a.mu.Unlock()
	if answer != nil {
		close(answer)
	}
}
