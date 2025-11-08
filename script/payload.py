# pip install crc
import random

from crc import Calculator, Crc8

KEY = "fanpwm:"

class Payload:
    def __init__(self, speed, seq):
        self.speed = speed
        self.seq = seq
        self.crc = self.calc_crc(f"{KEY}{speed}:{seq}")

    def encode(self):
        return f"{KEY}{self.speed}:{self.seq}:{self.crc}"
    
    def __str__(self):
        return self.encode()
    
    def calc_crc(self, data):
        crc8 = Calculator(Crc8.CCITT)
        res= crc8.checksum(data.encode())
        hex_res = hex(res)[2:].zfill(2)
        return hex_res

import random
import sys

if __name__ == "__main__":
    SPEED = sys.argv[1]
    SEQ = random.randint(1, 100)

    payload = Payload(SPEED, SEQ)
    print(payload)