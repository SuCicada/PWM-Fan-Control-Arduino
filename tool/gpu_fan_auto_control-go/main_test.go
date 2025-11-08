package main

import (
	"fmt"
	"testing"
)

func TestCalcCrc(t *testing.T) {
	payload := PayloadReq{
		Speed: 255,
		Seq:   3,
	}
	str := payload.Encode()
	fmt.Println(str)
}

func TestDecodeToPayloadRes(t *testing.T) {
	str := "fanpwm:15720:10 "
	payloadRes := DecodeToPayloadRes(str)
	fmt.Println(payloadRes)
}

func TestInt(t *testing.T) {

}
