.PHONY: build test clean docker run

GO=CGO_ENABLED=1 GO111MODULE=on go

MICROSERVICES=cmd/device-opcua

.PHONY: $(MICROSERVICES)

VERSION=$(shell cat ./VERSION)

GOFLAGS=-ldflags "-X github.com/edgexfoundry/device-opcua-go.Version=$(VERSION)"

GIT_SHA=$(shell git rev-parse HEAD)

build: $(MICROSERVICES)
	$(GO) install -tags=safe

cmd/device-opcua:
	$(GO) build $(GOFLAGS) -o $@ ./cmd

test:
	go test ./... -cover

clean:
	rm -f $(MICROSERVICES)

docker:
	docker build \
		--label "git_sha=$(GIT_SHA)" \
		-t edgexfoundry/device-opcua-go:$(VERSION)-dev \
		.

run:
	cd bin && ./edgex-launch.sh
