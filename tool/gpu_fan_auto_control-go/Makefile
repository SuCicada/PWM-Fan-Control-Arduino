remote_tool_dir=TOOL/gpu_fan_auto_control
remote_bin_dir=bin

upload:
	GOARCH=amd64 GOOS=linux go build -o gpu_fan_auto_control .
	$(call upload, gpu_fan_auto_control, $(remote_tool_dir)/)
	$(call upload, config.yml, $(remote_tool_dir)/)
	$(call upload, gpu_fan_auto_control.sh, $(remote_bin_dir)/gpu_fan_auto_control.sh)
	