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
	-which qmlgen && $(@) -v && cp $(@) $(shell which qmlgen)

run:
	# -rm -rf build
	# mkdir build
	$(BIN)
	cd build && qmlscene app.qml

arm:
	GOARCH=arm go build -o bin/qmlgen-linux-arm cmd/qmlgen/*.go
	cp bin/qmlgen-linux-arm ~/bin/qmlgen-linux-arm
