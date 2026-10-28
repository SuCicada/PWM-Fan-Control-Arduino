package internal

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

const gpuTempTimeout = 2 * time.Second

func queryGPUTemp(ctx context.Context, gpu int) (int, error) {
	ctx, cancel := context.WithTimeout(ctx, gpuTempTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "nvidia-smi",
		"--id="+strconv.Itoa(gpu),
		"--query-gpu=temperature.gpu",
		"--format=csv,noheader,nounits",
	)
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && len(exitErr.Stderr) > 0 {
			return 0, fmt.Errorf("nvidia-smi: %s", strings.TrimSpace(string(exitErr.Stderr)))
		}
		return 0, fmt.Errorf("nvidia-smi: %w", err)
	}
	return parseGPUTemp(out)
}

func parseGPUTemp(out []byte) (int, error) {
	line := strings.TrimSpace(string(out))
	n, err := strconv.Atoi(line)
	if err != nil {
		return 0, fmt.Errorf("nvidia-smi temperature %q: %w", line, err)
	}
	if n < 0 || n > maxGPUTemp {
		return 0, fmt.Errorf("nvidia-smi temperature %d is out of range 0-%d", n, maxGPUTemp)
	}
	return n, nil
}

// fanBoard 是回路要碰的串口面。*SerialController 满足它，测试用假板子替换。
type fanBoard interface {
	SetSpeedVerified(speed int) error
	GetSpeed(timeout time.Duration) (*PayloadRes, error)
}

// Controller 每 Interval 读一次核心温度，查表后只在转速变化时写串口。
type Controller struct {
	cfg      *Config
	board    fanBoard
	now      func() time.Time
	readTemp func(context.Context) (int, error)

	mu          sync.Mutex
	current     int
	haveCurrent bool
	seedLogged  bool
	manualUntil time.Time
	failSince   time.Time
	failing     bool
	lastTemp    int
	haveTemp    bool
}

func NewController(cfg *Config, sc *SerialController) *Controller {
	gpu := cfg.GPU
	return &Controller{
		cfg:   cfg,
		board: sc,
		now:   time.Now,
		readTemp: func(ctx context.Context) (int, error) {
			return queryGPUTemp(ctx, gpu)
		},
	}
}

// LastTemp 返回最近一次成功的核心温度。还没读到时 ok 为 false。
func (c *Controller) LastTemp() (int, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.lastTemp, c.haveTemp
}

// SetManual 立刻写转速，并在 ManualHold 之内不让自动回路覆盖。
func (c *Controller) SetManual(speed int) error {
	if speed < 0 || speed > MaxFanSpeed {
		return fmt.Errorf("speed %d out of range 0-%d", speed, MaxFanSpeed)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.board.SetSpeedVerified(speed); err != nil {
		return err
	}
	c.current = speed
	c.haveCurrent = true
	if c.cfg.ManualHold > 0 {
		c.manualUntil = c.clock().Add(c.cfg.ManualHold)
		log.Printf("manual fan %d%%, auto resumes after %s", speed, c.cfg.ManualHold)
	}
	return nil
}

// Run 一直跑到 ctx 取消。机器上没有 nvidia-smi 时直接返回，HTTP 手动接口仍可用。
func (c *Controller) Run(ctx context.Context) {
	if _, err := exec.LookPath("nvidia-smi"); err != nil {
		log.Printf("nvidia-smi not found, temperature loop disabled: %v", err)
		return
	}
	log.Printf("auto fan every %s, fail-safe %d%% after %s, manual hold %s",
		c.cfg.Interval, c.cfg.FailSpeed, c.cfg.FailAfter, c.cfg.ManualHold)

	timer := time.NewTimer(0)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			c.tick(ctx)
			timer.Reset(c.cfg.Interval)
		}
	}
}

func (c *Controller) tick(ctx context.Context) {
	temp, err := c.readTemp(ctx)

	c.mu.Lock()
	defer c.mu.Unlock()

	if err == nil {
		c.lastTemp = temp
		c.haveTemp = true
		c.failSince = time.Time{}
		c.failing = false
	}
	if !c.manualUntil.IsZero() && c.clock().Before(c.manualUntil) {
		return
	}
	if err != nil {
		c.noteFailureLocked(err)
		return
	}

	if !c.haveCurrent && !c.seedCurrentLocked() {
		return
	}
	target := c.cfg.FanSpeedFor(temp, c.current)
	if target != c.current {
		log.Printf("gpu %d°C -> fan %d%%", temp, target)
	}
	c.applyLocked(target)
}

func (c *Controller) noteFailureLocked(err error) {
	now := c.clock()
	if c.failSince.IsZero() {
		c.failSince = now
		log.Printf("gpu temperature read failed: %v", err)
	}
	if now.Sub(c.failSince) < c.cfg.FailAfter {
		return
	}
	if !c.failing {
		log.Printf("gpu temperature unread for %s, set fan %d%%", c.cfg.FailAfter, c.cfg.FailSpeed)
		c.failing = true
	}
	c.applyLocked(c.cfg.FailSpeed)
}

func (c *Controller) seedCurrentLocked() bool {
	res, err := c.board.GetSpeed(StatusTimeout)
	if err != nil {
		if !c.seedLogged {
			log.Printf("fan status unavailable, hold speed until the board reports: %v", err)
			c.seedLogged = true
		}
		return false
	}
	c.current = res.Speed
	c.haveCurrent = true
	c.seedLogged = false
	log.Printf("fan currently %d%%", res.Speed)
	return true
}

func (c *Controller) applyLocked(speed int) {
	if c.haveCurrent && c.current == speed {
		return
	}
	if err := c.board.SetSpeedVerified(speed); err != nil {
		log.Printf("set fan %d%%: %v", speed, err)
		return
	}
	c.current = speed
	c.haveCurrent = true
}

func (c *Controller) clock() time.Time {
	if c.now != nil {
		return c.now()
	}
	return time.Now()
}
