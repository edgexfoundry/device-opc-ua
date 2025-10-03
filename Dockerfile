#
# Copyright (c) 2018, 2019 Intel
# Copyright (c) 2021 Schneider Electric
# Copyright (C) 2023 YIQISOFT
# Copyright (C) 2024-2025 IOTech Ltd
#
# SPDX-License-Identifier: Apache-2.0
#

ARG BASE=golang:1.23-alpine3.20
FROM ${BASE} AS builder

ARG ADD_BUILD_TAGS=""
ARG MAKE="make -e ADD_BUILD_TAGS=$ADD_BUILD_TAGS build"

RUN apk add --update --no-cache make git zeromq-dev gcc pkgconfig musl-dev

# set the working directory
WORKDIR /device-opcua

COPY go.mod vendor* ./
RUN [ ! -d "vendor" ] && go mod download all || echo "skipping..."

COPY . .
RUN ${MAKE}

# Next image - Copy built Go binary into new workspace
FROM alpine:3.22.1

LABEL license='SPDX-License-Identifier: Apache-2.0' \
      copyright='Copyright (c) 2019-2025: IoTech Ltd'

# dumb-init needed for injected secure bootstrapping entrypoint script when run in secure mode.
RUN apk add --update --no-cache zeromq dumb-init
# Ensure using latest versions of all installed packages to avoid any recent CVEs
RUN apk --no-cache upgrade

COPY --from=builder /device-opcua/cmd /
COPY --from=builder /device-opcua/LICENSE /
COPY --from=builder /device-opcua/Attribution.txt /

EXPOSE 59997

ENTRYPOINT ["/device-opcua"]
CMD ["-cp=keeper.http://edgex-core-keeper:59890", "--registry"]
