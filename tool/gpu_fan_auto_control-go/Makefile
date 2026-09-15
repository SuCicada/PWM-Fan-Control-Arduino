remote_tool_dir=TOOL/gpu_fan_auto_control

build:
	GOARCH=amd64 GOOS=linux go build -o gpu_fan_auto_control -ldflags "-s -w" -v .
	ls -lh gpu_fan_auto_control
	mkdir -p output
	cp gpu_fan_auto_control gpu_fan_auto_control.sh output/

# 	$(call upload, config.yml, $(remote_tool_dir)/)
upload: build
	scp output/* server.sucicada.me:$(remote_tool_dir)/