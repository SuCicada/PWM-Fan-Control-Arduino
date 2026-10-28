package internal

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestParseGPUTemp(t *testing.T) {
	n, err := parseGPUTemp([]byte("42\n"))
	if err != nil || n != 42 {
		t.Fatalf("parseGPUTemp(42) = %d, %v", n, err)
	}
	if _, err := parseGPUTemp([]byte("N/A\n")); err == nil {
		t.Fatal("parseGPUTemp(N/A) succeeded, want an error")
	}
	if _, err := parseGPUTemp([]byte("200\n")); err == nil {
		t.Fatal("parseGPUTemp(200) succeeded, want an error")
	}
}

type fakeBoard struct {
	speed  int
	has    bool
	getErr error
	setErr error
	sets   []int
}

func (f *fakeBoard) SetSpeedVerified(speed int) error {
	if f.setErr != nil {
		return f.setErr
	}
	f.sets = append(f.sets, speed)
	f.speed = speed
	f.has = true
	return nil
}

func (f *fakeBoard) GetSpeed(time.Duration) (*PayloadRes, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	if !f.has {
		return nil, errors.New("no status")
	}
	return &PayloadRes{RPM: 1000, Speed: f.speed}, nil
}

func newTestController(board *fakeBoard, temp int, tempErr error) (*Controller, *time.Time) {
	now := time.Now()
	c := &Controller{
		cfg: &Config{
			FanLevel:   []FanLevel{{Temp: 30, Fan: 40}, {Temp: 60, Fan: 80}},
			MinFan:     20,
			Hysteresis: 3,
			Interval:   time.Second,
			FailAfter:  15 * time.Second,
			FailSpeed:  100,
			ManualHold: time.Minute,
		},
		board: board,
		now:   func() time.Time { return now },
		readTemp: func(context.Context) (int, error) {
			return temp, tempErr
		},
	}
	return c, &now
}

func TestTickSetsSpeedFromTemp(t *testing.T) {
	board := &fakeBoard{speed: 20, has: true}
	c, _ := newTestController(board, 60, nil)
	c.current = 20
	c.haveCurrent = true

	c.tick(context.Background())

	if board.speed != 80 || len(board.sets) != 1 {
		t.Fatalf("sets = %v, want [80]", board.sets)
	}
}

func TestTickSkipsUnchangedSpeed(t *testing.T) {
	board := &fakeBoard{speed: 80, has: true}
	c, _ := newTestController(board, 60, nil)
	c.current = 80
	c.haveCurrent = true

	c.tick(context.Background())

	if len(board.sets) != 0 {
		t.Fatalf("sets = %v, want none", board.sets)
	}
}

func TestTickHoldsThenFailsSafe(t *testing.T) {
	board := &fakeBoard{speed: 20, has: true}
	readErr := errors.New("no gpu")
	c, now := newTestController(board, 0, readErr)
	c.current = 20
	c.haveCurrent = true

	c.tick(context.Background())
	if len(board.sets) != 0 {
		t.Fatalf("first failure changed speed to %v", board.sets)
	}

	*now = now.Add(15 * time.Second)
	c.tick(context.Background())
	if board.speed != 100 {
		t.Fatalf("speed = %d, want fail-safe 100", board.speed)
	}
}

func TestTickRetriesAfterSetError(t *testing.T) {
	board := &fakeBoard{speed: 20, has: true, setErr: errors.New("busy")}
	c, _ := newTestController(board, 60, nil)
	c.current = 20
	c.haveCurrent = true

	c.tick(context.Background())
	if c.current != 20 {
		t.Fatalf("current = %d after a failed set, want 20", c.current)
	}

	board.setErr = nil
	c.tick(context.Background())
	if c.current != 80 || board.speed != 80 {
		t.Fatalf("retry speed = %d, current = %d, want 80", board.speed, c.current)
	}
}

func TestTickRespectsManualHold(t *testing.T) {
	board := &fakeBoard{speed: 20, has: true}
	c, now := newTestController(board, 60, nil)
	c.readTemp = func(context.Context) (int, error) { return 60, nil }

	if err := c.SetManual(30); err != nil {
		t.Fatalf("SetManual: %v", err)
	}
	if board.speed != 30 {
		t.Fatalf("manual speed = %d, want 30", board.speed)
	}

	c.tick(context.Background())
	if board.speed != 30 {
		t.Fatalf("auto overwrote manual speed with %d", board.speed)
	}

	*now = now.Add(c.cfg.ManualHold)
	c.tick(context.Background())
	if board.speed != 80 {
		t.Fatalf("speed after hold = %d, want 80", board.speed)
	}
}

func TestTickDoesNotCommandBeforeBoardStatus(t *testing.T) {
	board := &fakeBoard{getErr: errors.New("serial down")}
	c, _ := newTestController(board, 60, nil)

	c.tick(context.Background())
	if len(board.sets) != 0 {
		t.Fatalf("commanded %v without a board status", board.sets)
	}
}

func TestManualHoldIgnoresTempReadFailure(t *testing.T) {
	board := &fakeBoard{speed: 20, has: true}
	c, now := newTestController(board, 0, errors.New("no gpu"))
	if err := c.SetManual(30); err != nil {
		t.Fatalf("SetManual: %v", err)
	}

	*now = now.Add(c.cfg.FailAfter)
	c.tick(context.Background())
	if board.speed != 30 {
		t.Fatalf("fail-safe overwrote manual speed with %d", board.speed)
	}
}
