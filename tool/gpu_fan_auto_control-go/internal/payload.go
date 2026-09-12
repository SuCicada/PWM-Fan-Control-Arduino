package internal

import (
	"fmt"
	"math/rand"
	"strconv"
	"strings"

	"github.com/sigurn/crc8"
)

const (
	CMD_KEY   = "cmd:"
	STATE_KEY = "state:"
)

// 参数与固件侧 RobTillaart/CRC 的 calcCRC8 默认值一致：
// poly 0x07 / init 0x00 / xorOut 0x00 / 不反转
var crcTable = crc8.MakeTable(crc8.CRC8)

type PayloadReq struct {
	Speed int
	Seq   int
}

// 固件按整行内容去重，seq 的作用是让「同一转速的两次独立指令」不被误判成重复，
// 所以取随机值就够了。
func NewPayloadReq(speed int) *PayloadReq {
	return &PayloadReq{Speed: speed, Seq: rand.Intn(MaxFanSpeed) + 1}
}

func (p *PayloadReq) Encode() string {
	body := fmt.Sprintf("%s%d:%d", CMD_KEY, p.Speed, p.Seq)
	return fmt.Sprintf("%s:%02x", body, crc8.Checksum([]byte(body), crcTable))
}

type PayloadRes struct {
	RPM   int
	Speed int
}

// DecodeToPayloadRes 解析固件的状态行。格式不符或数值非法时返回 nil，调用方
// 据此跳过启动横幅和调试输出。
func DecodeToPayloadRes(line string) *PayloadRes {
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, STATE_KEY) {
		return nil
	}

	parts := strings.Split(strings.TrimPrefix(line, STATE_KEY), ":")
	if len(parts) != 2 {
		return nil
	}
	rpm, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil || rpm < 0 {
		return nil
	}
	speed, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil || speed < 0 || speed > MaxFanSpeed {
		return nil
	}
	return &PayloadRes{RPM: rpm, Speed: speed}
}

func (p *PayloadRes) String() string {
	return fmt.Sprintf("RPM: %d, Speed: %d", p.RPM, p.Speed)
}
