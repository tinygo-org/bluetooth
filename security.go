package bluetooth

// IOCapabilities describes this device's input and output capabilities for
// pairing, which determine the pairing methods (association models) that can
// be used.
type IOCapabilities uint8

const (
	// IOCapsNone means the device has no way to display or enter a passkey.
	// Only Just Works pairing is possible.
	IOCapsNone IOCapabilities = iota
	// IOCapsDisplayOnly means the device can show a 6-digit passkey to the
	// user, but has no input.
	IOCapsDisplayOnly
	// IOCapsDisplayYesNo means the device can show a 6-digit passkey and the
	// user can confirm or reject it (enables LE Secure Connections numeric
	// comparison).
	IOCapsDisplayYesNo
	// IOCapsKeyboardOnly means the user can enter a passkey, but the device
	// has no display.
	IOCapsKeyboardOnly
	// IOCapsKeyboardDisplay means the device has both a display and a way to
	// enter a passkey.
	IOCapsKeyboardDisplay
)

// PairingError reports a failed pairing procedure. The value is the
// stack-specific status code (the SMP failure reason / BLE_GAP_SEC_STATUS
// value on Nordic SoftDevices).
type PairingError uint8

func (e PairingError) Error() string {
	const hexDigits = "0123456789abcdef"
	return "bluetooth: pairing failed (status 0x" +
		string(hexDigits[e>>4]) + string(hexDigits[e&0xf]) + ")"
}

// PairingParams configures how the adapter responds to pairing initiated by a
// connected central.
//
// The handler callbacks are invoked outside interrupt context (on a dedicated
// goroutine on SoftDevices), one at a time. A handler that blocks (for
// example, waiting for user input) delays the delivery of subsequent pairing
// events.
type PairingParams struct {
	IOCapabilities IOCapabilities

	// LESC enables LE Secure Connections. Pairing falls back to legacy
	// pairing when the central doesn't support it.
	LESC bool

	// MITM requests man-in-the-middle protection. This requires a passkey or
	// numeric comparison, so both sides need suitable IO capabilities.
	MITM bool

	// StaticPasskey optionally fixes the displayed passkey to the given
	// 6-digit ASCII string instead of a randomly generated one. It is only
	// used with display IO capabilities.
	StaticPasskey string

	// PasskeyDisplayHandler is called when a passkey must be shown to the
	// user, who then enters it on the central.
	PasskeyDisplayHandler func(device Device, passkey string)

	// PasskeyComparisonHandler is called for LE Secure Connections numeric
	// comparison, when the same passkey is shown on both devices. The
	// application must call device.ConfirmPasskey to accept or reject the
	// pairing. If nil, numeric comparison requests are rejected.
	PasskeyComparisonHandler func(device Device, passkey string)

	// PasskeyEntryHandler is called when the user must enter the passkey
	// shown by the central. The application must call device.EnterPasskey
	// with the entered passkey. If nil, passkey entry requests are rejected.
	PasskeyEntryHandler func(device Device)

	// PairingCompleteHandler is called when the pairing procedure finishes:
	// err is nil on success or a PairingError on failure.
	PairingCompleteHandler func(device Device, err error)
}
