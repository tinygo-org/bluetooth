package ble

import "encoding/hex"

// DisconnectReason is the HCI error code that Nordic and HCI adapters report when
// a device disconnects. See Bluetooth Core Specification v5.4, Vol 1, Part F, Table 1.1.
type DisconnectReason uint8

// HCI error codes that end a connection, from Vol 1, Part F, Table 1.1.
const (
	DisconnectAuthenticationFailure    DisconnectReason = 0x05
	DisconnectKeyMissing               DisconnectReason = 0x06
	DisconnectTimeout                  DisconnectReason = 0x08
	DisconnectRemoteUser               DisconnectReason = 0x13
	DisconnectRemoteLowResources       DisconnectReason = 0x14
	DisconnectRemotePowerOff           DisconnectReason = 0x15
	DisconnectUnsupportedRemoteFeature DisconnectReason = 0x1A
	DisconnectResponseTimeout          DisconnectReason = 0x22
	DisconnectInstantPassed            DisconnectReason = 0x28
	DisconnectUnacceptableParameters   DisconnectReason = 0x3B
	DisconnectMICFailure               DisconnectReason = 0x3D
	DisconnectFailedToEstablish        DisconnectReason = 0x3E

	disconnectLocalHost DisconnectReason = 0x16
)

var disconnectReasonNames = map[DisconnectReason]string{
	DisconnectAuthenticationFailure:    "Authentication Failure",
	DisconnectKeyMissing:               "PIN or Key Missing",
	DisconnectTimeout:                  "Connection Timeout",
	DisconnectRemoteUser:               "Remote User Terminated Connection",
	DisconnectRemoteLowResources:       "Remote Device Terminated Connection due to Low Resources",
	DisconnectRemotePowerOff:           "Remote Device Terminated Connection due to Power Off",
	disconnectLocalHost:                "Connection Terminated By Local Host",
	DisconnectUnsupportedRemoteFeature: "Unsupported Remote Feature",
	DisconnectResponseTimeout:          "LL Response Timeout",
	DisconnectInstantPassed:            "Instant Passed",
	DisconnectUnacceptableParameters:   "Unacceptable Connection Parameters",
	DisconnectMICFailure:               "Connection Terminated due to MIC Failure",
	DisconnectFailedToEstablish:        "Connection Failed to be Established",
}

// Code returns the raw HCI error code.
func (r DisconnectReason) Code() uint8 {
	return uint8(r)
}

func (r DisconnectReason) Error() string {
	name, ok := disconnectReasonNames[r]
	if !ok {
		name = "Unknown"
	}
	return "HCI error 0x" + hex.EncodeToString([]byte{uint8(r)}) + " (" + name + ")"
}
