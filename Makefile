.EXPORT_ALL_VARIABLES:

CGO_ENABLED  = 1
CGO_CFLAGS   = -I/opt/homebrew/include -arch x86_64
CGO_LDFLAGS  = -L/opt/homebrew/lib
BUNDLES     += default.zip
BUNDLES     += dockmaster.zip
GOOS         = darwin
GOARCH       = amd64

build: bin/hydra

deps:
	@go get

fmt:
	@gofmt -w .
	@go generate -x

bin/hydra: deps fmt
	@go build -o $(@)

bundles: $(BUNDLES)
$(BUNDLES):
	-rm $(@)
	@cd $(PWD)/contrib/bundles/$(basename $(@)) && zip -r ../../../$(@) .

.PHONY: build bin/hydra $(BUNDLES)
