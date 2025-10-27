ifneq (,$(wildcard .env))
	include .env
	export
endif

env = nano

remote-upload:
	until pio remote run -t upload -e nano -v ; do sleep 1; done
	@make remote-monitor
remote-monitor:
	pio remote device monitor -e nano

build:
	pio run -e $(env) -v

.PHONY: build deploy upload
upload: build
	$(call upload, .pio/build/nano/firmware.hex, /tmp/firmware-$(env).hex)
	$(call command, avrdude -patmega328p -carduino -P/dev/ttyUSB0 -b57600 -Uflash:w:/tmp/firmware-$(env).hex:i )
# pio run -t upload -e $(env)

upload-fan:
	env=nano sumake upload