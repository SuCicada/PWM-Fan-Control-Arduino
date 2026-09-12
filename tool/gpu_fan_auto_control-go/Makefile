remote_tool_dir=TOOL/gpu_fan_auto_control

build:
	GOARCH=amd64 GOOS=linux go build -o gpu_fan_auto_control -ldflags "-s -w" -v .
	ls -lh gpu_fan_auto_control
	mkdir build
	cp gpu_fan_auto_control gpu_fan_auto_control.sh build/

# 	$(call upload, config.yml, $(remote_tool_dir)/)
upload: build
	scp build/* server.sucicada.me:$(remote_tool_dir)/