#include <avr/wdt.h>
#include <util/atomic.h>
#include "Arduino.h"
#include "data.h"
#include "eeprom.hpp"


// 4 线风扇 tach 开漏，每转 2 个脉冲。
// 不用边沿中断：PWM 一翻转就会往 tach 线串 25kHz 毛刺，ISR 会被打满，
// 间隙去抖也只是把噪声降采样成「看起来很稳」的假转速。
const unsigned long COUNTS_PER_REV = 2;
const unsigned long REPORT_INTERVAL_MS = 1000;
// 电平要稳住这么久才算一次翻转。25kHz 周期 40us，过不了这关；
// 真 tach 在几千 RPM 时低电平仍有数毫秒。
const unsigned long TACH_STABLE_US = 1000;

// 20% 能维持已在转的转子，拉不动停住的。连续几秒 0 脉冲就全速 1 秒再回到原占空比，
// 和插拔时 PWM 脚悬空、风扇按规范全速转一下是同一件事。
// 仍然是 0 就拉长间隔，避免转速线断开时整晚每几秒吼一次。
const unsigned long STALL_ZERO_MS = 5000;
const unsigned long KICK_MS = 1000;
const uint8_t KICK_STRIKE_CAP = 3;


int tachPin = 3;
unsigned long pulseCount = 0;
bool tachRaw = HIGH; // 上一拍读到的原始电平。一变就重置
bool tachStable = HIGH; // 已经通过 1ms 门、可以拿来数脉冲的电平。只有它翻转才可能 pulseCount++。
unsigned long tachChangeUs = 0;


int current_speed_percent = 0;


unsigned long lastReportMs = 0;

bool kicking = false;
unsigned long kickStartMs = 0;
bool zeroArm = false;
unsigned long zeroSinceMs = 0;
uint8_t stallStrikes = 0;
unsigned long nextKickOkMs = 0;


void pollTach() {
    bool level = digitalRead(tachPin);
    unsigned long now = micros();
    if (level != tachRaw) {
        tachRaw = level;
        tachChangeUs = now;
        return;
    }
    if ((now - tachChangeUs) < TACH_STABLE_US) {
        return;
    }
    if (level != tachStable) {
        if (tachStable == HIGH && level == LOW) {
            pulseCount++;
        }
        tachStable = level;
    }
}

void resetTachWindow() {
    pulseCount = 0;
    tachRaw = digitalRead(tachPin);
    tachStable = tachRaw;
    tachChangeUs = micros();
    lastReportMs = millis();
}

void initFan(int fanPin, int sensorPin) {
    tachPin = sensorPin;
    pinMode(sensorPin, INPUT_PULLUP);
    resetTachWindow();

    pinMode(fanPin, OUTPUT);
    TCCR1A = 0;
    TCCR1B = 0;

    // https://onlinedocs.microchip.com/oxy/GUID-80B1922D-872B-40C8-A8A5-0CBE009FD908-en-US-3/GUID-853E47EF-C46F-422D-AD77-A76D833D0760.html

    // Compare Output Mode, Phase Correct and Phase and Frequency Correct PWM
    // non-inverting mode
    // 设置比较匹配输出模式: 当向上计数时清除 OC1A，向下计数时设置 OC1A（非反转 PWM）
    // COM1A1 COM1A0 = 10  -> Non-inverting PWM
    TCCR1A |= (1 << COM1A1) | (0 << COM1A0);

    // Waveform Generation Mode
    // 设置为 Mode 10: 阶段与频率修正 PWM (Phase and Frequency Correct PWM), 顶点为 ICR1
    // ICR1
    // WGM13 WGM12 WGM11 WGM10 = 1000
    TCCR1A |= (0 << WGM11) | (0 << WGM10);
    TCCR1B |= (1 << WGM13) | (0 << WGM12);

    // 关键：设置时钟选择位 (Clock Select)
    // CS12=0, CS11=0, CS10=1 -> 不分频 (Prescaler = 1)
    // CS12 CS11 CS10 = 001 -> clk/1
    TCCR1B |= (1 << CS10);

    // 计算 ICR1 顶点值以达到 25kHz 频率：
    // 频率 = F_CPU / (2 * 分频 * ICR1)
    // 25000 = 16000000 / (2 * 1 * ICR1) -> ICR1 = 320
    ICR1 = 320;

    // 50% duty
    // OCR1A = 160;
}

void writeDuty(int speed_percent) {
    OCR1A = (uint32_t)speed_percent * ICR1 / 100;
}

// 第 1 次失败后再等 1 分钟，第 2 次 5 分钟，之后 10 分钟。
unsigned long kickCooldownMs(uint8_t strikes) {
    if (strikes <= 1) {
        return 60000UL;
    }
    if (strikes == 2) {
        return 300000UL;
    }
    return 600000UL;
}

void clearStall() {
    zeroArm = false;
    stallStrikes = 0;
    nextKickOkMs = 0;
}

void setSpeed(int speed_percent) {
    if (speed_percent < 0) {
        speed_percent = 0;
    }
    if (speed_percent > 100) {
        speed_percent = 100;
    }
    if (speed_percent != (int)current_speed_percent) {
        Serial.print(F("set speed to "));
        Serial.println(speed_percent);

        kicking = false;
        clearStall();
        writeDuty(speed_percent);
        writeSpeedPercent((uint8_t)speed_percent);
        current_speed_percent = speed_percent;
        // 调速后旧窗口里的脉冲/噪声不能拿来换算，丢掉重开一秒窗
        resetTachWindow();
    }
}

// 全速这一秒里的读数不能用来判断「已经转起来了」，20% 接回来之后才算。
void noteRpmSample(unsigned long rpm) {
    if (kicking) {
        return;
    }
    if (current_speed_percent <= 0 || rpm > 0) {
        clearStall();
        return;
    }

    unsigned long now = millis();
    if (!zeroArm) {
        zeroArm = true;
        zeroSinceMs = now;
        return;
    }
    if (now - zeroSinceMs < STALL_ZERO_MS) {
        return;
    }
    if (nextKickOkMs != 0 && (long)(nextKickOkMs - now) > 0) {
        return;
    }

    kicking = true;
    kickStartMs = now;
    if (stallStrikes < KICK_STRIKE_CAP) {
        stallStrikes++;
    }
    // OCR1A == TOP 时输出恒高，没有 25kHz 边沿，这一秒的 tach 也是干净的
    writeDuty(100);
    Serial.println(F("fan stalled, full speed kick"));
}

void serviceFanKick() {
    if (!kicking) {
        return;
    }
    if (millis() - kickStartMs < KICK_MS) {
        return;
    }
    kicking = false;
    writeDuty(current_speed_percent);
    nextKickOkMs = millis() + kickCooldownMs(stallStrikes);
    zeroArm = false;
    resetTachWindow();
}

void reportRpm() {
    unsigned long now = millis();
    unsigned long elapsed = now - lastReportMs;
    if (elapsed < REPORT_INTERVAL_MS) {
        return;
    }
    lastReportMs = now;

    unsigned long counts = pulseCount;
    pulseCount = 0;

    // 按实际经过的毫秒数换算，不能假定这一轮刚好是 1 秒
    unsigned long rpm = (counts * 60000UL) / (COUNTS_PER_REV * elapsed);

    noteRpmSample(rpm);
    // 上报的仍是设定占空比。全速只是把停住的转子带起来，主机不用跟着改档。
    RespData(rpm, current_speed_percent).printTo(Serial);
}

