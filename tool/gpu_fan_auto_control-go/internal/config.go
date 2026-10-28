package internal

import (
	"fmt"
	"os"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
	"gopkg.in/yaml.v3"
)

const (
	defaultInterval   = time.Second
	defaultFailAfter  = 15 * time.Second
	defaultManualHold = 10 * time.Minute
	// 驱动报出的核心温度不可能到这个数。更高的值当成读数损坏，不拿去查表。
	maxGPUTemp = 150
)

type FanLevel struct {
	Temp int `yaml:"temp" validate:"gte=0,lte=150"`
	Fan  int `yaml:"fan" validate:"gte=0,lte=100"`
}

type Config struct {
	FanLevel   []FanLevel    `yaml:"fan_level" validate:"required,dive"`
	SerialPort string        `yaml:"serial_port" validate:"required"`
	MinFan     int           `yaml:"min_fan" validate:"gte=0,lte=100"`
	Hysteresis int           `yaml:"hysteresis" validate:"gte=0"`
	Interval   time.Duration `yaml:"interval" validate:"gt=0"`
	FailAfter  time.Duration `yaml:"fail_after" validate:"gt=0"`
	FailSpeed  int           `yaml:"fail_speed" validate:"gte=0,lte=100"`
	ManualHold time.Duration `yaml:"manual_hold" validate:"gte=0"`
	GPU        int           `yaml:"gpu" validate:"gte=0"`
}

// 报错时用 yaml 名字，和配置文件里的字段对得上。lte=100 / lte=150 对应 MaxFanSpeed / maxGPUTemp。
var configValidator = newConfigValidator()

func newConfigValidator() *validator.Validate {
	v := validator.New()
	v.RegisterTagNameFunc(func(fld reflect.StructField) string {
		name := strings.SplitN(fld.Tag.Get("yaml"), ",", 2)[0]
		if name == "" || name == "-" {
			return fld.Name
		}
		return name
	})
	return v
}

func LoadConfig(path string) (*Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	// 缺省字段保持这些值。yaml 里显式写成 0 会覆盖掉，校验再决定收不收。
	c := &Config{
		SerialPort: DefaultSerialPort,
		Interval:   defaultInterval,
		FailAfter:  defaultFailAfter,
		FailSpeed:  MaxFanSpeed,
		ManualHold: defaultManualHold,
	}
	if err := yaml.Unmarshal(raw, c); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}
	sort.Slice(c.FanLevel, func(i, j int) bool { return c.FanLevel[i].Temp < c.FanLevel[j].Temp })
	if err := c.validate(path); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *Config) validate(path string) error {
	if err := configValidator.Struct(c); err != nil {
		return fmt.Errorf("config %s: %w", path, err)
	}
	// 相邻档要一起看：温度不能重复，转速必须随温度往上走，min_fan 不能高于第一档。
	first := c.FanLevel[0]
	if c.MinFan > first.Fan {
		return fmt.Errorf("config %s: min_fan %d is higher than fan %d at %d°C", path, c.MinFan, first.Fan, first.Temp)
	}
	for i := 1; i < len(c.FanLevel); i++ {
		prev, l := c.FanLevel[i-1], c.FanLevel[i]
		if l.Temp == prev.Temp {
			return fmt.Errorf("config %s: duplicate temp %d", path, l.Temp)
		}
		if l.Fan < prev.Fan {
			return fmt.Errorf("config %s: fan %d at %d°C is lower than fan %d at %d°C", path, l.Fan, l.Temp, prev.Fan, prev.Temp)
		}
	}
	return nil
}

// 命令行 -port 优先，其次配置文件，最后内置默认值。
func (c *Config) ResolveSerialPort(flagPort string) string {
	if flagPort != "" {
		return flagPort
	}
	if c != nil && c.SerialPort != "" {
		return c.SerialPort
	}
	return DefaultSerialPort
}

// speedAt 返回温度落在哪一档。低于最低档阈值时用 MinFan。
// FanLevel 必须已按 Temp 升序排列。
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

// FanSpeedFor 在查表之上加单向迟滞：升温立刻跟进，降温要再低 Hysteresis 度才降档。
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
