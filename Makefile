ifneq (,$(wildcard .env))
	include .env
	export
endif

env = nano
# env = test-nano

# remote-upload:
# 	until pio remote run -t upload -e nano -v ; do sleep 1; done
# 	@make remote-monitor
# remote-monitor:
# 	pio remote device monitor -e nano

build:
	pio run -e $(env) -v

.PHONY: build deploy _upload
_upload: build
	$(call upload, .pio/build/$(env)/firmware.hex, /tmp/firmware-$(env).hex)
	$(call ssh_file, ./deploy.sh, /tmp/firmware-$(env).hex)
# pio run -t upload -e $(env)

# 本地 Uno 测试烧录（只用 test/）
upload-test:
	pio run -t upload -e test-uno

upload-fan:
	env=nano sumake _upload

# build-gpu_fan_auto_control:
# upload-gpu_fan_auto_control-py:
# 	cp tool/gpu_fan_auto_control/config.yml tool/gpu_fan_auto_control/dist/config.yml
# 	$(call upload, tool/gpu_fan_auto_control/dist/, TOOL/gpu_fan_auto_control/)
upload-gpu_fan_auto_control:
	sumake -C tool/gpu_fan_auto_control-go upload
