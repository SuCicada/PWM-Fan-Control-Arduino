// gpu_temp_smi.go
package main

import (
	"bytes"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	FanLevel []struct {
		Temp int `yaml:"temp"`
		Fan  int `yaml:"fan"`
	} `yaml:"fan_level"`
	SerialPort string `yaml:"serial_port"`
}

func getFanSpeedFromTemperature(temperature int) (int, error) {
	var config Config
	cfg, err := os.ReadFile("config.yml")
	if err != nil {
		return 0, err
	}
	err = yaml.Unmarshal(cfg, &config)
	if err != nil {
		return 0, err
	}
	fanSpeed := 0
	for _, item := range config.FanLevel {
		if temperature < item.Temp {
			break
		}
		fanSpeed = item.Fan
	}
	return fanSpeed, nil
}

func gpuTemps() ([]int, error) {
	// 需要目标 Linux 上已安装 NVIDIA 驱动自带的 nvidia-smi
	cmd := exec.Command("nvidia-smi", "--query-gpu=temperature.gpu", "--format=csv,noheader,nounits")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	temps := make([]int, 0, len(lines))
	for _, ln := range lines {
		ln = strings.TrimSpace(ln)
		if ln == "" {
			continue
		}
		v, err := strconv.Atoi(ln)
		if err != nil {
			return nil, fmt.Errorf("parse temp %q: %w", ln, err)
		}
		temps = append(temps, v)
	}
	return temps, nil
}

func runAutoContorl(dryrun bool) {
	ts, err := gpuTemps()
	if err != nil {
		// 打印 nvidia-smi 的stderr方便诊断
		var stderr bytes.Buffer
		cmd := exec.Command("nvidia-smi")
		cmd.Stderr = &stderr
		_ = cmd.Run()
		fmt.Printf("read error: %v\nnvidia-smi stderr: %s\n", err, stderr.String())
		return
	}

	maxTemp := 0
	for _, t := range ts {
		maxTemp = max(maxTemp, t)
	}

	// calculate fan speed from temperature
	shouldFanSpeed, err := getFanSpeedFromTemperature(maxTemp)
	if err != nil {
		fmt.Printf("get fan speed error: %v\n", err)
		return
	}
	fmt.Printf("max temp: %d°C, should set fan speed: %d\n", maxTemp, shouldFanSpeed)

	if !dryrun {
		log.Println("autoset mode, set fan speed to", shouldFanSpeed)
		doCheckAndSendPayload(shouldFanSpeed)
	}
}

func doCheckAndSendPayload(fanSpeed int) {
	serialController := &SerialController{
		portName: serialPortName,
	}
	err := serialController.doCheckAndSendPayload(PayloadReq{
		Speed: fanSpeed,
	})
	if err != nil {
		fmt.Printf("send payload error: %v\n", err)
	}
}

func setFanSpeed(fanSpeed int) {
	serialController := &SerialController{
		portName: serialPortName,
	}
	payload := PayloadReq{
		Speed: fanSpeed,
	}
	err := serialController.sendToSerial(payload.Encode() + "\n")
	if err != nil {
		fmt.Printf("send payload error: %v\n", err)
	}
}
func readFanSpeed() int {
	serialController := &SerialController{
		portName: serialPortName,
	}
	payloadRes, err := serialController.readFanFromSerial()
	if err != nil {
		fmt.Printf("read fan speed error: %v\n", err)
		return -1
	}
	return payloadRes.Speed
}

const DEFAULT_SERIAL_PORT = "/dev/ttyUSB0"

var serialPortName string = DEFAULT_SERIAL_PORT

func main() {

	var fanSpeed int
	// var autosetEnable bool
	var dryRun bool
	var readOnly bool
	var setOnly bool
	flag.IntVar(&fanSpeed, "fan", -1, "manual set fan speed")
	flag.StringVar(&serialPortName, "port", DEFAULT_SERIAL_PORT, "serial port")
	// flag.BoolVar(&autosetEnable, "autoset", false, "set mode")
	flag.BoolVar(&dryRun, "dryrun", false, "dry run")
	flag.BoolVar(&readOnly, "readonly", false, "read only")
	flag.BoolVar(&setOnly, "setonly", false, "set only, no check and send payload")
	flag.Parse()

	// ------------------------------------------------
	if readOnly {
		fanSpeed := readFanSpeed()
		fmt.Printf("current fan speed: %d\n", fanSpeed)
		return
	}

	if fanSpeed >= 0 {
		if setOnly {
			doCheckAndSendPayload(fanSpeed)
		} else {
			setFanSpeed(fanSpeed)
		}
	} else {
		runAutoContorl(dryRun)
	}

	// // run()
	// serialController := &SerialController{
	// 	portName: SERIAL_PORT,
	// }
	// err := serialController.sendPayload(PayloadReq{
	// 	Speed: 10,
	// })
}
