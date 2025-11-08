package main

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"go.bug.st/serial"
)

type SerialController struct {
	portClient serial.Port
	portName   string
}

func (s *SerialController) getSerial() (serial.Port, error) {
	if s.portClient != nil {
		return s.portClient, nil
	}

	mode := &serial.Mode{
		BaudRate: 9600, // 常见有 9600/19200/57600/115200 …
		// 需要特殊设置再放开
		DataBits: 8,
		Parity:   serial.NoParity,
		StopBits: serial.OneStopBit,
	}
	serialPort, err := serial.Open(s.portName, mode)
	if err != nil {
		log.Fatalf("open serial error: %v", err)
		return nil, err
	}
	serialPort.SetReadTimeout(2 * time.Second)

	// var close = func() error {
	// 	return serialPort.Close()
	// }
	s.portClient = serialPort
	return serialPort, nil
}

var currentFanSpeed int = 0
var start = false

func (s *SerialController) readFanFromSerial() (*PayloadRes, error) {
	s.getSerial()

	// serialPort, err := getSerial()
	// if err != nil {
	// 	return nil, err
	// }
	// defer serialPort.Close()

	// 先读取 5 秒
	var buf bytes.Buffer
	deadline := time.Now().Add(3 * time.Second)
	tmp := make([]byte, 1024)

	for time.Now().Before(deadline) {
		// 非阻塞/短超时读取：如果没设 ReadTimeout，这里可能阻塞较久
		n, err := s.portClient.Read(tmp)
		if err != nil {
			// 大多数情况下，临时超时可忽略继续读；致命错误才退出
			log.Printf("read: %v", err)
			continue
		}
		if n > 0 {
			buf.Write(tmp[:n])
		}
	}

	var payloadRes *PayloadRes
	log.Println("read: ")
	res := buf.String()
	res = strings.TrimSpace(res)

	const startFlag = "setup start"
	if strings.Contains(res, startFlag) {
		i := strings.Index(res, startFlag)
		i = max(i-10, 0)
		res = res[i:]
	}

	fmt.Println("<--------------------------------")
	fmt.Println(res)
	fmt.Println(">--------------------------------")

	arr := strings.Split(res, "\n")
	for i := len(arr) - 1; i >= 0; i-- {
		item := arr[i]
		if strings.HasPrefix(item, KEY) {
			log.Println("decode to payload res", item)
			payloadRes = DecodeToPayloadRes(item)
			if payloadRes == nil {
				continue
			}
			log.Println("read fan from serial", payloadRes)
			currentFanSpeed = payloadRes.Speed
			break
		}
	}
	return payloadRes, nil
}

func (s *SerialController) sendToSerial(data string) error {
	s.getSerial()

	_, err := s.portClient.Write([]byte(data))
	if err != nil {
		errmsg := fmt.Sprintf("send to serial error: %v", err)
		log.Println(errmsg)
		return errors.New(errmsg)
	}
	return nil
}

// func sendPayload1(payloadReq PayloadReq) error {
// 	// go readFanFromSerial()
// 	time.Sleep(3 * time.Second)
// 	log.Println("[1] read fan from serial", currentFanSpeed)

// 	// sendToSerial(payloadReq.Encode() + "\n")
// 	log.Println("[2] send payload req", payloadReq.Encode())
// 	time.Sleep(3 * time.Second)
// 	log.Println("[3] read fan from serial", currentFanSpeed)
// 	return nil
// }

func (s *SerialController) doCheckAndSendPayload(payloadReq PayloadReq) error {
	// ------------------------------------------------
	// read fan from serial
	serialPort, err := s.getSerial()
	if err != nil {
		return err
	}
	defer serialPort.Close()

	var preFanSpeed int
	{
		payloadRes, err := s.readFanFromSerial()
		if err != nil {
			errmsg := fmt.Sprintf("read fan from serial error: %v", err)
			log.Println(errmsg)
			return errors.New(errmsg)
		}
		log.Println("[1] read fan from serial", payloadRes)
		preFanSpeed = payloadRes.Speed
	}

	// ------------------------------------------------
	// send payload req
	{
		if preFanSpeed == payloadReq.Speed {
			log.Println("preFanSpeed == payloadReq.Speed, skip")
			return nil
		}
		log.Println("[2] send payload req", payloadReq.Encode())
		err := s.sendToSerial(payloadReq.Encode() + "\n")
		if err != nil {
			errmsg := fmt.Sprintf("send payload req error: %v", err)
			log.Println(errmsg)
			return errors.New(errmsg)
		}
		time.Sleep(2 * time.Second)
	}
	// ------------------------------------------------
	// read fan from serial
	{
		payloadRes, err := s.readFanFromSerial()
		if err != nil {
			errmsg := fmt.Sprintf("read fan from serial error: %v", err)
			log.Println(errmsg)
			return errors.New(errmsg)
		}
		log.Println("[3] read fan from serial", payloadRes)
		if payloadRes.Speed != payloadReq.Speed {
			errmsg := fmt.Sprintf("send payload check error: expected: %d, actual: %d", payloadReq.Speed, payloadRes.Speed)
			log.Println(errmsg)
			return errors.New(errmsg)
		}
	}

	log.Println("sendPayload success")
	return nil
	// serial.Write([]byte(payload))
}
