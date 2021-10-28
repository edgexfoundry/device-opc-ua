.PHONY: build test clean docker run

GO=go
CGO=CGO_ENABLED=1 GO111MODULE=on $(GO)

MICROSERVICES=cmd/device-opcua

.PHONY: $(MICROSERVICES)

VERSION=$(shell cat ./VERSION)

GOFLAGS=-ldflags "-X github.com/edgexfoundry/device-opcua-go.Version=$(VERSION)"

GIT_SHA=$(shell git rev-parse HEAD)

TEST_OUT=test-results

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
	rm $(TEST_OUT)/cover.out

clean:
	rm -f $(MICROSERVICES)

docker:
	docker build \
		--label "git_sha=$(GIT_SHA)" \
		-t edgexfoundry/device-opcua-go:$(VERSION)-dev \
		.

run:
	cd bin && ./edgex-launch.sh
