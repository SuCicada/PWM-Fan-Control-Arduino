#include <avr/wdt.h>
#include "Arduino.h"
#include "data.h"

const int sensorPin = 3;
const int fanPin = 5;

volatile unsigned long pulseCount = 0;

void tachISR() {
    pulseCount++;
}

void setup() {
    Serial.begin(115200);
    pinMode(fanPin, OUTPUT);
    pinMode(sensorPin, INPUT_PULLUP);
    // pinMode(buttonPin, INPUT);
    // pinMode(fanLightPin, OUTPUT);
    attachInterrupt(digitalPinToInterrupt(sensorPin), tachISR, FALLING);

    analogWrite(fanPin, 0);

    wdt_enable(WDTO_2S);  // 8 ms watchdog
}

void setSpeed(int speed) {
    analogWrite(fanPin, speed);
    Serial.print("Speed: ");
    Serial.println(speed);
}

int speed = 0;

int preSeq = 0;
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
            return false;
        }

        if (req->seq == preSeq) {
            return false;
        }

        preSeq = req->seq;

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
    Serial.print("RPM: ");
    Serial.println(rpm);
    pulseCount = 0;


    String resp = RespData(rpm, speed).encode();
    Serial.println(resp);
}


void loop() {
    
    getRpm();

    bool ok = readAndHandle();
    if (!ok) {
        wdt_reset();  // 喂狗操作，使看门狗定时器复位
    }

    delay(1000);
}
