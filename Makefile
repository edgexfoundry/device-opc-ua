.PHONY: build test unittest lint clean docker run

# change the following boolean flag to enable or disable the Full RELRO (RELocation Read Only) for linux ELF (Executable and Linkable Format) binaries
ENABLE_FULL_RELRO=true
# change the following boolean flag to enable or disable PIE for linux binaries which is needed for ASLR (Address Space Layout Randomization) on Linux, the ASLR support on Windows is enabled by default
ENABLE_PIE=true

GO=go
CGO=CGO_ENABLED=1 GO111MODULE=on $(GO)

MICROSERVICES=cmd/device-opcua

.PHONY: $(MICROSERVICES)

ARCH=$(shell uname -m)

VERSION=$(shell cat ./VERSION 2>/dev/null || echo 0.0.0)
SDKVERSION=$(shell cat ./go.mod | grep 'github.com/edgexfoundry/device-sdk-go/v4 v' | sed 's/require//g' | awk '{print $$2}')

DOCKER_TAG=$(VERSION)-dev

ifeq ($(ENABLE_FULL_RELRO), true)
	ENABLE_FULL_RELRO_GOFLAGS = -bindnow
endif

GOFLAGS=-ldflags "-X github.com/edgexfoundry/device-opc-ua.Version=$(VERSION) \
                  -X github.com/edgexfoundry/device-sdk-go/v4/internal/common.SDKVersion=$(SDKVERSION) \
                  $(ENABLE_FULL_RELRO_GOFLAGS)" \
                   -trimpath -mod=readonly
GOTESTFLAGS?=-race

GIT_SHA=$(shell git rev-parse HEAD)

TEST_OUT=test-artifacts

ifeq ($(ENABLE_PIE), true)
	GOFLAGS += -buildmode=pie
endif

build: $(MICROSERVICES)
	$(CGO) install -tags=safe

cmd/device-opcua:
	$(CGO) build $(GOFLAGS) -o $@ ./cmd

build-nats:
	make -e ADD_BUILD_TAGS=include_nats_messaging build

tidy:
	go mod tidy

unittest:
	go test ./... -coverprofile=coverage.out

lint:
	@which golangci-lint >/dev/null || echo "WARNING: go linter not installed. To install, run make install-lint"
	@if [ "z${ARCH}" = "zx86_64" ] && which golangci-lint >/dev/null ; then golangci-lint run --config .golangci.yml ; else echo "WARNING: Linting skipped (not on x86_64 or linter not installed)"; fi

install-lint:
	sudo curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $$(go env GOPATH)/bin v2.5.0

test: unittest lint
	go vet ./...
	gofmt -l $$(find . -type f -name '*.go'| grep -v "/vendor/")
	[ "`gofmt -l $$(find . -type f -name '*.go'| grep -v "/vendor/")`" = "" ]
	./bin/test-attribution-txt.sh

clean:
	rm -f $(MICROSERVICES)

docker:
	docker build \
		-f Dockerfile \
		--label "git_sha=$(GIT_SHA)" \
		-t edgexfoundry/device-opcua:$(GIT_SHA) \
		-t edgexfoundry/device-opcua:$(DOCKER_TAG) \
		.

run:
	cd bin && ./edgex-launch.sh

vendor:
	CGO_ENABLED=0 go mod vendor
