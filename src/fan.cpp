#include <avr/wdt.h>
#include <EEPROM.h>
#include "Arduino.h"
#include "StringUtil.h"
#include "WString.h"
#include "data.h"

const int sensorPin = 3;
const int fanPin = 5;

// EEPROM 地址用于保存风扇速度
const int EEPROM_SPEED_ADDR = 0;

volatile unsigned long pulseCount = 0;

void tachISR() {
    pulseCount++;
}

int SPEED = 0;

void setSpeed(int speed) {
    analogWrite(fanPin, speed);
    Serial.print("Speed: ");
    Serial.println(speed);
    
    // 只有当速度真正改变时才写入 EEPROM（避免频繁写入）
    if (SPEED != speed) {
        EEPROM.write(EEPROM_SPEED_ADDR, speed);
        Serial.println("Speed saved to EEPROM");
    }
    
    SPEED = speed;
}

// int preSeq = 0;
String preCmd = "";

bool readAndHandle() {
    if (Serial.available() > 0) {
        // 如果串口缓冲区中有可用数据
        String s = Serial.readString();
        //        char receivedChar = (char)Serial.read(); // 读取字符
        // 在这里可以对接收到的数据进行处理
        s.trim();
        Serial.print("Received: ");  // 发送响应消息
        Serial.println(s);

        // if (s == "reset") {
        // }

        ReqData* req = ReqData::NewFromString(s);
        if (req == nullptr) {
            return false;
        }

        if (!req->checkCrc()) {
            Serial.println("CRC error");
            return false;
        }

        // if (req->seq == preSeq) {
        // return true;
        // }

        if (preCmd == s) {
            Serial.println("cmd repeat, skip");
            return true;
        }

        // preSeq = req->seq;
        preCmd = s;

        int speed = req->speed;
        if (speed > 0) {
            if (speed > 255) {
                speed = 255;
            }
            if (speed < 1) {
                speed = 1;
            }
            setSpeed(speed);

        } else {
            return false;
        }
    }

    return true;
}

void getRpm() {
    unsigned long rpm = (pulseCount / 2);  // 每转 2 脉冲
    rpm *= 60;
    fprintf(Serial, "pulseCount: %l, RPM: %l \n", pulseCount, rpm);
    Serial.print("RPM: ");
    Serial.println(rpm);
    pulseCount = 0;

    String resp = RespData(rpm, SPEED).encode();
    Serial.println(resp);
}

void setup() {
    // Serial.begin(115200);
    Serial.begin(9600);

    // wdt_disable();
    Serial.println("========================");
    Serial.println("setup start");

    pinMode(fanPin, OUTPUT);
    pinMode(sensorPin, INPUT_PULLUP);
    // 边沿触发
    // attachInterrupt(digitalPinToInterrupt(sensorPin), tachISR, FALLING);
    // 尝试上升沿
    // attachInterrupt(digitalPinToInterrupt(sensorPin), tachISR, RISING);
    // 或者尝试任意边沿变化
    attachInterrupt(digitalPinToInterrupt(sensorPin), tachISR, CHANGE);

    // analogWrite(fanPin, 0);
    Serial.println("attachInterrupt setup done");

    // 从 EEPROM 读取上次保存的速度
    int savedSpeed = EEPROM.read(EEPROM_SPEED_ADDR);
    // 检查读取的值是否有效（0-255），如果是首次使用或无效值，使用默认速度 128
    if (savedSpeed <= 0 || savedSpeed > 255) {
        savedSpeed = 128;  // 默认 50% 速度
        Serial.println("No valid speed in EEPROM, using default: 128");
    } else {
        Serial.print("Restored speed from EEPROM: ");
        Serial.println(savedSpeed);
    }
    setSpeed(savedSpeed);

    wdt_enable(WDTO_2S);
    Serial.println("wdt enabled");
    Serial.println("setup done");
    Serial.println("========================");
}

//
// void setup() {
//
// }
void loop() {
    getRpm();

    bool ok = readAndHandle();
    if (ok) {
        wdt_reset();  // 喂狗操作，使看门狗定时器复位
    }

    delay(1000);
}
