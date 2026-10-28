deploy:
	cd ../../../SuConfig/linux/asus/ && \
	make docker-gpu-fan-manager

build:
	GOARCH=amd64 GOOS=linux go build -o gpu_fan_auto_control -ldflags "-s -w" -v .
	ls -lh gpu_fan_auto_control
