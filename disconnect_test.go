package bluetooth

import (
	"errors"
	"testing"
)

func TestDisconnectError(t *testing.T) {
	if err := disconnectError(0x16); err != nil {
		t.Errorf("disconnectError(0x16) = %v, want nil", err)
	}

	var reason DisconnectReason
	if !errors.As(disconnectError(0x08), &reason) || reason != DisconnectTimeout {
		t.Errorf("disconnectError(0x08) = %v, want DisconnectTimeout", disconnectError(0x08))
	}
}
