PROJECT=log_monitor

build:
	go mod tidy
	go build -o ${PROJECT}