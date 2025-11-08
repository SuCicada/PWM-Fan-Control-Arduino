ifneq (,$(wildcard .env))
	include .env
	export
endif

env = nano

# remote-upload:
# 	until pio remote run -t upload -e nano -v ; do sleep 1; done
# 	@make remote-monitor
# remote-monitor:
# 	pio remote device monitor -e nano

build:
	pio run -e $(env) -v

.PHONY: build deploy upload
upload: build
	$(call upload, .pio/build/nano/firmware.hex, /tmp/firmware-$(env).hex)
	$(call ssh_file, ./deploy.sh, /tmp/firmware-$(env).hex)
# pio run -t upload -e $(env)

upload-fan:
	env=nano sumake upload

# build-gpu_fan_auto_control:
upload-gpu_fan_auto_control-py:
	cp tool/gpu_fan_auto_control/config.yml tool/gpu_fan_auto_control/dist/config.yml
	$(call upload, tool/gpu_fan_auto_control/dist/, TOOL/gpu_fan_auto_control/)

upload-gpu_fan_auto_control:
	cd tool/gpu_fan_auto_control-go && \
		GOARCH=amd64 GOOS=linux go build -o gpu_fan_auto_control .
	$(call upload, tool/gpu_fan_auto_control-go/gpu_fan_auto_control, TOOL/gpu_fan_auto_control/gpu_fan_auto_control)
	$(call upload, tool/gpu_fan_auto_control-go/config.yml, TOOL/gpu_fan_auto_control/config.yml)