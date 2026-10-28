package internal

import (
	"os"
	"path/filepath"
	"testing"
)

func writeConfig(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write temp config: %v", err)
	}
	return path
}

func testConfig() *Config {
	return &Config{
		FanLevel: []FanLevel{
			{Temp: 20, Fan: 40},
			{Temp: 30, Fan: 70},
			{Temp: 35, Fan: 100},
		},
		Hysteresis: 3,
	}
}

func TestSpeedAt(t *testing.T) {
	cfg := testConfig()
	tests := []struct {
		temp int
		want int
	}{
		{temp: 0, want: 0},
		{temp: 19, want: 0},
		{temp: 20, want: 40},
		{temp: 29, want: 40},
		{temp: 30, want: 70},
		{temp: 34, want: 70},
		{temp: 35, want: 100},
		{temp: 90, want: 100},
	}
	for _, tt := range tests {
		if got := cfg.speedAt(tt.temp); got != tt.want {
			t.Errorf("speedAt(%d) = %d, want %d", tt.temp, got, tt.want)
		}
	}
}

func TestSpeedAtUsesMinFanBelowLowestLevel(t *testing.T) {
	cfg := testConfig()
	cfg.MinFan = 20
	if got := cfg.speedAt(10); got != 20 {
		t.Errorf("speedAt(10) = %d, want 20 (min_fan)", got)
	}
}

func TestFanSpeedForHysteresis(t *testing.T) {
	cfg := testConfig()
	tests := []struct {
		name    string
		temp    int
		current int
		want    int
	}{
		{name: "upshift is immediate", temp: 30, current: 40, want: 70},
		{name: "big jump upshift", temp: 40, current: 40, want: 100},
		{name: "stays put inside the level", temp: 31, current: 70, want: 70},
		{name: "holds just below the threshold", temp: 29, current: 70, want: 70},
		{name: "still holds within hysteresis", temp: 27, current: 70, want: 70},
		{name: "downshifts past hysteresis", temp: 26, current: 70, want: 40},
		{name: "holds at top level", temp: 34, current: 100, want: 100},
		{name: "downshifts from top level", temp: 31, current: 100, want: 70},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := cfg.FanSpeedFor(tt.temp, tt.current); got != tt.want {
				t.Errorf("FanSpeedFor(temp=%d, current=%d) = %d, want %d", tt.temp, tt.current, got, tt.want)
			}
		})
	}
}

func TestFanSpeedForDoesNotFlapAroundThreshold(t *testing.T) {
	cfg := testConfig()
	speed := 70
	for _, temp := range []int{29, 30, 29, 30, 29, 30} {
		speed = cfg.FanSpeedFor(temp, speed)
		if speed != 70 {
			t.Fatalf("speed flapped to %d at temp %d, want a steady 70", speed, temp)
		}
	}
}

func TestLoadConfigSortsLevelsAndFillsDefaults(t *testing.T) {
	path := writeConfig(t, `
fan_level:
  - temp: 35
    fan: 100
  - temp: 20
    fan: 40
  - temp: 30
    fan: 70
serial_port: /dev/ttyUSB1
`)
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	wantTemps := []int{20, 30, 35}
	for i, want := range wantTemps {
		if cfg.FanLevel[i].Temp != want {
			t.Fatalf("FanLevel[%d].Temp = %d, want %d", i, cfg.FanLevel[i].Temp, want)
		}
	}
	if got := cfg.speedAt(30); got != 70 {
		t.Errorf("speedAt(30) = %d, want 70", got)
	}
	if cfg.Interval != defaultInterval || cfg.FailAfter != defaultFailAfter ||
		cfg.FailSpeed != MaxFanSpeed || cfg.ManualHold != defaultManualHold {
		t.Errorf("defaults = interval %s fail_after %s fail_speed %d manual_hold %s",
			cfg.Interval, cfg.FailAfter, cfg.FailSpeed, cfg.ManualHold)
	}
	if cfg.SerialPort != "/dev/ttyUSB1" {
		t.Errorf("serial_port = %s", cfg.SerialPort)
	}
}

func TestLoadConfigKeepsExplicitZeroFailSpeedAndManualHold(t *testing.T) {
	path := writeConfig(t, `
fan_level:
  - temp: 50
    fan: 40
fail_speed: 0
manual_hold: 0s
`)
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.FailSpeed != 0 {
		t.Errorf("fail_speed = %d, want 0", cfg.FailSpeed)
	}
	if cfg.ManualHold != 0 {
		t.Errorf("manual_hold = %s, want 0", cfg.ManualHold)
	}
}

func TestLoadConfigRejectsBadInput(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "empty fan_level", body: "serial_port: /dev/ttyUSB0\n"},
		{name: "fan above percent", body: "fan_level:\n  - temp: 20\n    fan: 150\n"},
		{name: "negative fan", body: "fan_level:\n  - temp: 20\n    fan: -1\n"},
		{name: "min_fan above range", body: "fan_level:\n  - temp: 20\n    fan: 50\nmin_fan: 101\n"},
		{name: "min_fan above first level", body: "fan_level:\n  - temp: 20\n    fan: 30\nmin_fan: 40\n"},
		{name: "negative hysteresis", body: "fan_level:\n  - temp: 20\n    fan: 50\nhysteresis: -2\n"},
		{name: "duplicate temp", body: "fan_level:\n  - temp: 40\n    fan: 30\n  - temp: 40\n    fan: 40\n"},
		{name: "fan decreases as temp rises", body: "fan_level:\n  - temp: 70\n    fan: 30\n  - temp: 40\n    fan: 80\n"},
		{name: "zero interval", body: "fan_level:\n  - temp: 20\n    fan: 50\ninterval: 0s\n"},
		{name: "zero fail_after", body: "fan_level:\n  - temp: 20\n    fan: 50\nfail_after: 0s\n"},
		{name: "negative gpu", body: "fan_level:\n  - temp: 20\n    fan: 50\ngpu: -1\n"},
		{name: "malformed yaml", body: "fan_level: [\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := LoadConfig(writeConfig(t, tt.body)); err == nil {
				t.Error("LoadConfig succeeded, want an error")
			}
		})
	}
}

func TestLoadConfigMissingFile(t *testing.T) {
	if _, err := LoadConfig(filepath.Join(t.TempDir(), "nope.yml")); err == nil {
		t.Error("LoadConfig succeeded on a missing file, want an error")
	}
}

func TestRepoConfigLoads(t *testing.T) {
	cfg, err := LoadConfig(filepath.Join("..", "config.yml"))
	if err != nil {
		t.Fatalf("repo config.yml: %v", err)
	}
	if cfg.FanSpeedFor(44, 0) != cfg.MinFan {
		t.Fatalf("below the first step should use min_fan")
	}
	if cfg.FanSpeedFor(80, 0) != MaxFanSpeed {
		t.Fatalf("a hot GPU should map to full speed, got %d", cfg.FanSpeedFor(80, 0))
	}
}

func TestResolveSerialPort(t *testing.T) {
	tests := []struct {
		name       string
		cfg        *Config
		flagPort   string
		wantResult string
	}{
		{name: "flag wins", cfg: &Config{SerialPort: "/dev/ttyUSB1"}, flagPort: "/dev/ttyACM0", wantResult: "/dev/ttyACM0"},
		{name: "config is used when no flag", cfg: &Config{SerialPort: "/dev/ttyUSB1"}, wantResult: "/dev/ttyUSB1"},
		{name: "default when neither is set", cfg: &Config{}, wantResult: DefaultSerialPort},
		{name: "default when config failed to load", cfg: nil, wantResult: DefaultSerialPort},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.cfg.ResolveSerialPort(tt.flagPort); got != tt.wantResult {
				t.Errorf("ResolveSerialPort(%q) = %q, want %q", tt.flagPort, got, tt.wantResult)
			}
		})
	}
}
