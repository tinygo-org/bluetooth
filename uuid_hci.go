//go:build hci || ninafw || cyw43439 || espradio

package bluetooth

// uuidIn reports whether uuid is in uuids.
func uuidIn(uuid UUID, uuids []UUID) bool {
	for _, u := range uuids {
		if u == uuid {
			return true
		}
	}

	return false
}
