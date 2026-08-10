package main

import (
	"bytes"
	"fmt"
	"log"
	"time"

	"go.bug.st/serial"
)

const (
	baudRate = 9600
	// 单次 Read 的超时，只决定轮询粒度，不决定整体等待时长
	readChunkTimeout = 200 * time.Millisecond
	// 固件每秒播报一次状态，2s 足够抓到至少一帧
	statusTimeout = 2 * time.Second
	// 指令生效前固件可能还会吐出一两帧旧状态，留足时间等新值出现
	verifyTimeout = 5 * time.Second
)

type SerialController struct {
	portName string
	port     serial.Port
}

func NewSerialController(portName string) *SerialController {
	return &SerialController{portName: portName}
}

func (s *SerialController) open() (serial.Port, error) {
	if s.port != nil {
		return s.port, nil
	}

	mode := &serial.Mode{
		BaudRate: baudRate,
		DataBits: 8,
		Parity:   serial.NoParity,
		StopBits: serial.OneStopBit,
	}
	port, err := serial.Open(s.portName, mode)
	if err != nil {
		return nil, fmt.Errorf("open serial %s: %w", s.portName, err)
	}
	if err := port.SetReadTimeout(readChunkTimeout); err != nil {
		port.Close()
		return nil, fmt.Errorf("set read timeout on %s: %w", s.portName, err)
	}

	s.port = port
	return port, nil
}

func (s *SerialController) Close() error {
	if s.port == nil {
		return nil
	}
	err := s.port.Close()
	s.port = nil
	return err
}

// ReadStatus 按行读取，扫到第一条合法状态行就立即返回，不会白等满 timeout。
// 启动横幅和调试输出会被 DecodeToPayloadRes 过滤掉；始终没等到就返回错误，
// 不会像以前那样返回 (nil, nil) 让调用方解引用空指针。
func (s *SerialController) ReadStatus(timeout time.Duration) (*PayloadRes, error) {
	port, err := s.open()
	if err != nil {
		return nil, err
	}

	deadline := time.Now().Add(timeout)
	var buf []byte
	chunk := make([]byte, 256)

	for time.Now().Before(deadline) {
		n, err := port.Read(chunk)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", s.portName, err)
		}
		if n == 0 {
			continue
		}
		buf = append(buf, chunk[:n]...)

		// 只处理已经完整收到的行，末尾那半行留到下一轮
		for {
			i := bytes.IndexByte(buf, '\n')
			if i < 0 {
				break
			}
			line := string(buf[:i])
			buf = buf[i+1:]
			if res := DecodeToPayloadRes(line); res != nil {
				return res, nil
			}
		}
	}
	return nil, fmt.Errorf("no %q status line from %s within %v", KEY, s.portName, timeout)
}

func (s *SerialController) Send(req *PayloadReq) error {
	port, err := s.open()
	if err != nil {
		return err
	}

	line := req.Encode()
	if _, err := port.Write([]byte(line + "\n")); err != nil {
		return fmt.Errorf("write %s: %w", s.portName, err)
	}
	log.Printf("sent %s", line)
	return nil
}

// SetSpeedVerified 下发指令并等固件播报的状态确认新转速真的生效了。
func (s *SerialController) SetSpeedVerified(speed int) error {
	if err := s.Send(NewPayloadReq(speed)); err != nil {
		return err
	}

	res, err := s.waitForSpeed(speed, verifyTimeout)
	if err != nil {
		return err
	}
	log.Printf("verified %s", res)
	return nil
}

// waitForSpeed 持续读状态行直到转速变成 want，借此跳过指令生效前发出的旧帧。
func (s *SerialController) waitForSpeed(want int, timeout time.Duration) (*PayloadRes, error) {
	deadline := time.Now().Add(timeout)
	var last *PayloadRes

	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			if last == nil {
				return nil, fmt.Errorf("no status line from %s while waiting for speed %d", s.portName, want)
			}
			return nil, fmt.Errorf("verify failed: want speed %d, last reported %d", want, last.Speed)
		}

		res, err := s.ReadStatus(remaining)
		if err != nil {
			if last != nil {
				return nil, fmt.Errorf("verify failed: want speed %d, last reported %d: %w", want, last.Speed, err)
			}
			return nil, err
		}
		if res.Speed == want {
			return res, nil
		}
		last = res
	}
}
