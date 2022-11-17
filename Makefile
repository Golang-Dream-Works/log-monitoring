default: help

# variable collection
# used way: only adjust 【PROJECT】and 【DOCKER_REGISTRY】
PROJECT = log-minitoring
DOCKER_REGISTRY=registry.iso.com:8150
GO_LDFLAGS = -ldflags " -w"
VERSION = $(shell date -u +v%Y%m%d)-$(shell git describe --tags --always)
BIN_LABELS = ${PROJECT}_$(VERSION) 
WIN_LABELS = ${PROJECT}_$(VERSION).exe
DOCKER_IMAGE_NAME = ${DOCKER_REGISTRY}/${PROJECT}:$(VERSION)
DOCKER_REMOVE_IMAGE_NAME = ${DOCKER_REGISTRY}/${PROJECT}:latest

build-lux:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build ${GO_LDFLAGS} -o ${BIN_LABELS} main.go

build-mac: 
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build ${GO_LDFLAGS} -o ${BIN_LABELS} main.go

build-win: 
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build ${GO_LDFLAGS} -o ${WIN_LABELS} main.go

build-img:
	docker build . -f Dockerfile -t ${DOCKER_IMAGE_NAME} --build-arg PROJECT=${PROJECT} --build-arg BIN_LABELS=${BIN_LABELS} 
	docker tag ${DOCKER_IMAGE_NAME} ${DOCKER_REMOVE_IMAGE_NAME}

docker-push: 
	docker push ${DOCKER_REMOVE_IMAGE_NAME}
	docker push ${DOCKER_IMAGE_NAME} 


build-all: build-lux build-mac build-win build-img 

help:
	@echo ""
	@echo "Build Usage:" 
	@echo "\033[32m    build-lux       \033[0m" "\033[36m to build binary program under Linux platform.   \033[0m"
	@echo "\033[32m    build-mac       \033[0m" "\033[36m to build binary program under Mac OS platform.  \033[0m"
	@echo "\033[32m    build-win       \033[0m" "\033[36m to build binary program under Windows platform. \033[0m"
	@echo "\033[32m    build-img       \033[0m" "\033[36m to build binary program under docker platform. \033[0m"
	@echo "\033[32m    docker-push     \033[0m" "\033[36m to push docker images for docker registry.  \033[0m"
	@echo "\033[32m    build-all       \033[0m" "\033[36m to build binary program under all platform.  \033[0m"
	@echo "\033[32m    help            \033[0m" "\033[36m to show how to execute binary program.  \033[0m"
	@echo ""