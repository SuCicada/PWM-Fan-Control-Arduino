import random
import subprocess
import sys
from pathlib import Path

import yaml
from payload import Payload

# sys.path.append(str(Path(__file__).parent.parent.parent))
# sys.path.append(str(Path(__file__).parent.parent.parent / "script"))
# print(sys.path)

def get_fan_speed_from_temperature(temperature, config_file="config.yml"):
    with open(config_file, 'r') as f:
        config = yaml.safe_load(f)
    fan_speed = 0
    for item in config['config']:
        if temperature < item['temp']:
            break
        fan_speed = item['fan']
    return fan_speed

def gpu_temps():
    """
    Uses 'nvidia-smi' to get a list of GPU temperatures as ints
    """
    try:
        out = subprocess.check_output([
            "nvidia-smi",
            "--query-gpu=temperature.gpu",
            "--format=csv,noheader,nounits"
        ], universal_newlines=True)
    except subprocess.CalledProcessError as err:
        # Try to print stderr for diagnosis
        try:
            result = subprocess.run(["nvidia-smi"], capture_output=True, text=True)
            err_output = result.stderr
        except Exception:
            err_output = ""
        print(f"read error: {err}\nnvidia-smi stderr: {err_output}")
        return None
    lines = out.strip().splitlines()
    temps = []
    for ln in lines:
        ln = ln.strip()
        if ln == '':
            continue
        try:
            temps.append(int(ln))
        except Exception as e:
            print(f"parse temp {ln!r} error: {e}")
            return None
    return temps


def get_pc_payload(fan_speed):
    seq = random.randint(1, 100)
    payload = Payload(fan_speed, seq)
    return payload

def main():
    ts = gpu_temps()
    print("gpu temps: ", ts)
    if not ts:
        return
    max_temp = max(ts)
    print("max temp: ", max_temp)
    try:
        fan_speed = get_fan_speed_from_temperature(max_temp)
        print("fan speed: ", fan_speed)


    except Exception as e:
        print(f"get fan speed error: {e}")
        return
    print(f"max temp: {max_temp}°C, fan speed: {fan_speed}")

if __name__ == "__main__":

    main()
    # payload = get_pc_payload(255)
    # print(payload)
    # print(get_fan_speed_from_temperature(50))
