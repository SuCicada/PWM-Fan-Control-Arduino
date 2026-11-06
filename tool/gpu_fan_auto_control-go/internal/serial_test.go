package internal

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type fakePort struct {
	mu      sync.Mutex
	cond    *sync.Cond
	reads   []portChunk
	closed  bool
	waiting chan struct{}
	once    sync.Once
}

type portChunk struct {
	data []byte
	err  error
}

func newFakePort() *fakePort {
	p := &fakePort{waiting: make(chan struct{})}
	p.cond = sync.NewCond(&p.mu)
	return p
}

func (p *fakePort) push(line string) {
	p.mu.Lock()
	p.reads = append(p.reads, portChunk{data: []byte(line)})
	p.cond.Signal()
	p.mu.Unlock()
}

func (p *fakePort) fail(err error) {
	p.mu.Lock()
	p.reads = append(p.reads, portChunk{err: err})
	p.cond.Signal()
	p.mu.Unlock()
}

func (p *fakePort) Read(buf []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for len(p.reads) == 0 && !p.closed {
		p.once.Do(func() { close(p.waiting) })
		p.cond.Wait()
	}
	if len(p.reads) == 0 {
		return 0, errors.New("Port has been closed")
	}
	chunk := p.reads[0]
	p.reads = p.reads[1:]
	if chunk.err != nil {
		return 0, chunk.err
	}
	return copy(buf, chunk.data), nil
}

func (p *fakePort) Write(buf []byte) (int, error) {
	return len(buf), nil
}

func (p *fakePort) Close() error {
	p.mu.Lock()
	p.closed = true
	p.cond.Broadcast()
	p.mu.Unlock()
	return nil
}

func waitUntil(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("condition not met before deadline")
}

func TestMonitorReopensAfterPortClosed(t *testing.T) {
	first := newFakePort()
	second := newFakePort()
	var opens atomic.Int32

	sc := NewSerialController("/dev/ttyUSB0")
	sc.retryMin = 5 * time.Millisecond
	sc.retryMax = 20 * time.Millisecond
	sc.openPort = func(string) (serialPort, error) {
		if opens.Add(1) == 1 {
			return first, nil
		}
		return second, nil
	}

	first.push("state:1290:10\n")
	if err := sc.EnsureReading(); err != nil {
		t.Fatalf("EnsureReading: %v", err)
	}
	t.Cleanup(func() { _ = sc.Close() })

	waitUntil(t, func() bool {
		res := sc.Last()
		return res != nil && res.RPM == 1290 && res.Speed == 10
	})
	first.fail(errors.New("Port has been closed"))
	waitUntil(t, func() bool { return sc.Last() == nil })

	second.push("state:360:20\n")
	waitUntil(t, func() bool {
		res := sc.Last()
		return res != nil && res.RPM == 360 && res.Speed == 20
	})
	if opens.Load() < 2 {
		t.Fatalf("opens = %d, want at least 2", opens.Load())
	}
}

func TestMonitorStopsWhenClosed(t *testing.T) {
	port := newFakePort()
	var opens atomic.Int32

	sc := NewSerialController("/dev/ttyUSB0")
	sc.retryMin = 10 * time.Millisecond
	sc.retryMax = 10 * time.Millisecond
	sc.openPort = func(string) (serialPort, error) {
		opens.Add(1)
		return port, nil
	}
	if err := sc.EnsureReading(); err != nil {
		t.Fatalf("EnsureReading: %v", err)
	}

	select {
	case <-port.waiting:
	case <-time.After(time.Second):
		t.Fatal("read loop did not start")
	}
	if err := sc.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	time.Sleep(50 * time.Millisecond)
	if got := opens.Load(); got != 1 {
		t.Fatalf("opens = %d, want 1 after Close", got)
	}
	if err := sc.Send(NewPayloadReq(20)); err == nil {
		t.Fatal("Send after Close succeeded")
	}
}
