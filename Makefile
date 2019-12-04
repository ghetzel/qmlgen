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

pan-build:
	ssh pan "sudo systemctl stop ui"
	ssh pan "test -d /opt/pan-ui/bin || sudo mkdir -p /opt/pan-ui/bin"
	scp bin/qmlgen-linux-arm pan:/tmp/qmlgen
	ssh pan "sudo mv /tmp/qmlgen /opt/pan-ui/bin/qmlgen"

pan-update:
	scp pan.yaml pan:/tmp/app.yaml
	ssh pan "sudo mv /tmp/app.yaml /opt/pan-ui/app.yaml"

pan-enforce:
	ssh pan "\
		sudo chown -R root.sudo /opt/pan-ui; \
		sudo chmod -R 0755 /opt/pan-ui; \
		sudo chmod 0644 /opt/pan-ui/*.yaml; \
		sudo rm -rf /opt/pan-ui/build; \
		sudo systemctl restart ui \
	"

pan: pan-build pan-update pan-enforce

pan-push: pan-update pan-enforce

run:
	# -rm -rf build
	# mkdir build
	$(BIN)
	cd build && qmlscene app.qml