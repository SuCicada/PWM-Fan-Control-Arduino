package main

import (
	"fmt"
	"math/rand"
	"strings"

	"github.com/sigurn/crc8"
	"gopkg.in/cstockton/go-conv.v1"
)

const KEY = "fanpwm:"

type PayloadReq struct {
	Speed int
	Seq   int
	CRC   string
}

func NewPayload(speed, seq int) *PayloadReq {
	return &PayloadReq{
		Speed: speed,
		Seq:   seq,
	}
}

func (p *PayloadReq) Encode() string {

	if p.Seq == 0 {
		p.Seq = rand.Intn(100) + 1
	}

	data := fmt.Sprintf("%s%d:%d", KEY, p.Speed, p.Seq)
	table := crc8.MakeTable(crc8.CRC8)
	crc := crc8.Checksum([]byte(data), table)
	p.CRC = fmt.Sprintf("%02x", crc)
	return fmt.Sprintf("%s%d:%d:%s", KEY, p.Speed, p.Seq, p.CRC)
}

//  ================================

type PayloadRes struct {
	RPM   int
	Speed int
}

func DecodeToPayloadRes(str string) *PayloadRes {
	if !strings.HasPrefix(str, KEY) {
		return nil
	}
	str = strings.TrimSpace(str)
	str = strings.TrimPrefix(str, KEY)
	arr := strings.Split(str, ":")
	if len(arr) != 2 {
		return nil
	}
	// log.Println("[debug] arr[1]", arr[1])
	// log.Println("[debug] conv.Int(arr[1])", conv.Int(arr[1]))
	return &PayloadRes{
		RPM:   conv.Int(arr[0]),
		Speed: conv.Int(arr[1]),
	}
}

func (p *PayloadRes) String() string {
	return fmt.Sprintf("RPM: %d, Speed: %d", p.RPM, p.Speed)
}
