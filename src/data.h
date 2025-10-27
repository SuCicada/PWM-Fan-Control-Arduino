#include <CRC.h>
#include "Arduino.h"

const String KEY = "fanpwm:";

class ReqData {
public:
    int speed = 0;
    int seq = 0;
    uint8_t crc = 0;

    explicit ReqData() {}

    // 单参数构造函数应该标记为explicit，以避免意外的隐式转换。
    static ReqData* NewFromString(String str) {
        ReqData* req = new ReqData();
        if (req->decode(str)) {
            return req;

        } else {
            delete req;
            return nullptr;
        }
    }
    bool decode(String str) {
        str.trim();
        if (str.startsWith(KEY)) {
            String value = str.substring(KEY.length());
            // value[0]

            int speed_i = value.indexOf(':');
            if (speed_i == -1) return false;

            int seq_i = value.indexOf(':', speed_i + 1);
            if (seq_i == -1) return false;

            int crc_i = value.indexOf(':', seq_i + 1);
            if (crc_i == -1) return false;


            this->speed = value.substring(0, speed_i).toInt();
            this->seq = value.substring(seq_i + 1).toInt();
            this->crc = strtol(value.substring(crc_i + 1).c_str(),NULL,16);
            return true;
        }
        return false;
    }

    bool checkCrc() {
        uint8_t crc = calcCrc();
        return this->crc == crc;
    }

    uint8_t calcCrc() {
        String str = String(this->speed) + ":" + String(this->seq);
        uint8_t crc = calcCRC8((uint8_t*) str.c_str(), str.length());
        // this->crc = crc;
        return crc;
    }
};

class RespData {
public:
    int rpm = 0;
    int speed = 0;

    RespData(int rpm,int speed) {
        this->rpm = rpm;
        this->speed = speed;
    }

    String encode() {
        return KEY + String(this->rpm) + ":" + String(this->speed);
    }
};
