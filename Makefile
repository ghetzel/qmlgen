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
	go build -o $(@) cmd/*.go

pan:
	scp clock.yaml pan:app.yaml
	scp bin/qmlgen-linux-arm pan:bin/qmlgen
	ssh pan "rm -rf build; systemctl --user restart ui"

run:
	# -rm -rf build
	# mkdir build
	$(BIN)
	cd build && qmlscene app.qml