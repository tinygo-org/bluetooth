//go:build (softdevice && s113v7) || (softdevice && s132v6) || (softdevice && s140v6) || (softdevice && s140v7)

package bluetooth

import (
	"crypto/ecdh"
	"errors"
	"runtime/volatile"
	"time"
)

/*
#include "ble.h"
#include "ble_gap.h"
#include "nrf_soc.h"
*/
import "C"

var (
	errPairingNotEnabled = errors.New("bluetooth: pairing is not enabled")
	errInvalidPasskey    = errors.New("bluetooth: passkey must be 6 ASCII digits")
)

// Security event flags, set from the SoftDevice event handler (interrupt
// context) and consumed by the security worker goroutine.
const (
	flagSecParamsRequest = 1 << iota
	flagPasskeyDisplay
	flagNumericComparison
	flagAuthKeyRequest
	flagLESCDHKeyRequest
	flagAuthStatus
	flagSecInfoRequest
)

// Static buffers for the pairing procedure. The SoftDevice requires the
// keyset memory to stay valid until the BLE_GAP_EVT_AUTH_STATUS event, so
// package-level variables are used. This limits pairing to one procedure at a
// time, matching the single-connection design of this backend.
var (
	pairingConfig  PairingParams
	pairingEnabled volatile.Register8
	workerStarted  bool

	secParamsReply C.ble_gap_sec_params_t
	secKeyset      C.ble_gap_sec_keyset_t
	ownPubkey      C.ble_gap_lesc_p256_pk_t // our LESC public key, in SMP format
	peerPubkey     C.ble_gap_lesc_p256_pk_t // the SoftDevice stores the peer key here
	lescDHKey      C.ble_gap_lesc_dhkey_t
	lescPrivateKey *ecdh.PrivateKey
	staticPasskey  [6]C.uint8_t
	authKeyBuf     [6]C.uint8_t

	// Mailbox from the event handler to the security worker. The buffers are
	// written before the flag is set, and the flag is cleared inside a
	// critical region.
	secEventFlags    volatile.Register8
	secEventConn     = volatileHandle{handle: volatile.Register16{C.BLE_CONN_HANDLE_INVALID}}
	passkeyBuf       [6]C.uint8_t
	authStatusCode   volatile.Register8
	authStatusBonded volatile.Register8
	secInfoMasterID  C.ble_gap_master_id_t
	secInfoEncReq    volatile.Register8

	// RAM-only bond storage: the LTK from the last bonding procedure. It is
	// lost on reset, after which the central has to delete the bond and pair
	// again. bondValid is only accessed by the security worker.
	ownEncKey C.ble_gap_enc_key_t
	bondValid bool
)

// EnablePairing configures the adapter to accept pairing and bonding requests
// from a connected central. It must be called after Enable() and before a
// central connects. Without it, incoming pairing requests are rejected with
// "pairing not supported".
//
// Bonding keys are kept in RAM only: a bonded central can re-encrypt the link
// on reconnection without pairing again, but after a reset of this device the
// central has to delete the bond and pair again.
func (a *Adapter) EnablePairing(params PairingParams) error {
	if params.StaticPasskey != "" {
		if !isValidPasskey(params.StaticPasskey) {
			return errInvalidPasskey
		}
		for i := 0; i < 6; i++ {
			staticPasskey[i] = C.uint8_t(params.StaticPasskey[i])
		}
		var opt C.ble_opt_t
		opt.unionfield_gap_opt().unionfield_passkey().p_passkey = &staticPasskey[0]
		errCode := C.sd_ble_opt_set(C.BLE_GAP_OPT_PASSKEY, &opt)
		if errCode != 0 {
			return Error(errCode)
		}
	}

	secParamsReply = C.ble_gap_sec_params_t{
		min_key_size: 7,
		max_key_size: 16,
	}
	// Advertise bonding support: common centrals (Windows, iOS) do not
	// complete their pairing flow without a bond. The keys are only kept in
	// RAM: after a reset the central has to delete the bond and pair again.
	secParamsReply.set_bitfield_bond(1)
	secParamsReply.kdist_own.set_bitfield_enc(1)
	if params.MITM {
		secParamsReply.set_bitfield_mitm(1)
	}
	if params.LESC {
		secParamsReply.set_bitfield_lesc(1)
	}
	secParamsReply.set_bitfield_io_caps(params.IOCapabilities.sdIOCaps())
	secParamsReply.set_bitfield_oob(0)

	// The SoftDevice stores keys generated during pairing in application
	// memory referenced by this keyset. The own encryption key receives the
	// LTK (distributed by us in legacy pairing, generated locally in LESC)
	// that a bonded central uses to re-encrypt the link on reconnection.
	// keys_peer.p_enc_key must stay NULL for LESC and is not needed in the
	// peripheral role.
	secKeyset.keys_own.p_enc_key = &ownEncKey
	secKeyset.keys_own.p_pk = &ownPubkey
	secKeyset.keys_peer.p_pk = &peerPubkey

	if params.LESC {
		if err := lescGenerateKeypair(); err != nil {
			return err
		}
	}

	pairingConfig = params
	if !workerStarted {
		workerStarted = true
		go securityWorker()
	}
	pairingEnabled.Set(1)
	return nil
}

// RequestPairing sends a security request to the connected central, asking it
// to initiate pairing. The central may ignore the request. EnablePairing must
// have been called first.
func (d Device) RequestPairing() error {
	if pairingEnabled.Get() == 0 {
		return errPairingNotEnabled
	}
	errCode := C.sd_ble_gap_authenticate(d.connectionHandle, &secParamsReply)
	return makeError(errCode)
}

// EnterPasskey supplies the 6-digit passkey shown by the central, in response
// to PairingParams.PasskeyEntryHandler. Pass an empty string to reject the
// pairing.
func (d Device) EnterPasskey(passkey string) error {
	if passkey == "" {
		errCode := C.sd_ble_gap_auth_key_reply(d.connectionHandle, C.BLE_GAP_AUTH_KEY_TYPE_NONE, nil)
		return makeError(errCode)
	}
	if !isValidPasskey(passkey) {
		return errInvalidPasskey
	}
	for i := 0; i < 6; i++ {
		authKeyBuf[i] = C.uint8_t(passkey[i])
	}
	errCode := C.sd_ble_gap_auth_key_reply(d.connectionHandle, C.BLE_GAP_AUTH_KEY_TYPE_PASSKEY, &authKeyBuf[0])
	return makeError(errCode)
}

// ConfirmPasskey answers a numeric comparison request from
// PairingParams.PasskeyComparisonHandler: match reports whether the user
// confirmed that both devices show the same passkey.
func (d Device) ConfirmPasskey(match bool) error {
	keyType := C.uint8_t(C.BLE_GAP_AUTH_KEY_TYPE_NONE)
	if match {
		keyType = C.BLE_GAP_AUTH_KEY_TYPE_PASSKEY
	}
	errCode := C.sd_ble_gap_auth_key_reply(d.connectionHandle, keyType, nil)
	return makeError(errCode)
}

func isValidPasskey(passkey string) bool {
	if len(passkey) != 6 {
		return false
	}
	for i := 0; i < 6; i++ {
		if passkey[i] < '0' || passkey[i] > '9' {
			return false
		}
	}
	return true
}

func (io IOCapabilities) sdIOCaps() C.uint8_t {
	switch io {
	case IOCapsDisplayOnly:
		return C.BLE_GAP_IO_CAPS_DISPLAY_ONLY
	case IOCapsDisplayYesNo:
		return C.BLE_GAP_IO_CAPS_DISPLAY_YESNO
	case IOCapsKeyboardOnly:
		return C.BLE_GAP_IO_CAPS_KEYBOARD_ONLY
	case IOCapsKeyboardDisplay:
		return C.BLE_GAP_IO_CAPS_KEYBOARD_DISPLAY
	default:
		return C.BLE_GAP_IO_CAPS_NONE
	}
}

// The following functions are called from the SoftDevice event handler, in
// interrupt context. They only record the event for the security worker,
// except for replies that need no application input (which are single SVC
// calls, like the other replies already done in the event handler).

func secSetFlag(flag uint8) {
	secEventFlags.Set(secEventFlags.Get() | flag)
}

func secClearFlag(flag uint8) {
	mask := DisableInterrupts()
	secEventFlags.Set(secEventFlags.Get() &^ flag)
	RestoreInterrupts(mask)
}

func secOnSecParamsRequest(connHandle C.uint16_t) {
	if pairingEnabled.Get() == 0 {
		// Pairing is not configured: politely reject instead of letting the
		// central wait for the SMP timeout.
		C.sd_ble_gap_sec_params_reply(connHandle, C.BLE_GAP_SEC_STATUS_PAIRING_NOT_SUPP, nil, nil)
		return
	}
	secEventConn.Set(connHandle)
	secSetFlag(flagSecParamsRequest)
}

func secOnSecInfoRequest(connHandle C.uint16_t, evt *C.ble_gap_evt_sec_info_request_t) {
	if pairingEnabled.Get() == 0 {
		// No pairing support configured, so no keys can be stored: reply that
		// the keys are lost so the central can re-pair.
		C.sd_ble_gap_sec_info_reply(connHandle, nil, nil, nil)
		return
	}
	secInfoMasterID = evt.master_id
	secInfoEncReq.Set(uint8(evt.bitfield_enc_info()))
	secEventConn.Set(connHandle)
	secSetFlag(flagSecInfoRequest)
}

func secOnPasskeyDisplay(connHandle C.uint16_t, evt *C.ble_gap_evt_passkey_display_t) {
	passkeyBuf = evt.passkey
	secEventConn.Set(connHandle)
	// match_request is a single bitfield, which TinyGo's CGo exposes as a
	// plain byte field (bit 0).
	if evt.match_request&1 != 0 {
		secSetFlag(flagNumericComparison)
	} else {
		secSetFlag(flagPasskeyDisplay)
	}
}

func secOnAuthKeyRequest(connHandle C.uint16_t) {
	secEventConn.Set(connHandle)
	secSetFlag(flagAuthKeyRequest)
}

func secOnLESCDHKeyRequest(connHandle C.uint16_t) {
	// The peer public key has been stored by the SoftDevice in peerPubkey
	// (via the keyset), so only the request itself must be recorded.
	secEventConn.Set(connHandle)
	secSetFlag(flagLESCDHKeyRequest)
}

func secOnAuthStatus(connHandle C.uint16_t, evt *C.ble_gap_evt_auth_status_t) {
	if pairingEnabled.Get() == 0 {
		return
	}
	authStatusCode.Set(uint8(evt.auth_status))
	authStatusBonded.Set(uint8(evt.bitfield_bonded()))
	secEventConn.Set(connHandle)
	secSetFlag(flagAuthStatus)
}

// securityWorker runs all pairing replies and application callbacks outside
// interrupt context: replies may need to be retried (NRF_ERROR_BUSY), the
// LESC DHKey computation is too slow for an interrupt handler, and callbacks
// may block on user input.
func securityWorker() {
	for {
		flags := secEventFlags.Get()
		if flags == 0 {
			time.Sleep(16 * time.Millisecond)
			continue
		}
		connHandle := secEventConn.Get()
		device := Device{connectionHandle: connHandle}
		switch {
		case flags&flagSecParamsRequest != 0:
			secClearFlag(flagSecParamsRequest)
			for {
				errCode := C.sd_ble_gap_sec_params_reply(connHandle, C.BLE_GAP_SEC_STATUS_SUCCESS, &secParamsReply, &secKeyset)
				if errCode != 17 { // C.NRF_ERROR_BUSY, which TinyGo's CGo cannot parse
					if debug && errCode != 0 {
						println("sec params reply failed:", errCode)
					}
					break
				}
				time.Sleep(time.Millisecond)
			}
		case flags&flagPasskeyDisplay != 0:
			secClearFlag(flagPasskeyDisplay)
			if handler := pairingConfig.PasskeyDisplayHandler; handler != nil {
				handler(device, passkeyString())
			}
		case flags&flagNumericComparison != 0:
			secClearFlag(flagNumericComparison)
			if handler := pairingConfig.PasskeyComparisonHandler; handler != nil {
				handler(device, passkeyString())
			} else {
				// No way to ask the user: fail closed.
				device.ConfirmPasskey(false)
			}
		case flags&flagAuthKeyRequest != 0:
			secClearFlag(flagAuthKeyRequest)
			if handler := pairingConfig.PasskeyEntryHandler; handler != nil {
				handler(device)
			} else {
				device.EnterPasskey("")
			}
		case flags&flagLESCDHKeyRequest != 0:
			secClearFlag(flagLESCDHKeyRequest)
			lescComputeAndReply(connHandle)
		case flags&flagAuthStatus != 0:
			secClearFlag(flagAuthStatus)
			var err error
			if code := authStatusCode.Get(); code != C.BLE_GAP_SEC_STATUS_SUCCESS {
				err = PairingError(code)
			}
			bondValid = err == nil && authStatusBonded.Get() != 0
			if handler := pairingConfig.PairingCompleteHandler; handler != nil {
				handler(device, err)
			}
		case flags&flagSecInfoRequest != 0:
			secClearFlag(flagSecInfoRequest)
			// A central that bonded with us before asks to encrypt the link
			// with the LTK identified by the master ID (all zero for LESC).
			// Without a matching key, reply that the keys are lost so the
			// central can re-pair.
			var encInfo *C.ble_gap_enc_info_t
			if bondValid && secInfoEncReq.Get() != 0 &&
				secInfoMasterID.ediv == ownEncKey.master_id.ediv &&
				secInfoMasterID.rand == ownEncKey.master_id.rand {
				encInfo = &ownEncKey.enc_info
			}
			errCode := C.sd_ble_gap_sec_info_reply(connHandle, encInfo, nil, nil)
			if debug && errCode != 0 {
				println("sec info reply failed:", errCode)
			}
		}
	}
}

func passkeyString() string {
	var passkey [6]byte
	for i := 0; i < 6; i++ {
		passkey[i] = byte(passkeyBuf[i])
	}
	return string(passkey[:])
}

// sdRand fills the buffer with random bytes from the SoftDevice random pool.
// The pool is smaller than a P-256 key seed and refills slowly, so the buffer
// is filled in as many steps as needed. The RNG peripheral itself is owned by
// the SoftDevice, so crypto/rand must not be used.
func sdRand(buf []byte) {
	for len(buf) > 0 {
		var available C.uint8_t
		C.sd_rand_application_bytes_available_get(&available)
		n := int(available)
		if n == 0 {
			time.Sleep(time.Millisecond)
			continue
		}
		if n > len(buf) {
			n = len(buf)
		}
		if C.sd_rand_application_vector_get((*C.uint8_t)(&buf[0]), C.uint8_t(n)) == 0 {
			buf = buf[n:]
		}
	}
}

// lescGenerateKeypair generates the P-256 keypair used for LE Secure
// Connections and stores the public key in ownPubkey, in the SMP format the
// SoftDevice expects: X and Y coordinates, both little-endian.
func lescGenerateKeypair() error {
	var seed [32]byte
	for {
		sdRand(seed[:])
		privateKey, err := ecdh.P256().NewPrivateKey(seed[:])
		if err != nil {
			// The seed was out of range for a P-256 scalar; try again.
			continue
		}
		lescPrivateKey = privateKey
		// SEC1 uncompressed format: 0x04 || X (big-endian) || Y (big-endian).
		publicKey := privateKey.PublicKey().Bytes()
		for i := 0; i < 32; i++ {
			ownPubkey.pk[i] = C.uint8_t(publicKey[1+31-i])
			ownPubkey.pk[32+i] = C.uint8_t(publicKey[33+31-i])
		}
		return nil
	}
}

// lescComputeAndReply computes the LESC DHKey from our private key and the
// peer public key received during pairing, and hands it to the SoftDevice.
func lescComputeAndReply(connHandle C.uint16_t) {
	var raw [65]byte
	raw[0] = 4 // SEC1 uncompressed point
	for i := 0; i < 32; i++ {
		raw[1+i] = byte(peerPubkey.pk[31-i])
		raw[33+i] = byte(peerPubkey.pk[63-i])
	}
	peerKey, err := ecdh.P256().NewPublicKey(raw[:])
	if err != nil {
		// Invalid peer public key (not a point on the curve): abort pairing.
		C.sd_ble_gap_disconnect(connHandle, C.BLE_HCI_AUTHENTICATION_FAILURE)
		return
	}
	shared, err := lescPrivateKey.ECDH(peerKey)
	if err != nil {
		C.sd_ble_gap_disconnect(connHandle, C.BLE_HCI_AUTHENTICATION_FAILURE)
		return
	}
	// The shared secret is the X coordinate, big-endian; the SoftDevice wants
	// it little-endian.
	for i := 0; i < 32; i++ {
		lescDHKey.key[i] = C.uint8_t(shared[31-i])
	}
	errCode := C.sd_ble_gap_lesc_dhkey_reply(connHandle, &lescDHKey)
	if debug && errCode != 0 {
		println("lesc dhkey reply failed:", errCode)
	}
}
