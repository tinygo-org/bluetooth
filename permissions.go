package bluetooth

// bluezFlags changes BlueZ characteristic flags to permissions. BlueZ shows a
// protected flag such as "encrypt-read" in place of "read".
// See BlueZ doc/org.bluez.GattCharacteristic.rst, the Flags property.
func bluezFlags(flags []string) (p CharacteristicPermissions) {
	for _, f := range flags {
		switch f {
		case "broadcast":
			p |= CharacteristicBroadcastPermission
		case "read", "encrypt-read", "encrypt-authenticated-read", "secure-read":
			p |= CharacteristicReadPermission
		case "write-without-response":
			p |= CharacteristicWriteWithoutResponsePermission
		case "write", "encrypt-write", "encrypt-authenticated-write", "secure-write":
			p |= CharacteristicWritePermission
		case "notify", "encrypt-notify", "encrypt-authenticated-notify", "secure-notify":
			p |= CharacteristicNotifyPermission
		case "indicate", "encrypt-indicate", "encrypt-authenticated-indicate", "secure-indicate":
			p |= CharacteristicIndicatePermission
		}
	}

	return p
}
