package main

import (
	"fmt"
	"os"
	"sort"

	"gopkg.in/yaml.v3"
)

const (
	defaultSerialPort = "/dev/ttyUSB0"
	maxFanSpeed       = 255
)

type FanLevel struct {
	Temp int `yaml:"temp"`
	Fan  int `yaml:"fan"`
}

type Config struct {
	FanLevel   []FanLevel `yaml:"fan_level"`
	SerialPort string     `yaml:"serial_port"`
	// MinFan 是温度低于最低档阈值时使用的转速，用来避免把风扇整个停掉
	MinFan int `yaml:"min_fan"`
	// Hysteresis 是降档额外需要满足的温度余量（℃），防止温度在阈值附近抖动时来回跳档
	Hysteresis int `yaml:"hysteresis"`
}

func LoadConfig(path string) (*Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var c Config
	if err := yaml.Unmarshal(raw, &c); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}
	if err := c.validate(path); err != nil {
		return nil, err
	}

	// 允许配置文件里乱序书写，加载时排好，speedAt 才能依赖顺序提前 break
	sort.Slice(c.FanLevel, func(i, j int) bool { return c.FanLevel[i].Temp < c.FanLevel[j].Temp })
	return &c, nil
}

func (c *Config) validate(path string) error {
	if len(c.FanLevel) == 0 {
		return fmt.Errorf("config %s: fan_level is empty", path)
	}
	for _, l := range c.FanLevel {
		if l.Fan < 0 || l.Fan > maxFanSpeed {
			return fmt.Errorf("config %s: fan %d at temp %d is out of range 0-%d", path, l.Fan, l.Temp, maxFanSpeed)
		}
	}
	if c.MinFan < 0 || c.MinFan > maxFanSpeed {
		return fmt.Errorf("config %s: min_fan %d is out of range 0-%d", path, c.MinFan, maxFanSpeed)
	}
	if c.Hysteresis < 0 {
		return fmt.Errorf("config %s: hysteresis %d must not be negative", path, c.Hysteresis)
	}
	return nil
}

// 优先级：命令行 -port，其次配置文件的 serial_port，最后内置默认值
func (c *Config) ResolveSerialPort(flagPort string) string {
	if flagPort != "" {
		return flagPort
	}
	if c != nil && c.SerialPort != "" {
		return c.SerialPort
	}
	return defaultSerialPort
}

// speedAt 返回温度落在哪一档，低于最低档阈值时回落到 MinFan。
// 要求 FanLevel 已按 Temp 升序排列。
func (c *Config) speedAt(temp int) int {
	speed := c.MinFan
	for _, l := range c.FanLevel {
		if temp < l.Temp {
			break
		}
		speed = l.Fan
	}
	return speed
}

// FanSpeedFor 在 speedAt 之上加了单向迟滞：升温立刻跟进，降温则要求温度比档位
// 阈值再低 Hysteresis 度才认账，否则维持当前转速。
func (c *Config) FanSpeedFor(temp, currentSpeed int) int {
	target := c.speedAt(temp)
	if target >= currentSpeed {
		return target
	}
	if relaxed := c.speedAt(temp + c.Hysteresis); relaxed < currentSpeed {
		return relaxed
	}
	return currentSpeed
}
