#include <avr/wdt.h>
#include <util/atomic.h>
#include <EEPROM.h>
#include "Arduino.h"
#include "data.h"

const int sensorPin = 3;
const int fanPin = 5;

const int EEPROM_SPEED_ADDR = 0;
const int EEPROM_MAGIC_ADDR = 1;
// 空白 EEPROM 读出来是 0xFF，靠单独的标记位区分「存过 255」和「从没存过」
const uint8_t EEPROM_MAGIC = 0xA5;

const int DEFAULT_SPEED = 128;

// 中断挂在 CHANGE 上，一个 tach 脉冲的上升沿和下降沿各触发一次；
// 4 线风扇每转 2 个脉冲，所以每转对应 4 次计数。
const unsigned long COUNTS_PER_REV = 4;
const unsigned long REPORT_INTERVAL_MS = 1000;

// 放得下 "fanpwm:255:255:ff" 并留有余量
const uint8_t CMD_BUF_SIZE = 32;

volatile unsigned long pulseCount = 0;

void tachISR() {
    pulseCount++;
}

int currentSpeed = 0;

char cmdBuf[CMD_BUF_SIZE];
uint8_t cmdLen = 0;
char lastCmd[CMD_BUF_SIZE] = "";

unsigned long lastReportMs = 0;

void setSpeed(int speed) {
    analogWrite(fanPin, speed);

    Serial.print(F("speed: "));
    Serial.print(currentSpeed);
    Serial.print(F(" -> "));
    Serial.println(speed);

    if (currentSpeed != speed) {
        EEPROM.update(EEPROM_SPEED_ADDR, (uint8_t) speed);
        EEPROM.update(EEPROM_MAGIC_ADDR, EEPROM_MAGIC);
    }
    currentSpeed = speed;
}

void handleCommand(const char* line) {
    Serial.print(F("recv: "));
    Serial.println(line);

    ReqData req;
    if (!req.decode(line)) {
        Serial.println(F("not a command, skip"));
        return;
    }
    if (!req.checkCrc()) {
        Serial.print(F("crc error, want "));
        Serial.println(req.expectedCrc(), HEX);
        return;
    }
    if (strcmp(lastCmd, line) == 0) {
        Serial.println(F("duplicate cmd, skip"));
        return;
    }
    strncpy(lastCmd, line, CMD_BUF_SIZE - 1);
    lastCmd[CMD_BUF_SIZE - 1] = '\0';

    int speed = req.speed;
    if (speed < 0) {
        speed = 0;
    }
    if (speed > 255) {
        speed = 255;
    }
    setSpeed(speed);
}

// 逐字节收取，攒够一整行才交给 handleCommand。
// 全程不阻塞，半行数据会留在缓冲区等下一轮，看门狗因此始终有充足余量。
void pollSerial() {
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
        if (cmdLen > 0) {
            handleCommand(cmdBuf);
        }
        cmdLen = 0;
    }
}

void reportRpm() {
    unsigned long now = millis();
    unsigned long elapsed = now - lastReportMs;
    if (elapsed < REPORT_INTERVAL_MS) {
        return;
    }
    lastReportMs = now;

    // pulseCount 有 4 字节，AVR 上读写会被 ISR 从中间打断，必须整块保护
    unsigned long counts;
    ATOMIC_BLOCK(ATOMIC_RESTORESTATE) {
        counts = pulseCount;
        pulseCount = 0;
    }

    // 按实际经过的毫秒数换算，不能假定这一轮刚好是 1 秒
    unsigned long rpm = (counts * 60000UL) / (COUNTS_PER_REV * elapsed);

    RespData(rpm, currentSpeed).printTo(Serial);
}

void setup() {
    Serial.begin(9600);

    Serial.println(F("========================"));
    Serial.println(F("setup start"));

    pinMode(fanPin, OUTPUT);
    pinMode(sensorPin, INPUT_PULLUP);
    attachInterrupt(digitalPinToInterrupt(sensorPin), tachISR, CHANGE);

    int savedSpeed = DEFAULT_SPEED;
    if (EEPROM.read(EEPROM_MAGIC_ADDR) == EEPROM_MAGIC) {
        savedSpeed = EEPROM.read(EEPROM_SPEED_ADDR);
        Serial.print(F("restored speed from EEPROM: "));
    } else {
        Serial.print(F("no speed in EEPROM, using default: "));
    }
    Serial.println(savedSpeed);

    // 先对齐 currentSpeed，避免 setSpeed 把刚读出来的值原样写回 EEPROM
    currentSpeed = savedSpeed;
    setSpeed(savedSpeed);

    lastReportMs = millis();

    // 一轮 loop 现在最多几百毫秒（全是串口输出耗时），2s 留了足够余量
    wdt_enable(WDTO_2S);
    Serial.println(F("setup done"));
    Serial.println(F("========================"));
}

void loop() {
    pollSerial();
    reportRpm();
    wdt_reset();
}
