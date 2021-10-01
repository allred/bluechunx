#include .env
# https://ieftimov.com/post/golang-package-multiple-binaries/
# https://github.com/gobuffalo/packr/blob/master/packr/main.go
# checkout godag?
GOROOT = .
BLUECHUNX_BIN = "./bin"
BLUECHUNX_CMD = "./cmd"


build:
	go build -v -o $(BLUECHUNX_BIN)/bluechunx $(BLUECHUNX_CMD)/bluechunx

run:
	./bin/bluechunx

test:
	go test
