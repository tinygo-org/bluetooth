//go:build hci || ninafw || cyw43439

package bluetooth

type shortUUID uint16

// UUID returns the full length UUID for this short UUID.
func (s shortUUID) UUID() UUID {
	return New16BitUUID(uint16(s))
}

// isIn checks the passed in slice of UUIDs to see if this uuid is in it.
func (uuid UUID) isIn(uuids []UUID) bool {
	for _, u := range uuids {
		if u == uuid {
			return true
		}
	}
	return false
}
