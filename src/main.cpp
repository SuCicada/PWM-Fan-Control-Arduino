#include <avr/wdt.h>
#include <util/atomic.h>
#include "Arduino.h"
#include "data.h"
#include "eeprom.hpp"
#include "cmd.hpp"
#include "fan.hpp"

// const int sensorPin = 3;

// // 中断挂在 CHANGE 上，一个 tach 脉冲的上升沿和下降沿各触发一次；
// // 4 线风扇每转 2 个脉冲，所以每转对应 4 次计数。
// const unsigned long COUNTS_PER_REV = 4;
// const unsigned long REPORT_INTERVAL_MS = 1000;


// volatile unsigned long pulseCount = 0;



// const int fanPin = 9;

const int fanPin = 9;
const int sensorPin = 3;


void setup() {
    Serial.begin(9600);

    Serial.println(F("========================"));
    Serial.println(F("setup start"));

    initFan(fanPin, sensorPin);

    int speed_percent = readSpeedPercent();
    setSpeed(speed_percent);
    // current_speed_percent = speed_percent;

    lastReportMs = millis();

    // 一轮 loop 现在最多几百毫秒（全是串口输出耗时），2s 留了足够余量
    wdt_enable(WDTO_2S);
    Serial.println(F("setup done"));
    Serial.println(F("========================"));
}
void loop() {

    pollTach();

    if (pollSerial()) {
        setSpeed(req.speed);
    }

    reportRpm();
    wdt_reset();
}
