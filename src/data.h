#pragma once

#include <Arduino.h>
#include <CRC.h>

// 线协议（两个方向都以 '\n' 结尾）：
//   请求  cmd:<speed>:<seq>:<crc8>
//   响应  state:<rpm>:<speed>
// crc8 是 "cmd:<speed>:<seq>" 的 CRC-8，两位十六进制。
// 参数为 poly 0x07 / init 0x00 / xorOut 0x00 / 不反转，与 Go 侧 crc8.CRC8 一致。
const char CMD_KEY[] = "cmd:";
const uint8_t CMD_KEY_LEN = sizeof(CMD_KEY) - 1;
const char STATE_KEY[] = "state:";
const uint8_t STATE_KEY_LEN = sizeof(STATE_KEY) - 1;

// 重建待校验串所需空间：前缀 + 两个 int 的十进制形式 + 分隔符
const uint8_t CRC_BUF_SIZE = CMD_KEY_LEN + 16;

struct ReqData {
    int speed = 0;
    int seq = 0;
    uint8_t crc = 0;

    bool decode(const char* line) {
        if (strncmp(line, CMD_KEY, CMD_KEY_LEN) != 0) {
            return false;
        }
        const char* value = line + CMD_KEY_LEN;

        const char* seqSep = strchr(value, ':');
        if (seqSep == nullptr) {
            return false;
        }
        const char* crcSep = strchr(seqSep + 1, ':');
        if (crcSep == nullptr) {
            return false;
        }

        // atoi 遇到 ':' 自然停止，不需要先切分出子串
        speed = atoi(value);
        seq = atoi(seqSep + 1);
        crc = (uint8_t) strtol(crcSep + 1, nullptr, 16);
        return true;
    }

    uint8_t expectedCrc() const {
        char buf[CRC_BUF_SIZE];
        int n = snprintf(buf, sizeof(buf), "%s%d:%d", CMD_KEY, speed, seq);
        if (n <= 0) {
            return 0;
        }
        if (n > (int) sizeof(buf) - 1) {
            n = sizeof(buf) - 1;
        }
        return calcCRC8((const uint8_t*) buf, (crc_size_t) n);
    }

    bool checkCrc() const {
        return crc == expectedCrc();
    }
};

struct RespData {
    unsigned long rpm = 0;
    int speed = 0;

    RespData(unsigned long rpm, int speed) : rpm(rpm), speed(speed) {}

    void printTo(Print& out) const {
        out.print(STATE_KEY);
        out.print(rpm);
        out.print(':');
        out.println(speed);
    }
};
