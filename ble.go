package bluetooth

import "tinygo.org/x/bluetooth/ble"

// The core value types live in the ble subpackage, so that the protocol
// subpackages can use them without an import cycle. These are aliases, not new
// types, so bluetooth.UUID and ble.UUID are interchangeable.
type (
	// UUID is a 16-bit, 32-bit, or 128-bit BLE UUID.
	UUID = ble.UUID

	// MAC represents a MAC address, in little endian format.
	MAC = ble.MAC

	// MACAddress contains a Bluetooth address which is a MAC address.
	MACAddress = ble.MACAddress
)

// NewUUID returns a new 128-bit UUID for a 16 byte array in big endian order.
func NewUUID(uuid [16]byte) UUID { return ble.NewUUID(uuid) }

// New16BitUUID returns a new 128-bit UUID based on a 16-bit UUID.
func New16BitUUID(shortUUID uint16) UUID { return ble.New16BitUUID(shortUUID) }

// New32BitUUID returns a new 128-bit UUID based on a 32-bit UUID.
func New32BitUUID(shortUUID uint32) UUID { return ble.New32BitUUID(shortUUID) }

// ParseUUID parses the given UUID, which must be in one of the following
// forms: 0000, 00000000, or 00000000-0000-1000-8000-00805F9B34FB.
func ParseUUID(s string) (UUID, error) { return ble.ParseUUID(s) }

// UUIDFromBytes returns the UUID for a 16 byte array in little endian order.
func UUIDFromBytes(b [16]byte) UUID { return ble.UUIDFromBytes(b) }

// ParseMAC parses the given MAC address, which must be in
// 11:22:33:AA:BB:CC format.
func ParseMAC(s string) (MAC, error) { return ble.ParseMAC(s) }

// NewMACAddress returns a MACAddress for the given MAC, marked as random or
// public.
func NewMACAddress(mac MAC, random bool) MACAddress { return ble.NewMACAddress(mac, random) }

// The error values are aliased so that a comparison against the old name still
// holds.
var (
	ErrInvalidMAC        = ble.ErrInvalidMAC
	ErrInvalidBinaryMac  = ble.ErrInvalidBinaryMac
	ErrInvalidBinaryUUID = ble.ErrInvalidBinaryUUID
)
