used_pid=$(lsof -t /dev/ttyUSB0)
if [ -n "$used_pid" ]; then
    echo "Killing process $pid"
    kill -9 $used_pid
else
    echo "No process found"
fi


hex_file=$1
if [ -z "$hex_file" ]; then
    echo "Hex file is required"
    exit 1
fi

avrdude -p m328p -c arduino -P /dev/ttyUSB0 -b 115200 -Uflash:w:${hex_file}:i

echo "Done"