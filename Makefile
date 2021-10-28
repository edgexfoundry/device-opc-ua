.PHONY: build test clean docker run

GO=go
CGO=CGO_ENABLED=1 GO111MODULE=on $(GO)

MICROSERVICES=cmd/device-opcua

.PHONY: $(MICROSERVICES)

VERSION=$(shell cat ./VERSION 2>/dev/null || echo 0.0.0)

GOFLAGS=-ldflags "-X github.com/edgexfoundry/device-opcua-go.Version=$(VERSION)"

GIT_SHA=$(shell git rev-parse HEAD)

TEST_OUT=test-artifacts

build: $(MICROSERVICES)
	$(CGO) install -tags=safe

cmd/device-opcua:
	$(CGO) build $(GOFLAGS) -o $@ ./cmd

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
		--label "git_sha=$(GIT_SHA)" \
		-t edgexfoundry/device-opcua-go:$(VERSION)-dev \
		.

run:
	cd bin && ./edgex-launch.sh
