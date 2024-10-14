.PHONY: build test clean docker run

# change the following boolean flag to enable or disable the Full RELRO (RELocation Read Only) for linux ELF (Executable and Linkable Format) binaries
ENABLE_FULL_RELRO:="true"
# change the following boolean flag to enable or disable PIE for linux binaries which is needed for ASLR (Address Space Layout Randomization) on Linux, the ASLR support on Windows is enabled by default
ENABLE_PIE:="true"

GO=go
CGO=CGO_ENABLED=1 GO111MODULE=on $(GO)

MICROSERVICES=cmd/device-opcua

.PHONY: $(MICROSERVICES)

VERSION=$(shell cat ./VERSION 2>/dev/null || echo 0.0.0)
SDKVERSION=$(shell cat ./go.mod | grep 'github.com/edgexfoundry/device-sdk-go/v3 v' | sed 's/require//g' | awk '{print $$2}')

DOCKER_TAG=$(VERSION)-dev

GOFLAGS=-ldflags "-X github.com/edgexfoundry/device-opc-ua.Version=$(VERSION) \
                  -X github.com/edgexfoundry/device-sdk-go/v3/internal/common.SDKVersion=$(SDKVERSION)" \
                   -trimpath -mod=readonly
GOTESTFLAGS?=-race

GIT_SHA=$(shell git rev-parse HEAD)

TEST_OUT=test-artifacts

ifeq ($(ENABLE_FULL_RELRO), "true")
	GOFLAGS += -ldflags "-bindnow"
endif

ifeq ($(ENABLE_PIE), "true")
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

test:
	$(GO) install github.com/jstemmer/go-junit-report@v0.9.1
	$(GO) install github.com/axw/gocov/gocov@v1.0.0
	$(GO) install github.com/AlekSi/gocov-xml@v1.0.0
	$(GO) install github.com/jandelgado/gcov2lcov@v1.0.5
	rm -rf $(TEST_OUT)
	mkdir $(TEST_OUT)
	$(GO) test -v ./... -coverprofile=$(TEST_OUT)/cover.out | go-junit-report > $(TEST_OUT)/report.xml
	gocov convert $(TEST_OUT)/cover.out | gocov-xml > $(TEST_OUT)/coverage.xml
	gcov2lcov -infile=$(TEST_OUT)/cover.out -outfile=$(TEST_OUT)/coverage.lcov

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
