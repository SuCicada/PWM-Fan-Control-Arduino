package internal

import (
	"bytes"
	"fmt"
	"log"
	"sync"
	"time"

	"go.bug.st/serial"
)

const (
	baudRate = 9600
	// 单次 Read 的超时，只决定轮询粒度，不决定整体等待时长
	readChunkTimeout = 200 * time.Millisecond
	// 固件每秒播报一次状态，2s 足够抓到至少一帧
	StatusTimeout = 2 * time.Second
	// 指令生效前固件可能还会吐出一两帧旧状态，留足时间等新值出现
	VerifyTimeout = 5 * time.Second
	// USB 掉线后按这个节奏重开。设备长时间不在时逐步拉长，避免空转打开。
	reconnectMin = time.Second
	reconnectMax = 30 * time.Second
)

// serialPort 是读循环实际用到的那一部分，方便测试替换。
type serialPort interface {
	Read(p []byte) (int, error)
	Write(p []byte) (int, error)
	Close() error
}

type SerialController struct {
	portName string
	port     serialPort
	openPort func(name string) (serialPort, error)

	mu      sync.Mutex
	lastRes *PayloadRes

	readOnce sync.Once
	stop     chan struct{}
	stopped  bool

	retryMin time.Duration
	retryMax time.Duration
}

func NewSerialController(portName string) *SerialController {
	if portName == "" {
		portName = DefaultSerialPort
	}
	return &SerialController{
		portName: portName,
		openPort: openSerialPort,
		stop:     make(chan struct{}),
		retryMin: reconnectMin,
		retryMax: reconnectMax,
	}
}

func openSerialPort(name string) (serialPort, error) {
	mode := &serial.Mode{
		BaudRate: baudRate,
		DataBits: 8,
		Parity:   serial.NoParity,
		StopBits: serial.OneStopBit,
	}
	port, err := serial.Open(name, mode)
	if err != nil {
		return nil, err
	}
	if err := port.SetReadTimeout(readChunkTimeout); err != nil {
		port.Close()
		return nil, fmt.Errorf("set read timeout on %s: %w", name, err)
	}
	return port, nil
}

func (s *SerialController) open() (serialPort, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.stopped {
		return nil, fmt.Errorf("serial %s closed", s.portName)
	}
	if s.port != nil {
		return s.port, nil
	}

	port, err := s.openPort(s.portName)
	if err != nil {
		return nil, fmt.Errorf("open serial %s: %w", s.portName, err)
	}
	s.port = port
	return port, nil
}

func (s *SerialController) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.stopped {
		s.stopped = true
		close(s.stop)
	}
	return s.closePortLocked()
}

func (s *SerialController) closePortLocked() error {
	if s.port == nil {
		return nil
	}
	err := s.port.Close()
	s.port = nil
	s.lastRes = nil
	return err
}

// dropPort 扔掉当前句柄。USB 断开后旧 fd 还会留在可读状态，必须关掉再打开。
func (s *SerialController) dropPort() {
	s.mu.Lock()
	defer s.mu.Unlock()
	_ = s.closePortLocked()
}

func (s *SerialController) dropIfSame(port serialPort) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.port != port {
		return
	}
	_ = s.closePortLocked()
}

func (s *SerialController) HandleMessage(res *PayloadRes) {
	// log.Printf("received %s", res)
	s.mu.Lock()
	s.lastRes = res
	s.mu.Unlock()
}

// EnsureReading 启动后台读循环（只一次）。端口断开后循环会自己重开。
func (s *SerialController) EnsureReading() error {
	if _, err := s.open(); err != nil {
		return err
	}
	s.readOnce.Do(func() {
		go s.monitor()
	})
	return nil
}

func (s *SerialController) monitor() {
	delay := s.retryMin
	reconnecting := false
	for {
		if s.stopping() {
			return
		}
		healthy, err := s.readLoop(reconnecting)
		if s.stopping() {
			return
		}
		s.dropPort()
		if healthy {
			delay = s.retryMin
		}
		log.Printf("serial read stopped: %v; retry in %s", err, delay)
		if !s.wait(delay) {
			return
		}
		if !healthy && delay < s.retryMax {
			delay *= 2
			if delay > s.retryMax {
				delay = s.retryMax
			}
		}
		reconnecting = true
	}
}

func (s *SerialController) readLoop(reconnecting bool) (bool, error) {
	port, err := s.open()
	if err != nil {
		return false, err
	}
	if reconnecting {
		log.Printf("serial %s reconnected", s.portName)
	}

	var buf []byte
	chunk := make([]byte, 256)
	healthy := false

	for {
		n, err := port.Read(chunk)
		if err != nil {
			return healthy, fmt.Errorf("read %s: %w", s.portName, err)
		}
		if n == 0 {
			continue
		}
		healthy = true
		buf = append(buf, chunk[:n]...)

		for {
			i := bytes.IndexByte(buf, '\n')
			if i < 0 {
				break
			}
			line := string(buf[:i])
			buf = buf[i+1:]
			if res := DecodeToPayloadRes(line); res != nil {
				s.HandleMessage(res)
			}
		}
	}
}

func (s *SerialController) stopping() bool {
	select {
	case <-s.stop:
		return true
	default:
		return false
	}
}

func (s *SerialController) wait(d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-s.stop:
		return false
	case <-timer.C:
		return true
	}
}

func (s *SerialController) Send(req *PayloadReq) error {
	port, err := s.open()
	if err != nil {
		return err
	}

	line := req.Encode()
	if _, err := port.Write([]byte(line + "\n")); err != nil {
		s.dropIfSame(port)
		return fmt.Errorf("write %s: %w", s.portName, err)
	}
	log.Printf("sent %s", line)
	return nil
}

func (s *SerialController) Last() *PayloadRes {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastRes
}

// WaitStatus 等到一帧合法 state: 或超时。
func (s *SerialController) WaitStatus(timeout time.Duration) (*PayloadRes, error) {
	if err := s.EnsureReading(); err != nil {
		return nil, err
	}

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if res := s.Last(); res != nil {
			return res, nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return nil, fmt.Errorf("no %q status line from %s within %v", STATE_KEY, s.portName, timeout)
}

// GetSpeed 阻塞等到一帧状态。
func (s *SerialController) GetSpeed(timeout time.Duration) (*PayloadRes, error) {
	return s.WaitStatus(timeout)
}

// SetSpeedVerified 下发指令并等固件播报确认新转速生效。
func (s *SerialController) SetSpeedVerified(speed int) error {
	if speed < 0 || speed > MaxFanSpeed {
		return fmt.Errorf("speed %d out of range 0-%d", speed, MaxFanSpeed)
	}
	if err := s.EnsureReading(); err != nil {
		return err
	}

	if cur := s.Last(); cur != nil && cur.Speed == speed {
		log.Printf("fan speed already %d, skip", speed)
		return nil
	}

	if err := s.Send(NewPayloadReq(speed)); err != nil {
		return err
	}

	res, err := s.waitForSpeed(speed, VerifyTimeout)
	if err != nil {
		return err
	}
	log.Printf("verified %s", res)
	return nil
}

func (s *SerialController) waitForSpeed(want int, timeout time.Duration) (*PayloadRes, error) {
	deadline := time.Now().Add(timeout)
	var last *PayloadRes

	for time.Now().Before(deadline) {
		res := s.Last()
		if res != nil {
			last = res
			if res.Speed == want {
				return res, nil
			}
		}
		time.Sleep(50 * time.Millisecond)
	}

	if last == nil {
		return nil, fmt.Errorf("no status line from %s while waiting for speed %d", s.portName, want)
	}
	return nil, fmt.Errorf("verify failed: want speed %d, last reported %d", want, last.Speed)
}
