// #include <avr/wdt.h>
// #include "Arduino.h"
// #include "data.h"

// const int sensorPin = 3;
// const int fanPin = 5;

// volatile unsigned long pulseCount = 0;

// void tachISR() {
//     pulseCount++;
// }

// void setup() {
//     Serial.begin(115200);
//     pinMode(fanPin, OUTPUT);
//     pinMode(sensorPin, INPUT_PULLUP);
//     attachInterrupt(digitalPinToInterrupt(sensorPin), tachISR, FALLING);

//     analogWrite(fanPin, 0);

//     wdt_enable(WDTO_2S);  // 2秒看门狗
// }

// void setSpeed(int speed) {
//     analogWrite(fanPin, speed);
//     Serial.print("Speed: ");
//     Serial.println(speed);
// }

// int speed = 0;
// int preSeq = 0;

// void loop() {
//     unsigned long rpm = (pulseCount / 2);  // 每转 2 脉冲
//     Serial.print("RPM: ");
//     Serial.println(rpm);
//     pulseCount = 0;

//     int error = 0;
//     if (Serial.available() > 0) {
//         String s = Serial.readString();
//         s.trim();
//         Serial.print("Received: ");
//         Serial.println(s);

//         ReqData* req = ReqData::NewFromString(s);
        
//         if (req != nullptr && req->checkCrc()) {
//             // CRC校验通过，使用解析出的速度值
//             if (req->speed > 0) {
//                 if (req->speed > 255) {
//                     req->speed = 255;
//                 }
//                 if (req->speed < 1) {
//                     req->speed = 1;
//                 }
//                 setSpeed(req->speed);
//                 speed = req->speed; // 更新全局speed变量
//             } else {
//                 error = 1;
//             }
//             delete req; // 释放内存
//         } else {
//             // 解析失败或CRC校验失败，尝试作为简单数字处理（向后兼容）
//             if (req != nullptr) {
//                 delete req; // 释放内存
//             }
//             int simpleSpeed = s.toInt();
//             if (simpleSpeed > 0) {
//                 if (simpleSpeed > 255) {
//                     simpleSpeed = 255;
//                 }
//                 if (simpleSpeed < 1) {
//                     simpleSpeed = 1;
//                 }
//                 setSpeed(simpleSpeed);
//                 speed = simpleSpeed;
//             } else {
//                 error = 1;
//             }
//         }
//     }

//     Serial.println("Speed: " + String(speed));

//     if (error == 0) {
//         wdt_reset();  // 喂狗操作，使看门狗定时器复位
//     }

//     delay(1000);
// }
