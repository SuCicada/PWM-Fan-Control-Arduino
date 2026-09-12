#pragma once

#include <EEPROM.h>
#include "Arduino.h"

const uint8_t EEPROM_MAGIC_ADDR = 1;      // 标记位，标记是否存过速度
const uint8_t EEPROM_SPEED_PTR_ADDR = 2;  // 当前 speed code 所在地址

// 空白 EEPROM 读出来是 0xFF，靠魔数区分「存过 255」和「从没存过」
const uint8_t EEPROM_MAGIC = 0xA5;
const uint8_t DEFAULT_SPEED_PERCENT = 50; // 50%

// 磨损均衡：speed code 占 2 字节，从 3 开始绕开魔数和指针
const uint8_t SPEED_CODE_SIZE = 2;
const uint8_t SPEED_DATA_START = 3;
const uint8_t SPEED_DATA_LAST = 253;  // 最后一格起点，保证 addr+1 仍在 255 内

struct SpeedRecord {
    bool ok;
    uint8_t speed_percent;
};

inline uint8_t speedChecksum(uint8_t speed_percent) {
    return speed_percent ^ EEPROM_MAGIC;
}

// 高字节=speed，低字节=checksum。空白 0xFFFF 校验必失败。
inline uint16_t encodeSpeedCode(uint8_t speed_percent) {
    return ((uint16_t)speed_percent << 8) | speedChecksum(speed_percent);
}

inline SpeedRecord decodeSpeedCode(uint16_t code) {
    SpeedRecord rec;
    rec.speed_percent = (uint8_t)(code >> 8);
    uint8_t chk = (uint8_t)(code & 0xFF);
    rec.ok = (speedChecksum(rec.speed_percent) == chk);
    if (!rec.ok) {
        rec.speed_percent = DEFAULT_SPEED_PERCENT;
    }
    return rec;
}

// 读到相同就不擦写，避免无谓磨损。返回是否真正写入。
inline bool eepromWrite(uint8_t addr, uint8_t value) {
    if (EEPROM.read(addr) == value) {
        return false;
    }
    EEPROM.write(addr, value);
    return true;
}

inline uint16_t readSpeedCodeAt(uint8_t addr) {
    uint8_t hi = EEPROM.read(addr);
    uint8_t lo = EEPROM.read(addr + 1);
    return ((uint16_t)hi << 8) | lo;
}

inline void writeSpeedCodeAt(uint8_t addr, uint16_t code) {
    eepromWrite(addr, (uint8_t)(code >> 8));
    eepromWrite(addr + 1, (uint8_t)(code & 0xFF));
}

inline bool addrInSpeedRegion(uint8_t addr) {
    return addr >= SPEED_DATA_START && 
           addr <= SPEED_DATA_LAST &&
           ((addr - SPEED_DATA_START) % SPEED_CODE_SIZE == 0);
}

// 对两字节格子各写一次取反再读回，能对上才算这个地址能写。
inline bool canWriteSpeedAddr(uint8_t addr) {
    if (!addrInSpeedRegion(addr)) {
        return false;
    }
    for (uint8_t i = 0; i < SPEED_CODE_SIZE; i++) {
        uint8_t old = EEPROM.read(addr + i);
        uint8_t probe = (uint8_t)~old;
        eepromWrite(addr + i, probe);
        bool ok = EEPROM.read(addr + i) == probe;
        eepromWrite(addr + i, old);
        if (!ok) {
            return false;
        }
    }
    return true;
}

inline uint8_t nextSpeedAddr(uint8_t addr) {
    uint16_t next = (uint16_t)addr + SPEED_CODE_SIZE;
    if (!addrInSpeedRegion(addr) || next > SPEED_DATA_LAST) {
        return SPEED_DATA_START;
    }
    return (uint8_t)next;
}

inline uint8_t readSpeedPercent() {
    if (EEPROM.read(EEPROM_MAGIC_ADDR) != EEPROM_MAGIC) {
        Serial.print(F("no speed in EEPROM, using default: "));
        Serial.println(DEFAULT_SPEED_PERCENT);
        return DEFAULT_SPEED_PERCENT;
    }

    uint8_t addr = EEPROM.read(EEPROM_SPEED_PTR_ADDR);
    SpeedRecord rec = decodeSpeedCode(readSpeedCodeAt(addr));
    if (!rec.ok) {
        Serial.print(F("speed code invalid, using default: "));
        Serial.println(DEFAULT_SPEED_PERCENT);
        return DEFAULT_SPEED_PERCENT;
    }

    Serial.print(F("restored speed from EEPROM: "));
    Serial.println(rec.speed_percent);
    return rec.speed_percent;
}

inline bool writeSpeedPercent(uint8_t speed_percent) {
    uint8_t addr = nextSpeedAddr(EEPROM.read(EEPROM_SPEED_PTR_ADDR));
    const uint8_t slots =
        (SPEED_DATA_LAST - SPEED_DATA_START) / SPEED_CODE_SIZE + 1;

    for (uint8_t i = 0; i < slots; i++) {
        if (!canWriteSpeedAddr(addr)) {
            Serial.print(F("eeprom addr not writable: "));
            Serial.println(addr);
            addr = nextSpeedAddr(addr);
            continue;
        }

        uint16_t code = encodeSpeedCode(speed_percent);
        writeSpeedCodeAt(addr, code);
        SpeedRecord rec = decodeSpeedCode(readSpeedCodeAt(addr));
        if (!rec.ok || rec.speed_percent != speed_percent) {
            Serial.print(F("eeprom verify failed at: "));
            Serial.println(addr);
            addr = nextSpeedAddr(addr);
            continue;
        }

        eepromWrite(EEPROM_SPEED_PTR_ADDR, addr);
        eepromWrite(EEPROM_MAGIC_ADDR, EEPROM_MAGIC);
        Serial.print(F("eeprom speed stored at: "));
        Serial.println(addr);
        return true;
    }

    Serial.println(F("eeprom write failed, keep RAM speed"));
    return false;
}
