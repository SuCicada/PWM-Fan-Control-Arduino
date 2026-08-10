package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

const nvidiaSMITimeout = 5 * time.Second

// 需要目标机上装有 NVIDIA 驱动自带的 nvidia-smi
func gpuTemps() ([]int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), nvidiaSMITimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "nvidia-smi", "--query-gpu=temperature.gpu", "--format=csv,noheader,nounits")
	out, err := cmd.Output()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("nvidia-smi timed out after %v", nvidiaSMITimeout)
		}
		// Output 失败时 stderr 已经在 ExitError 里，不需要再跑一次 nvidia-smi
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && len(exitErr.Stderr) > 0 {
			return nil, fmt.Errorf("nvidia-smi: %w: %s", err, strings.TrimSpace(string(exitErr.Stderr)))
		}
		return nil, fmt.Errorf("nvidia-smi: %w", err)
	}

	var temps []int
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		v, err := strconv.Atoi(line)
		if err != nil {
			return nil, fmt.Errorf("parse temperature %q: %w", line, err)
		}
		temps = append(temps, v)
	}
	if len(temps) == 0 {
		return nil, errors.New("nvidia-smi reported no GPU temperature")
	}
	return temps, nil
}

func maxGPUTemp() (int, error) {
	temps, err := gpuTemps()
	if err != nil {
		return 0, err
	}
	maxTemp := temps[0]
	for _, t := range temps[1:] {
		maxTemp = max(maxTemp, t)
	}
	return maxTemp, nil
}

func runAutoControl(cfg *Config, sc *SerialController, dryRun bool) error {
	temp, err := maxGPUTemp()
	if err != nil {
		return err
	}

	// 迟滞判断需要知道风扇当前转速，所以先读一次状态
	current, err := sc.ReadStatus(statusTimeout)
	if err != nil {
		return err
	}

	target := cfg.FanSpeedFor(temp, current.Speed)
	log.Printf("max temp %d°C, current speed %d, target speed %d", temp, current.Speed, target)

	if dryRun {
		log.Println("dryrun, nothing sent")
		return nil
	}
	if target == current.Speed {
		return nil
	}
	return sc.SetSpeedVerified(target)
}

// applySpeed 只在转速确实需要变化时才下发，省掉固件一次无谓的 EEPROM 写入。
func applySpeed(sc *SerialController, speed int) error {
	current, err := sc.ReadStatus(statusTimeout)
	if err != nil {
		return err
	}
	if current.Speed == speed {
		log.Printf("fan speed already %d, skip", speed)
		return nil
	}
	return sc.SetSpeedVerified(speed)
}

func main() {
	log.SetFlags(log.Ldate | log.Ltime)
	if err := run(); err != nil {
		log.Printf("error: %v", err)
		os.Exit(1)
	}
}

func run() error {
	var (
		fanSpeed   int
		portFlag   string
		configFile string
		dryRun     bool
		readOnly   bool
		setOnly    bool
	)
	flag.IntVar(&fanSpeed, "fan", -1, "manually set fan speed (0-255); omit to run temperature control")
	flag.StringVar(&portFlag, "port", "", "serial port, overrides serial_port in the config file (default "+defaultSerialPort+")")
	flag.StringVar(&configFile, "config", "config.yml", "config file")
	flag.BoolVar(&dryRun, "dryrun", false, "compute the target speed but do not send it")
	flag.BoolVar(&readOnly, "readonly", false, "read the current fan speed and exit")
	flag.BoolVar(&setOnly, "setonly", false, "send the speed without reading it back to verify")
	flag.Parse()

	if fanSpeed > maxFanSpeed {
		return fmt.Errorf("-fan %d is out of range 0-%d", fanSpeed, maxFanSpeed)
	}

	// 只有温控模式离不开配置文件，手动和只读模式缺配置也应该能用
	needsConfig := !readOnly && fanSpeed < 0
	cfg, err := LoadConfig(configFile)
	if err != nil {
		if needsConfig {
			return err
		}
		log.Printf("warning: %v, continuing without config", err)
	}

	sc := NewSerialController(cfg.ResolveSerialPort(portFlag))
	defer sc.Close()

	switch {
	case readOnly:
		res, err := sc.ReadStatus(statusTimeout)
		if err != nil {
			return err
		}
		fmt.Printf("fan speed: %d, rpm: %d\n", res.Speed, res.RPM)
		return nil

	case fanSpeed >= 0:
		if setOnly {
			return sc.Send(NewPayloadReq(fanSpeed))
		}
		return applySpeed(sc, fanSpeed)

	default:
		return runAutoControl(cfg, sc, dryRun)
	}
}
