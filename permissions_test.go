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
			[]string{"encrypt-read", "encrypt-authenticated-write"},
			CharacteristicReadPermission | CharacteristicWritePermission,
		},
		{
			[]string{"read", "authenticated-signed-writes", "reliable-write", "authorize"},
			CharacteristicReadPermission,
		},
	} {
		if got := bluezFlags(tc.flags); got != tc.want {
			t.Errorf("%v gave %#x, want %#x", tc.flags, got, tc.want)
		}
	}
}
