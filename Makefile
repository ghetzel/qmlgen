.EXPORT_ALL_VARIABLES:

GO111MODULE ?= on
BIN         ?= bin/qmlgen-$(shell go env GOOS)-$(shell go env GOARCH)

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
	go build -o $(@) cmd/qmlgen/*.go

run:
	# -rm -rf build
	# mkdir build
	$(BIN)
	cd build && qmlscene app.qml
