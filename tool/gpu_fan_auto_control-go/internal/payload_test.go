package internal

import (
	"testing"

	"github.com/sigurn/crc8"
)

// 固件用的是 RobTillaart/CRC 的 calcCRC8 默认参数。这里用 CRC-8 的标准检验值把
// Go 侧的表钉死，两边一旦选错算法这个测试会先炸，而不是等到串口上校验失败。
func TestCRC8TableMatchesFirmware(t *testing.T) {
	got := crc8.Checksum([]byte("123456789"), crcTable)
	if got != 0xF4 {
		t.Fatalf("crc8 check value = %#x, want 0xf4", got)
	}
}

func TestPayloadReqEncode(t *testing.T) {
	tests := []struct {
		name  string
		speed int
		seq   int
		want  string
	}{
		{"max speed", 100, 3, "cmd:100:3:" + crcSuffix(t, "cmd:100:3")},
		{"zero speed", 0, 1, "cmd:0:1:" + crcSuffix(t, "cmd:0:1")},
		{"mid speed", 50, 42, "cmd:50:42:" + crcSuffix(t, "cmd:50:42")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &PayloadReq{Speed: tt.speed, Seq: tt.seq}
			if got := p.Encode(); got != tt.want {
				t.Errorf("Encode() = %q, want %q", got, tt.want)
			}
		})
	}
}

// Encode 必须是纯函数：doCheckAndSendPayload 会先打日志再发送，两次结果不能不一样。
func TestPayloadReqEncodeIsStable(t *testing.T) {
	p := NewPayloadReq(50)
	first := p.Encode()
	if second := p.Encode(); first != second {
		t.Errorf("Encode() is not stable: %q then %q", first, second)
	}
}

func TestNewPayloadReqSeqIsNonZero(t *testing.T) {
	for i := 0; i < 100; i++ {
		if seq := NewPayloadReq(10).Seq; seq < 1 || seq > MaxFanSpeed {
			t.Fatalf("seq = %d, want 1-%d", seq, MaxFanSpeed)
		}
	}
}

func TestDecodeToPayloadRes1(t *testing.T) {
	tests := []struct {
		name      string
		line      string
		wantRPM   int
		wantSpeed int
		wantNil   bool
	}{
		{name: "plain", line: "state:1572:10", wantRPM: 1572, wantSpeed: 10},
		{name: "trailing space", line: "state:1572:10 ", wantRPM: 1572, wantSpeed: 10},
		{name: "crlf", line: "state:0:100\r", wantRPM: 0, wantSpeed: 100},
		{name: "boot banner", line: "setup start", wantNil: true},
		{name: "debug line", line: "speed: 128 -> 150", wantNil: true},
		{name: "cmd line", line: "cmd:50:7:ab", wantNil: true},
		{name: "empty", line: "", wantNil: true},
		{name: "missing field", line: "state:1572", wantNil: true},
		{name: "extra field", line: "state:1572:10:20", wantNil: true},
		{name: "non numeric rpm", line: "state:abc:10", wantNil: true},
		{name: "non numeric speed", line: "state:1572:xyz", wantNil: true},
		{name: "speed above range", line: "state:1572:101", wantNil: true},
		{name: "negative speed", line: "state:1572:-1", wantNil: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DecodeToPayloadRes(tt.line)
			if tt.wantNil {
				if got != nil {
					t.Fatalf("DecodeToPayloadRes(%q) = %v, want nil", tt.line, got)
				}
				return
			}
			if got == nil {
				t.Fatalf("DecodeToPayloadRes(%q) = nil, want RPM %d speed %d", tt.line, tt.wantRPM, tt.wantSpeed)
			}
			if got.RPM != tt.wantRPM || got.Speed != tt.wantSpeed {
				t.Errorf("DecodeToPayloadRes(%q) = %v, want RPM %d speed %d", tt.line, got, tt.wantRPM, tt.wantSpeed)
			}
		})
	}
}

// 请求编码出去再按响应格式读回来，两侧对前缀和分隔符的理解必须一致
func TestEncodeDecodeAgreeOnFormat(t *testing.T) {
	req := (&PayloadReq{Speed: 50, Seq: 7}).Encode()
	if DecodeToPayloadRes(req) != nil {
		t.Errorf("DecodeToPayloadRes(%q) accepted a request line, want nil", req)
	}
}

func crcSuffix(t *testing.T, body string) string {
	t.Helper()
	const hex = "0123456789abcdef"
	c := crc8.Checksum([]byte(body), crcTable)
	return string([]byte{hex[c>>4], hex[c&0x0f]})
}
