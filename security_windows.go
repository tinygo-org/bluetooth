package bluetooth

import (
	"errors"
	"fmt"
	"sync"
	"unsafe"

	"github.com/go-ole/go-ole"
	"github.com/saltosystems/winrt-go"
	"github.com/saltosystems/winrt-go/windows/devices/enumeration"
	"github.com/saltosystems/winrt-go/windows/foundation"
)

var errNoPairingRequest = errors.New("bluetooth: no pairing request is waiting for an answer")

// Pair pairs and bonds this host with the connected peripheral, and blocks until
// pairing ends. It returns nil on success, also when the device was already paired.
func (d Device) Pair(params PairingParams) (err error) {
	defer func() {
		// Reject a request the application never answered.
		d.pairing.answer(nil)
		if params.PairingCompleteHandler != nil {
			params.PairingCompleteHandler(d, err)
		}
	}()

	pairing, err := d.devicePairing()
	if err != nil {
		return err
	}
	defer pairing.Release()

	custom, err := pairing.GetCustom()
	if err != nil {
		return err
	}
	defer custom.Release()

	guid := winrt.ParameterizedInstanceGUID(
		foundation.GUIDTypedEventHandler,
		enumeration.SignatureDeviceInformationCustomPairing,
		enumeration.SignatureDevicePairingRequestedEventArgs,
	)
	handler := foundation.NewTypedEventHandler(ole.NewGUID(guid), func(_ *foundation.TypedEventHandler, _, arg unsafe.Pointer) {
		d.onPairingRequested(params, (*enumeration.DevicePairingRequestedEventArgs)(arg))
	})
	defer handler.Release()

	token, err := custom.AddPairingRequested(handler)
	if err != nil {
		return err
	}
	defer custom.RemovePairingRequested(token)

	level := enumeration.DevicePairingProtectionLevelEncryption
	if params.MITM {
		level = enumeration.DevicePairingProtectionLevelEncryptionAndAuthentication
	}
	op, err := custom.PairWithProtectionLevelAsync(params.IOCapabilities.pairingKinds(), level)
	if err != nil {
		return err
	}
	defer op.Release()

	if err := awaitAsyncOperation(op, enumeration.SignatureDevicePairingResult); err != nil {
		return fmt.Errorf("bluetooth: pairing failed: %w", err)
	}
	res, err := op.GetResults()
	if err != nil {
		return err
	}
	result := (*enumeration.DevicePairingResult)(res)
	defer result.Release()

	status, err := result.GetStatus()
	if err != nil {
		return err
	}
	switch status {
	case enumeration.DevicePairingResultStatusPaired, enumeration.DevicePairingResultStatusAlreadyPaired:
		return nil
	}
	return fmt.Errorf("%w: %s", PairingError(status), pairingStatusText(status))
}

// Unpair removes this host's bond with the device. It returns nil if the
// device is not paired.
func (d Device) Unpair() error {
	pairing, err := d.devicePairing()
	if err != nil {
		return err
	}
	defer pairing.Release()

	op, err := pairing.UnpairAsync()
	if err != nil {
		return err
	}
	defer op.Release()

	if err := awaitAsyncOperation(op, enumeration.SignatureDeviceUnpairingResult); err != nil {
		return fmt.Errorf("bluetooth: unpairing failed: %w", err)
	}
	res, err := op.GetResults()
	if err != nil {
		return err
	}
	result := (*enumeration.DeviceUnpairingResult)(res)
	defer result.Release()

	status, err := result.GetStatus()
	if err != nil {
		return err
	}
	switch status {
	case enumeration.DeviceUnpairingResultStatusUnpaired, enumeration.DeviceUnpairingResultStatusAlreadyUnpaired:
		return nil
	}
	return fmt.Errorf("bluetooth: unpairing failed (status %d)", status)
}

// EnterPasskey answers PairingParams.PasskeyEntryHandler with the 6 digit
// passkey shown by the peripheral. An empty passkey rejects the pairing.
func (d Device) EnterPasskey(passkey string) error {
	if passkey == "" {
		return d.pairing.answer(nil)
	}
	if !isValidPasskey(passkey) {
		return errInvalidPasskey
	}
	return d.pairing.answer(func(args *enumeration.DevicePairingRequestedEventArgs) error {
		return args.AcceptWithPin(passkey)
	})
}

// ConfirmPasskey answers PairingParams.PasskeyComparisonHandler. match reports
// whether both devices show the same passkey.
func (d Device) ConfirmPasskey(match bool) error {
	if !match {
		return d.pairing.answer(nil)
	}
	return d.pairing.answer(func(args *enumeration.DevicePairingRequestedEventArgs) error {
		return args.Accept()
	})
}

func (d Device) devicePairing() (*enumeration.DeviceInformationPairing, error) {
	if d.device == nil {
		return nil, errors.New("bluetooth: not connected")
	}
	info, err := d.device.GetDeviceInformation()
	if err != nil {
		return nil, err
	}
	defer info.Release()
	return info.GetPairing()
}

// onPairingRequested answers now, or defers until the application answers.
// https://learn.microsoft.com/en-us/uwp/api/windows.devices.enumeration.devicepairingrequestedeventargs.getdeferral
func (d Device) onPairingRequested(params PairingParams, args *enumeration.DevicePairingRequestedEventArgs) {
	kind, err := args.GetPairingKind()
	if err != nil {
		return
	}
	switch kind {
	case enumeration.DevicePairingKindsConfirmOnly:
		_ = args.Accept()
	case enumeration.DevicePairingKindsDisplayPin:
		if params.PasskeyDisplayHandler != nil {
			pin, _ := args.GetPin()
			params.PasskeyDisplayHandler(d, pin)
		}
		_ = args.Accept()
	case enumeration.DevicePairingKindsConfirmPinMatch:
		if params.PasskeyComparisonHandler == nil {
			return
		}
		pin, _ := args.GetPin()
		if d.pairing.wait(args) == nil {
			params.PasskeyComparisonHandler(d, pin)
		}
	case enumeration.DevicePairingKindsProvidePin:
		if params.PasskeyEntryHandler == nil {
			return
		}
		if d.pairing.wait(args) == nil {
			params.PasskeyEntryHandler(d)
		}
	}
}

// pairingRequest is a deferred pairing request waiting for EnterPasskey or
// ConfirmPasskey.
type pairingRequest struct {
	mu       sync.Mutex
	args     *enumeration.DevicePairingRequestedEventArgs
	deferral *foundation.Deferral
}

func (r *pairingRequest) wait(args *enumeration.DevicePairingRequestedEventArgs) error {
	deferral, err := args.GetDeferral()
	if err != nil {
		return err
	}
	args.AddRef()
	r.mu.Lock()
	defer r.mu.Unlock()
	r.args, r.deferral = args, deferral
	return nil
}

// answer completes the waiting request. A nil accept rejects it.
func (r *pairingRequest) answer(accept func(*enumeration.DevicePairingRequestedEventArgs) error) error {
	if r == nil {
		return errNoPairingRequest
	}
	r.mu.Lock()
	args, deferral := r.args, r.deferral
	r.args, r.deferral = nil, nil
	r.mu.Unlock()
	if args == nil {
		return errNoPairingRequest
	}
	defer args.Release()
	defer deferral.Release()
	defer deferral.Complete()
	if accept == nil {
		return nil
	}
	return accept(args)
}

// pairingKinds returns the pairing methods these IO capabilities allow.
// See Bluetooth Core Specification v5.4, Vol 3, Part H, Section 2.3.5.1.
func (io IOCapabilities) pairingKinds() enumeration.DevicePairingKinds {
	kinds := enumeration.DevicePairingKindsConfirmOnly
	switch io {
	case IOCapsDisplayOnly:
		kinds |= enumeration.DevicePairingKindsDisplayPin
	case IOCapsDisplayYesNo:
		kinds |= enumeration.DevicePairingKindsDisplayPin | enumeration.DevicePairingKindsConfirmPinMatch
	case IOCapsKeyboardOnly:
		kinds |= enumeration.DevicePairingKindsProvidePin
	case IOCapsKeyboardDisplay:
		kinds |= enumeration.DevicePairingKindsDisplayPin | enumeration.DevicePairingKindsProvidePin | enumeration.DevicePairingKindsConfirmPinMatch
	}
	return kinds
}

func pairingStatusText(status enumeration.DevicePairingResultStatus) string {
	switch status {
	case enumeration.DevicePairingResultStatusNotReadyToPair:
		return "not ready to pair"
	case enumeration.DevicePairingResultStatusNotPaired:
		return "not paired"
	case enumeration.DevicePairingResultStatusConnectionRejected:
		return "connection rejected"
	case enumeration.DevicePairingResultStatusTooManyConnections:
		return "too many connections"
	case enumeration.DevicePairingResultStatusHardwareFailure:
		return "hardware failure"
	case enumeration.DevicePairingResultStatusAuthenticationTimeout:
		return "authentication timeout"
	case enumeration.DevicePairingResultStatusAuthenticationNotAllowed:
		return "authentication not allowed"
	case enumeration.DevicePairingResultStatusAuthenticationFailure:
		return "authentication failure"
	case enumeration.DevicePairingResultStatusNoSupportedProfiles:
		return "no supported profiles"
	case enumeration.DevicePairingResultStatusProtectionLevelCouldNotBeMet:
		return "protection level could not be met"
	case enumeration.DevicePairingResultStatusAccessDenied:
		return "access denied"
	case enumeration.DevicePairingResultStatusInvalidCeremonyData:
		return "invalid ceremony data"
	case enumeration.DevicePairingResultStatusPairingCanceled:
		return "pairing canceled"
	case enumeration.DevicePairingResultStatusOperationAlreadyInProgress:
		return "operation already in progress"
	case enumeration.DevicePairingResultStatusRequiredHandlerNotRegistered:
		return "required handler not registered"
	case enumeration.DevicePairingResultStatusRejectedByHandler:
		return "rejected by handler"
	case enumeration.DevicePairingResultStatusRemoteDeviceHasAssociation:
		return "remote device has association"
	}
	return "failed"
}
