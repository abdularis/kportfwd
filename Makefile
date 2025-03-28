GO_IMG:="golang:1.22"

_GOPATH:=$(shell which go > /dev/null && go env GOPATH || echo "/project/.cache/go-build")
_GOCACHE:=$(shell which go > /dev/null && go env GOCACHE || echo "/project/.cache/mod")
APP_NAME:=kportfwd
MOD_NAME:=github.com/abdularis/kportfwd

.PHONY: build

build-forwarder-agent-go-linux:
	docker run --rm -v ./:/project -w /project\
		-e GOOS=linux\
		-e GOARCH=amd64\
		$(GO_IMG) go build -ldflags "-w -s" -gcflags="all=-l" -o internal/config/build/forwarder-agent-linux-amd64 $(MOD_NAME)/cmd/forwarder-agent
	docker run --rm -v ./:/project -w /project\
		alpine:3.18 sh -c "apk add upx && upx --best --ultra-brute internal/config/build/forwarder-agent-linux-amd64"
	cd internal/config/build && md5sum forwarder-agent-linux-amd64 > forwarder-agent-linux-amd64.md5sum

build-macos:
	@make build GOOS=darwin GOARCH=arm64

build-linux:
	@make build GOOS=linux GOARCH=amd64

build:
	docker run --rm -v ./:/project -w /project \
		-v $(_GOCACHE):/var/caches \
		-v $(_GOPATH):/opt/go \
		-e GOCACHE=/var/caches \
		-e GOPATH=/opt/go \
		-e GOOS=$(GOOS) \
		-e GOARCH=$(GOARCH) \
		-e GOFLAGS="-buildvcs=false" \
		$(GO_IMG) go build -ldflags "-w -s" -gcflags="all=-l" -o build/$(APP_NAME)-$(GOOS)-$(GOARCH) $(MOD_NAME)/cmd/portfwd

install:
	@@which go > /dev/null 2>&1 || { echo "Go is not installed"; exit 1; }
	@make build GOOS=$(GOOS) GOARCH=$(GOARCH)
	@cp build/$(APP_NAME)-$(GOOS)-$(GOARCH) $$(go env GOPATH)/bin/$(APP_NAME)
	@echo "$(APP_NAME) is installed on $$(go env GOPATH)/bin/ (make sure directory is in you PATH)"

install-macos:
	@make install GOOS=darwin GOARCH=arm64