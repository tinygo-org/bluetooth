package bluetooth

import "testing"

// The zero value has to stay ConnectionPriorityUnspecified, so that an unset
// Priority means "leave the connection alone" like every other field in
// ConnectionParams. Reordering the constants would silently change what an
// empty ConnectionParams asks for.
func TestConnectionPriorityZeroValue(t *testing.T) {
	var priority ConnectionPriority
	if priority != ConnectionPriorityUnspecified {
		t.Errorf("zero value = %d, want ConnectionPriorityUnspecified", priority)
	}
	if got := (ConnectionParams{}).Priority; got != ConnectionPriorityUnspecified {
		t.Errorf("ConnectionParams{}.Priority = %s, want unspecified", got)
	}
}

func TestConnectionPriorityString(t *testing.T) {
	tests := []struct {
		priority ConnectionPriority
		want     string
	}{
		{ConnectionPriorityUnspecified, "unspecified"},
		{ConnectionPriorityThroughput, "throughput"},
		{ConnectionPriorityBalanced, "balanced"},
		{ConnectionPriorityPowerSaving, "power-saving"},
		{ConnectionPriority(99), "unspecified"},
	}
	for _, test := range tests {
		if got := test.priority.String(); got != test.want {
			t.Errorf("ConnectionPriority(%d).String() = %q, want %q", test.priority, got, test.want)
		}
	}
}
