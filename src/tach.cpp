// #include "Arduino.h"

// const int tachPin = 2; // 黄线接 D2

// void setup() {
//   pinMode(tachPin, INPUT_PULLUP);
//   attachInterrupt(digitalPinToInterrupt(tachPin), tachISR, FALLING);
//   Serial.begin(9600);
// }

// void loop() {
//   pulseCount = 0;
//   delay(1000); // 计数 1 秒
//   unsigned long rpm = (pulseCount / 2) * 60; // 每转 2 脉冲
//   Serial.print("RPM: ");
//   Serial.println(rpm);
// }