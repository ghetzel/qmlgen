.EXPORT_ALL_VARIABLES:

GO111MODULE ?= on
BIN         ?= bin/hydra-$(shell go env GOOS)-$(shell go env GOARCH)
CGO_ENABLED ?= 0

.PHONY: fmt deps test $(BIN)

all: deps fmt $(BIN)

fmt:
	go fmt ./...

deps:
	go get ./...
	go mod tidy

test:
	go test ./...

$(BIN):
	go build -o $(@) cmd/hydra/*.go
	-which hydra && $(@) -v && cp $(@) $(shell which hydra)

run:
	# -rm -rf build
	# mkdir build
	$(BIN)
	cd build && qmlscene app.qml

arm:
	GOARCH=arm go build -o bin/hydra-linux-arm cmd/hydra/*.go
	cp bin/hydra-linux-arm ~/bin/hydra-linux-arm
