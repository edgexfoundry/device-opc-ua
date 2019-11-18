#
# Copyright (c) 2018, 2019 Intel
#
# SPDX-License-Identifier: Apache-2.0
#
FROM golang:1.11-alpine AS builder
WORKDIR /go/src/github.com/edgexfoundry/device-opcua-go

# Replicate the APK repository override.
RUN sed -e 's/dl-cdn[.]alpinelinux.org/mirrors.ustc.edu.cn/g' -i~ /etc/apk/repositories

# Install our build time packages.
RUN apk update && apk add make git

COPY . .

RUN make build

# Next image - Copy built Go binary into new workspace
FROM scratch

# expose command data port
EXPOSE 49997

COPY --from=builder /go/src/github.com/edgexfoundry/device-opcua-go/cmd /

ENTRYPOINT ["/device-opcua","--profile=docker","--confdir=/res","--registry=consul://edgex-core-consul:8500"]
