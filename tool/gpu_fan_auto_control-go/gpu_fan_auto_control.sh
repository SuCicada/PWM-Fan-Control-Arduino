#! /bin/bash
bin_dir=/home/peng/TOOL/gpu_fan_auto_control

${bin_dir}/gpu_fan_auto_control \
    -port /dev/ttyUSB0 \
    -config ${bin_dir}/config.yml 

