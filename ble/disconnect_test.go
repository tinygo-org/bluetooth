package ble

import "testing"

func TestDisconnectReasonError(t *testing.T) {
	tests := []struct {
		reason DisconnectReason
		want   string
	}{
		{DisconnectTimeout, "HCI error 0x08 (Connection Timeout)"},
		{DisconnectRemoteUser, "HCI error 0x13 (Remote User Terminated Connection)"},
		{0x01, "HCI error 0x01 (Unknown)"},
	}
	for _, tt := range tests {
		if got := tt.reason.Error(); got != tt.want {
			t.Errorf("DisconnectReason(%#x).Error() = %q, want %q", tt.reason.Code(), got, tt.want)
		}
	}
}
