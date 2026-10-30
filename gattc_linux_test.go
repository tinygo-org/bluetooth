//go:build !baremetal

package bluetooth

import "testing"

func TestBluezFlags(t *testing.T) {
	for _, tc := range []struct {
		flags []string
		want  CharacteristicPermissions
	}{
		{nil, 0},
		{[]string{"read"}, CharacteristicReadPermission},
		{[]string{"write-without-response"}, CharacteristicWriteWithoutResponsePermission},
		{
			[]string{"read", "write", "notify"},
			CharacteristicReadPermission | CharacteristicWritePermission | CharacteristicNotifyPermission,
		},
		{
			[]string{"read", "authenticated-signed-writes", "reliable-write"},
			CharacteristicReadPermission,
		},
	} {
		if got := bluezFlags(tc.flags); got != tc.want {
			t.Errorf("%v gave %#x, want %#x", tc.flags, got, tc.want)
		}
	}
}

func TestBluezFlagsRoundTrip(t *testing.T) {
	for p := CharacteristicPermissions(0); p <= characteristicPermissionsMask; p++ {
		if got := bluezFlags(bluezFlagNames(p)); got != p {
			t.Errorf("%#x gave %#x", p, got)
		}
	}
}
