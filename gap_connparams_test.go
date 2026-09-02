package bluetooth

import (
	"testing"
	"time"
)

// connectionPriorities are the priorities that give parameters.
var connectionPriorities = []ConnectionPriority{
	ConnectionPriorityThroughput,
	ConnectionPriorityBalanced,
	ConnectionPriorityPowerSaving,
}

// The presets must obey the guidelines from Apple (section 35.6 Connection
// Parameters): https://developer.apple.com/accessories/Accessory-Design-Guidelines.pdf
func TestConnectionPriorityParams(t *testing.T) {
	const (
		unit         = 625 * time.Microsecond
		intervalStep = 15 * time.Millisecond
		minTimeout   = 2 * time.Second
		maxTimeout   = 6 * time.Second
	)

	if got := ConnectionPriorityUnspecified.Params(); got != (ConnectionParams{}) {
		t.Errorf("ConnectionPriorityUnspecified.Params() = %+v, want the zero value", got)
	}

	for _, priority := range connectionPriorities {
		params := priority.Params()
		if params.Priority != priority {
			t.Errorf("%s: Params().Priority = %s, want %s", priority, params.Priority, priority)
		}

		minInterval := time.Duration(params.MinInterval) * unit
		maxInterval := time.Duration(params.MaxInterval) * unit
		timeout := time.Duration(params.Timeout) * unit

		if minInterval%intervalStep != 0 || maxInterval%intervalStep != 0 {
			t.Errorf("%s: intervals %v/%v are not multiples of %v", priority, minInterval, maxInterval, intervalStep)
		}
		if maxInterval-minInterval < intervalStep {
			t.Errorf("%s: MaxInterval %v is less than %v above MinInterval %v", priority, maxInterval, intervalStep, minInterval)
		}
		if timeout < minTimeout || timeout > maxTimeout {
			t.Errorf("%s: supervision timeout %v outside [%v, %v]", priority, timeout, minTimeout, maxTimeout)
		}

		// The SoftDevice rejects a supervision timeout that is not longer than
		// the time that the peripheral can stay silent.
		silence := time.Duration(1+params.PeripheralLatency) * maxInterval * 2
		if timeout <= silence {
			t.Errorf("%s: supervision timeout %v does not exceed %v of allowed silence "+
				"(peripheral latency %d, max interval %v)",
				priority, timeout, silence, params.PeripheralLatency, maxInterval)
		}
	}
}

func TestConnectionParamsResolved(t *testing.T) {
	t.Run("an unset priority changes nothing", func(t *testing.T) {
		params := ConnectionParams{MinInterval: NewDuration(20 * time.Millisecond)}
		if got := params.Resolved(); got != params {
			t.Errorf("Resolved() = %+v, want %+v", got, params)
		}
	})

	t.Run("a priority fills the unset fields", func(t *testing.T) {
		got := ConnectionParams{Priority: ConnectionPriorityPowerSaving}.Resolved()
		want := ConnectionPriorityPowerSaving.Params()
		if got != want {
			t.Errorf("Resolved() = %+v, want %+v", got, want)
		}
	})

	t.Run("explicit values stay", func(t *testing.T) {
		timeout := NewDuration(3 * time.Second)
		got := ConnectionParams{Priority: ConnectionPriorityBalanced, Timeout: timeout}.Resolved()
		if got.Timeout != timeout {
			t.Errorf("Resolved().Timeout = %d, want the explicit %d", got.Timeout, timeout)
		}
		if got.MaxInterval != ConnectionPriorityBalanced.Params().MaxInterval {
			t.Error("Resolved() did not fill the interval from the priority")
		}
	})
}
