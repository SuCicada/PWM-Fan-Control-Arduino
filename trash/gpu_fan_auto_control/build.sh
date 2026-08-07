apt update
apt install -y software-properties-common
add-apt-repository ppa:deadsnakes/ppa
apt update
apt install -y python3.11 python3.11-pip python-is-python3.11


apt-get install -y gcc binutils make file
# apt install -y 
apt install-y python3-venv
python3 -m venv .venv
. .venv/bin/activate

pip install -r requirements.txt 
pyinstaller --onefile gpu_fan_auto_control.py

