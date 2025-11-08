#include <CRC.h>
#include "HardwareSerial.h"
#include <StringUtil.h>
using namespace StringUtil;

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
            fprintf(Serial, "value: %s\n", value.c_str());
            int speed_i = 0;
            // if (speed_i == -1) return false;

            int seq_i = value.indexOf(':', speed_i + 1);
            if (seq_i == -1) return false;

            int crc_i = value.indexOf(':', seq_i + 1);
            if (crc_i == -1) return false;


            this->speed = value.substring(speed_i, seq_i).toInt();
            this->seq = value.substring(seq_i + 1, crc_i).toInt();
            this->crc = strtol(value.substring(crc_i + 1).c_str(),NULL,16);

            fprintf(Serial, "decode res: speed: %d, seq: %d, crc: %d\n", this->speed, this->seq, this->crc);
            return true;
        }
        Serial.println("not cmd, skip");
        return false;
    }

    bool checkCrc() {
        uint8_t crc = calcCrc();
        bool res = this->crc == crc;
        if (!res) {
            fprintf(Serial, "checkCrc error: %d, %d\n", this->crc, crc);
        }
        return res;
    }

    uint8_t calcCrc() {
        String str = KEY + String(this->speed) + ":" + String(this->seq);
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
