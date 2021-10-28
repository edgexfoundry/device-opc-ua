#
# Copyright (c) 2018, 2019 Intel
#
# SPDX-License-Identifier: Apache-2.0
#
FROM golang:1.16-alpine3.12 AS builder
WORKDIR /device-opcua-go

# Install our build time packages.
RUN apk update && apk add --no-cache make git zeromq-dev gcc pkgconfig musl-dev

COPY . .

RUN make build

# Next image - Copy built Go binary into new workspace
FROM alpine:3.12

# dumb-init needed for injected secure bootstrapping entrypoint script when run in secure mode.
RUN apk add --update --no-cache zeromq dumb-init

# expose command data port
EXPOSE 59997

COPY --from=builder /device-opcua-go/cmd/device-opcua /
COPY --from=builder /device-opcua-go/cmd/res /res
COPY LICENSE /

ENTRYPOINT ["/device-opcua"]
CMD ["--cp=consul://edgex-core-consul:8500", "--registry", "--confdir=/res"]
