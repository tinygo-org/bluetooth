package bluetooth

import (
	"errors"
	"io"
	"runtime"
	"sync"
	"time"

	"github.com/tinygo-org/cbgo"
)

// l2capResult is used internally to communicate the result of an L2CAP channel
// open operation from the delegate callback to the calling goroutine.
type l2capResult struct {
	channel cbgo.L2CAPChannel
	err     error
}

// L2CAPConn represents an L2CAP Connection-Oriented Channel connection.
// It implements io.ReadWriteCloser for bidirectional communication.
type L2CAPConn struct {
	channel cbgo.L2CAPChannel
	mu      sync.Mutex
	closed  bool
}

// Compile-time check that L2CAPConn implements io.ReadWriteCloser.
var _ io.ReadWriteCloser = (*L2CAPConn)(nil)

// OpenL2CAPChannel opens an L2CAP Connection-Oriented Channel to the
// connected peripheral. The PSM (Protocol/Service Multiplexer) identifies the
// L2CAP service to connect to on the remote device.
//
// The device must already be connected via Connect before calling this method.
func (d Device) OpenL2CAPChannel(psm L2CAPPSM) (*L2CAPConn, error) {
	ch := make(chan l2capResult, 1)
	d.l2capChan = ch
	defer func() { d.l2capChan = nil }()

	d.prph.OpenL2CAPChannel(psm)

	select {
	case result := <-ch:
		if result.err != nil {
			return nil, result.err
		}
		return &L2CAPConn{
			channel: result.channel,
		}, nil
	case <-time.NewTimer(30 * time.Second).C:
		return nil, errors.New("bluetooth: timeout on OpenL2CAPChannel")
	}
}

// Read reads data from the L2CAP channel. It blocks until data is available,
// the channel is closed locally, or the remote end closes the stream (EOF).
func (c *L2CAPConn) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}

	for {
		c.mu.Lock()
		if c.closed {
			c.mu.Unlock()
			return 0, io.ErrClosedPipe
		}
		c.mu.Unlock()

		n := c.channel.Read(p)
		if n < 0 {
			return 0, errors.New("bluetooth: L2CAP read error")
		}
		if n > 0 {
			return n, nil
		}

		// c.channel.Read returns 0 in two cases:
		//  - hasBytesAvailable was false → no data yet, retry.
		//  - hasBytesAvailable was true but NSInputStream read returned 0 → EOF.
		// Distinguish them by checking HasBytesAvailable after a 0-byte read.
		if !c.channel.HasBytesAvailable() {
			// No data available yet, yield and retry.
			runtime.Gosched()
			continue
		}
		// The stream reported bytes available yet Read returned 0:
		// the input stream has reached end-of-stream.
		return 0, io.EOF
	}
}

// Write writes data to the L2CAP channel. It blocks until all data has been
// written or the channel is closed.
func (c *L2CAPConn) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}

	total := 0
	for total < len(p) {
		c.mu.Lock()
		if c.closed {
			c.mu.Unlock()
			return total, io.ErrClosedPipe
		}
		c.mu.Unlock()

		// Output stream not ready yet, yield and retry.
		if !c.channel.HasSpaceAvailable() {
			runtime.Gosched()
			continue
		}

		n := c.channel.Write(p[total:])
		if n < 0 {
			return total, errors.New("bluetooth: L2CAP write error")
		}
		if n > 0 {
			total += n
			continue
		}
	}
	return total, nil
}

// Close closes the L2CAP channel.
func (c *L2CAPConn) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil
	}
	c.closed = true
	c.channel.Close()
	return nil
}

// PSM returns the Protocol/Service Multiplexer identifier for this channel.
func (c *L2CAPConn) PSM() L2CAPPSM {
	return c.channel.PSM()
}
