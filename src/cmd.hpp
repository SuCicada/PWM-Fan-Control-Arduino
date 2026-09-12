#pragma once

#include "Arduino.h"
#include "data.h"

// 放得下 "cmd:255:255:ff" 并留有余量
const uint8_t CMD_BUF_SIZE = 32;
char cmdBuf[CMD_BUF_SIZE];
uint8_t cmdLen = 0;
char lastCmd[CMD_BUF_SIZE] = "";
ReqData req;

bool parseCommand(const char* cmd) {
    Serial.print(F("recv: "));
    Serial.println(cmd);

    if (!req.decode(cmd)) {
        Serial.println(F("not a command, skip"));
        return false;
    }
    if (!req.checkCrc()) {
        Serial.print(F("crc error, want "));
        Serial.println(req.expectedCrc(), HEX);
        return false;
    }
    if (strcmp(lastCmd, cmd) == 0) {
        Serial.println(F("duplicate cmd, skip"));
        return false;
    }
    strncpy(lastCmd, cmd, CMD_BUF_SIZE - 1);
    lastCmd[CMD_BUF_SIZE - 1] = '\0';
    return true;
}

// 攒够一行且校验通过才返回 true。成功后速度在 req.speed。
bool pollSerial() {
    while (Serial.available() > 0) {
        char c = (char) Serial.read();
        if (c == '\r') {
            continue;
        }
        if (c != '\n') {
            if (cmdLen < CMD_BUF_SIZE - 1) {
                cmdBuf[cmdLen++] = c;
            }
            continue;
        }

        cmdBuf[cmdLen] = '\0';
        uint8_t n = cmdLen;
        cmdLen = 0;
        if (n > 0 && parseCommand(cmdBuf)) {
            return true;
        }
    }
    return false;
}
