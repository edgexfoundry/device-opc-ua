// -*- Mode: Go; indent-tabs-mode: t -*-
//
// Copyright (C) 2026 Encentive Energy
//
// SPDX-License-Identifier: Apache-2.0

package driver

import (
	"context"
	"encoding/base64"
	stderrors "errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/edgexfoundry/go-mod-core-contracts/v4/dtos/common"
	edgexerrors "github.com/edgexfoundry/go-mod-core-contracts/v4/errors"
	"github.com/gopcua/opcua/ua"
	"github.com/labstack/echo/v4"
)

const browseTimeout = 15 * time.Second

func (d *Driver) handleBrowse(c echo.Context) error {
	deviceName := strings.TrimSpace(c.Param("deviceName"))
	if deviceName == "" {
		return writeBrowseError(c, http.StatusBadRequest, "deviceName is required")
	}
	query, err := parseBrowseQuery(c.QueryParams())
	if err != nil {
		return writeBrowseError(c, http.StatusBadRequest, err.Error())
	}
	if d.sdkService == nil {
		return writeBrowseError(c, http.StatusBadGateway, "device service is not initialized")
	}
	device, err := d.sdkService.GetDeviceByName(deviceName)
	if err != nil {
		return writeBrowseError(c, http.StatusNotFound, err.Error())
	}

	parent := context.Background()
	if c.Request() != nil {
		parent = c.Request().Context()
	}
	ctx, cancel := context.WithTimeout(parent, browseTimeout)
	defer cancel()

	client, release, err := d.opcuaClient(ctx, deviceName, device.Protocols)
	if err != nil {
		return writeBrowseError(c, acquireStatus(err), err.Error())
	}
	var browseErr error
	defer func() { release(browseErr) }()

	d.browseMu.Lock()
	defer d.browseMu.Unlock()
	var response browseResponse
	response, browseErr = collectChildren(ctx, client, deviceName, query)
	if browseErr != nil {
		if d.Logger != nil {
			d.Logger.Warnf("browse %s %s: %v", deviceName, query.nodeID, browseErr)
		}
		if isTransportError(browseErr) {
			return writeBrowseError(c, http.StatusBadGateway, browseErr.Error())
		}
		var failure *browseFailure
		if stderrors.As(browseErr, &failure) {
			return writeBrowseError(c, failure.status, failure.msg)
		}
		return writeBrowseError(c, http.StatusBadGateway, browseErr.Error())
	}
	return c.JSON(http.StatusOK, response)
}

func parseBrowseQuery(values url.Values) (browseQuery, error) {
	if _, ok := values["depth"]; ok {
		return browseQuery{}, fmt.Errorf("depth is not accepted; one request returns the direct children of one node")
	}
	nodeID := strings.TrimSpace(values.Get("nodeId"))
	if nodeID == "" {
		nodeID = objectsNodeID
	}
	if _, err := ua.ParseNodeID(nodeID); err != nil {
		return browseQuery{}, fmt.Errorf("nodeId: %w", err)
	}

	maxNodes := defaultMaxNodes
	if raw := strings.TrimSpace(values.Get("maxNodes")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > hardMaxNodes {
			return browseQuery{}, fmt.Errorf("maxNodes must be an integer from 1 to %d", hardMaxNodes)
		}
		maxNodes = parsed
	}

	var classes map[ua.NodeClass]struct{}
	if raw := strings.TrimSpace(values.Get("nodeClass")); raw != "" {
		classes = make(map[ua.NodeClass]struct{})
		for _, part := range strings.Split(raw, ",") {
			part = strings.TrimSpace(part)
			class, ok := parseNodeClassFilter(part)
			if !ok {
				return browseQuery{}, fmt.Errorf("nodeClass %q is not Object, Variable, Method, or View", part)
			}
			classes[class] = struct{}{}
		}
	}

	includeProperties := false
	if _, ok := values["includeProperties"]; ok {
		parsed, err := strconv.ParseBool(strings.TrimSpace(values.Get("includeProperties")))
		if err != nil {
			return browseQuery{}, fmt.Errorf("includeProperties must be true or false")
		}
		includeProperties = parsed
	}

	var continuation []byte
	if raw := strings.TrimSpace(values.Get("continuationToken")); raw != "" {
		decoded, err := base64.StdEncoding.DecodeString(raw)
		if err != nil || len(decoded) == 0 {
			return browseQuery{}, fmt.Errorf("continuationToken is not valid base64")
		}
		continuation = decoded
	}

	return browseQuery{
		nodeID:            nodeID,
		maxNodes:          maxNodes,
		classes:           classes,
		includeProperties: includeProperties,
		continuation:      continuation,
	}, nil
}

func parseNodeClassFilter(value string) (ua.NodeClass, bool) {
	switch value {
	case "Object":
		return ua.NodeClassObject, true
	case "Variable":
		return ua.NodeClassVariable, true
	case "Method":
		return ua.NodeClassMethod, true
	case "View":
		return ua.NodeClassView, true
	default:
		return 0, false
	}
}

func acquireStatus(err error) int {
	switch edgexerrors.Kind(err) {
	case edgexerrors.KindContractInvalid, edgexerrors.KindEntityDoesNotExist:
		return http.StatusNotFound
	}
	message := err.Error()
	if strings.Contains(message, "invalid OPC UA") ||
		strings.Contains(message, "CertFile and KeyFile") ||
		strings.Contains(message, "cannot convert") {
		return http.StatusBadRequest
	}
	return http.StatusBadGateway
}

func writeBrowseError(c echo.Context, status int, message string) error {
	return c.JSON(status, common.NewBaseResponse("", message, status))
}
