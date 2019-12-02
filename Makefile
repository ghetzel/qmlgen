.PHONY: fmt deps test bin/qmlgen app.qml
.EXPORT_ALL_VARIABLES:

GO111MODULE ?= on

all: deps fmt bin/qmlgen

fmt:
	go fmt ./...

deps:
	go get ./...
	go mod tidy

test:
	go test ./...

bin/qmlgen:
	go build -o $(@) cmd/*.go

run:
	./bin/qmlgen
	cd build && qmlscene root.qml