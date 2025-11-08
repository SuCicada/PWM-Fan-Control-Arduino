# Arduino PWM Fan Control System

An intelligent PWM fan control system based on Arduino, featuring real-time RPM monitoring and GPU temperature-adaptive speed adjustment.

## Core Features

### Arduino Hardware Controller
- **Precise PWM Control**: 0-255 speed levels, supports 4-wire PWM fans
- **Real-time RPM Monitoring**: Hardware interrupt-driven tachometer detection
- **Data Validation**: CRC8 checksum ensures reliable serial communication
- **Power-off Memory**: EEPROM saves speed settings, automatically restores after power loss
- **Watchdog Protection**: Prevents system crashes, automatic recovery

### Go Software Controller
- **GPU Temperature Adaptive**: Automatically adjusts fan speed based on NVIDIA GPU temperature
- **Flexible Configuration**: YAML config file for custom temperature curves
- **Multiple Modes**: Auto mode, manual mode, read-only mode, test mode

## Technical Highlights

1. **Industrial-Grade Reliability**
   - CRC validation ensures accurate data transmission
   - Watchdog timer prevents system crashes
   - EEPROM persistent storage

2. **Real-time Feedback**
   - Reads actual fan RPM values
   - Can detect fan failures
   - Bidirectional communication protocol

3. **Hardware Interrupt Design**
   - Uses hardware interrupts for precise tachometer pulse counting
   - Accurate measurements even under high load

4. **Intelligent Temperature Curve**
   - Automatically finds optimal fan speed based on GPU temperature
   - Smooth transitions between temperature zones

## Hardware Connection

- **Arduino Nano** (ATmega328P)
- **4-wire PWM Fan**
- **Pin Configuration**:
  - Pin 3: Fan TACH signal (interrupt input)
  - Pin 5: PWM output

[Hardware Connection Diagram](./doc/breadboard_.png)

## Quick Start

### Configuration File Example

```yaml
fan_level:
  - temp: 20
    fan: 50
  - temp: 30
    fan: 150
  - temp: 35
    fan: 255
    
serial_port: /dev/ttyUSB0
```

## Usage Examples

```bash
# Manually set fan speed
./gpu_fan_auto_control -fan 150

# Read current speed
./gpu_fan_auto_control -readonly

# Automatic temperature control
./gpu_fan_auto_control

# Test mode (doesn't actually change speed)
./gpu_fan_auto_control -dryrun
```

## Communication Protocol

```
Request:  fanpwm:<speed>:<sequence>:<CRC8>
Response: fanpwm:<RPM>:<speed>
```

## Future Improvements

- [ ] Fix RPM detection
- [ ] Real-time RPM logging to PC

