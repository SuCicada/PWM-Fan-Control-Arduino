```bash
docker run -v $(pwd):/app -w /app -it --platform linux/amd64 --name gpu_fan_auto_control ubuntu:24.04 /bin/bash

pyinstaller --onefile gpu_fan_auto_control.py
```