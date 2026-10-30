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
	"strconv"

	"github.com/gopcua/opcua/id"
	"github.com/gopcua/opcua/ua"
)

const (
	defaultMaxNodes = 200
	hardMaxNodes    = 1000
	objectsNodeID   = "i=85"
	maxBrowsePages  = 10000

	accessCurrentRead  = 0x01
	accessCurrentWrite = 0x02
)

// nodeBrowser is the OPC UA surface browse uses. *opcua.Client implements it.
type nodeBrowser interface {
	Browse(ctx context.Context, req *ua.BrowseRequest) (*ua.BrowseResponse, error)
	BrowseNext(ctx context.Context, req *ua.BrowseNextRequest) (*ua.BrowseNextResponse, error)
	Read(ctx context.Context, req *ua.ReadRequest) (*ua.ReadResponse, error)
}

type browseQuery struct {
	nodeID            string
	maxNodes          int
	classes           map[ua.NodeClass]struct{}
	includeProperties bool
	continuation      []byte
}

type browseFailure struct {
	status int
	msg    string
}

func (e *browseFailure) Error() string { return e.msg }

func failBrowse(status int, format string, args ...any) error {
	return &browseFailure{status: status, msg: fmt.Sprintf(format, args...)}
}

// browseReference is one direct child of the requested node.
type browseReference struct {
	ParentNodeID   string `json:"parentNodeId"`
	NodeID         string `json:"nodeId"`
	BrowseName     string `json:"browseName"`
	NamespaceURI   string `json:"namespaceUri,omitempty"`
	DisplayName    string `json:"displayName"`
	NodeClass      string `json:"nodeClass"`
	ReferenceType  string `json:"referenceType"`
	TypeDefinition string `json:"typeDefinition,omitempty"`
	DataType       string `json:"dataType,omitempty"`
	DataTypeID     string `json:"dataTypeId,omitempty"`
	ValueRank      *int32 `json:"valueRank,omitempty"`
	AccessLevel    string `json:"accessLevel,omitempty"`
	EdgeXValueType string `json:"edgexValueType,omitempty"`
}

// browseResponse is one level of the address space. It does not contain children of children.
type browseResponse struct {
	DeviceName        string            `json:"deviceName"`
	StartNodeID       string            `json:"startNodeId"`
	Namespaces        []string          `json:"namespaces"`
	Truncated         bool              `json:"truncated"`
	ContinuationToken string            `json:"continuationToken"`
	References        []browseReference `json:"references"`
}

func collectChildren(ctx context.Context, browser nodeBrowser, deviceName string, q browseQuery) (browseResponse, error) {
	nodeID, err := ua.ParseNodeID(q.nodeID)
	if err != nil {
		return browseResponse{}, failBrowse(http.StatusBadRequest, "nodeId: %v", err)
	}
	if q.maxNodes < 1 || q.maxNodes > hardMaxNodes {
		return browseResponse{}, failBrowse(http.StatusBadRequest, "maxNodes must be an integer from 1 to %d", hardMaxNodes)
	}

	var classMask uint32
	for class := range q.classes {
		classMask |= uint32(class)
	}
	if classMask == 0 {
		// A mask of zero means "no class is of interest" on some servers.
		classMask = uint32(ua.NodeClassAll)
	}

	kept := make([]*ua.ReferenceDescription, 0)
	remaining := q.maxNodes
	cp := append([]byte(nil), q.continuation...)
	useNext := len(cp) > 0
	var token []byte
	truncated := false

	for pages := 0; remaining > 0 && pages < maxBrowsePages; pages++ {
		var result *ua.BrowseResult
		if useNext || pages > 0 {
			result, err = browseNextPage(ctx, browser, cp)
			useNext = false
		} else {
			result, err = browseFirstPage(ctx, browser, nodeID, classMask, uint32(remaining))
		}
		if err != nil {
			if result != nil {
				releaseContinuation(ctx, browser, result.ContinuationPoint)
			}
			return browseResponse{}, err
		}

		matching := make([]*ua.ReferenceDescription, 0, len(result.References))
		for _, ref := range result.References {
			if keepReference(ref, q) {
				matching = append(matching, ref)
			}
		}
		if len(matching) > remaining {
			releaseContinuation(ctx, browser, result.ContinuationPoint)
			return browseResponse{}, failBrowse(http.StatusBadGateway, "OPC UA server returned more references than maxNodes")
		}
		kept = append(kept, matching...)
		remaining -= len(matching)
		if len(result.ContinuationPoint) == 0 {
			cp = nil
			break
		}
		if remaining == 0 {
			truncated = true
			token = append([]byte(nil), result.ContinuationPoint...)
			cp = nil
			break
		}
		cp = append([]byte(nil), result.ContinuationPoint...)
	}
	if remaining > 0 && len(cp) > 0 && len(token) == 0 {
		// The page cap stopped the walk while the server still has a continuation point.
		truncated = true
		token = append([]byte(nil), cp...)
	}

	namespaces, err := readNamespaces(ctx, browser)
	if err != nil {
		releaseContinuation(ctx, browser, token)
		return browseResponse{}, err
	}
	refs, err := describeReferences(ctx, browser, nodeID.String(), namespaces, kept)
	if err != nil {
		releaseContinuation(ctx, browser, token)
		return browseResponse{}, err
	}
	if namespaces == nil {
		namespaces = []string{}
	}
	if refs == nil {
		refs = []browseReference{}
	}

	encoded := ""
	if len(token) > 0 {
		encoded = base64.StdEncoding.EncodeToString(token)
	}
	return browseResponse{
		DeviceName:        deviceName,
		StartNodeID:       nodeID.String(),
		Namespaces:        namespaces,
		Truncated:         truncated,
		ContinuationToken: encoded,
		References:        refs,
	}, nil
}

func browseFirstPage(ctx context.Context, browser nodeBrowser, nodeID *ua.NodeID, classMask uint32, limit uint32) (*ua.BrowseResult, error) {
	req := &ua.BrowseRequest{
		RequestedMaxReferencesPerNode: limit,
		NodesToBrowse: []*ua.BrowseDescription{{
			NodeID:          nodeID,
			BrowseDirection: ua.BrowseDirectionForward,
			ReferenceTypeID: ua.NewNumericNodeID(0, id.HierarchicalReferences),
			IncludeSubtypes: true,
			NodeClassMask:   classMask,
			ResultMask:      uint32(ua.BrowseResultMaskAll),
		}},
	}
	resp, err := browser.Browse(ctx, req)
	if err != nil {
		return nil, callFailure(err)
	}
	if resp == nil {
		return nil, failBrowse(http.StatusBadGateway, "OPC UA browse returned no result")
	}
	return takeBrowseResult(resp.Header(), resp.Results)
}

func browseNextPage(ctx context.Context, browser nodeBrowser, cp []byte) (*ua.BrowseResult, error) {
	req := &ua.BrowseNextRequest{
		ContinuationPoints: [][]byte{cp},
	}
	resp, err := browser.BrowseNext(ctx, req)
	if err != nil {
		return nil, callFailure(err)
	}
	if resp == nil {
		return nil, failBrowse(http.StatusBadGateway, "OPC UA browse returned no result")
	}
	return takeBrowseResult(resp.Header(), resp.Results)
}

func takeBrowseResult(header *ua.ResponseHeader, results []*ua.BrowseResult) (*ua.BrowseResult, error) {
	var result *ua.BrowseResult
	if len(results) > 0 {
		result = results[0]
	}
	if header != nil && !isGoodStatus(header.ServiceResult) {
		return result, statusFailure(header.ServiceResult)
	}
	if result == nil {
		return nil, failBrowse(http.StatusBadGateway, "OPC UA browse returned no result")
	}
	if !isGoodStatus(result.StatusCode) {
		return result, statusFailure(result.StatusCode)
	}
	return result, nil
}

func releaseContinuation(ctx context.Context, browser nodeBrowser, cp []byte) {
	if len(cp) == 0 || browser == nil {
		return
	}
	_, _ = browser.BrowseNext(ctx, &ua.BrowseNextRequest{
		ReleaseContinuationPoints: true,
		ContinuationPoints:        [][]byte{cp},
	})
}

func keepReference(ref *ua.ReferenceDescription, q browseQuery) bool {
	if ref == nil {
		return false
	}
	if !q.includeProperties && isHasProperty(ref) {
		return false
	}
	if len(q.classes) > 0 {
		if _, ok := q.classes[ref.NodeClass]; !ok {
			return false
		}
	}
	return true
}

func isHasProperty(ref *ua.ReferenceDescription) bool {
	return ref.ReferenceTypeID != nil &&
		ref.ReferenceTypeID.Namespace() == 0 &&
		ref.ReferenceTypeID.IntID() == id.HasProperty
}

func callFailure(err error) error {
	if err == nil {
		return nil
	}
	var status ua.StatusCode
	if stderrors.As(err, &status) {
		return statusFailure(status)
	}
	return transportFailure(err)
}

func statusFailure(status ua.StatusCode) error {
	switch status {
	case ua.StatusBadContinuationPointInvalid, ua.StatusBadNoContinuationPoints:
		return failBrowse(http.StatusConflict, "%s", status.Error())
	case ua.StatusBadNodeIDInvalid, ua.StatusBadNodeIDUnknown:
		return failBrowse(http.StatusBadRequest, "%s", status.Error())
	default:
		return failBrowse(http.StatusBadGateway, "%s", status.Error())
	}
}

func isGoodStatus(status ua.StatusCode) bool {
	return status&0xC0000000 == 0
}

func readNamespaces(ctx context.Context, browser nodeBrowser) ([]string, error) {
	resp, err := browser.Read(ctx, &ua.ReadRequest{
		TimestampsToReturn: ua.TimestampsToReturnNeither,
		NodesToRead: []*ua.ReadValueID{{
			NodeID:      ua.NewNumericNodeID(0, id.Server_NamespaceArray),
			AttributeID: ua.AttributeIDValue,
		}},
	})
	if err != nil {
		return nil, callFailure(err)
	}
	if resp == nil || resp.Header() != nil && !isGoodStatus(resp.Header().ServiceResult) {
		if resp != nil && resp.Header() != nil && !isGoodStatus(resp.Header().ServiceResult) {
			return nil, statusFailure(resp.Header().ServiceResult)
		}
		return nil, failBrowse(http.StatusBadGateway, "OPC UA namespace read returned no result")
	}
	if len(resp.Results) == 0 || resp.Results[0] == nil || !isGoodStatus(resp.Results[0].Status) || resp.Results[0].Value == nil {
		return []string{}, nil
	}
	switch raw := resp.Results[0].Value.Value().(type) {
	case []string:
		return raw, nil
	case string:
		return []string{raw}, nil
	default:
		return []string{}, nil
	}
}

func describeReferences(ctx context.Context, browser nodeBrowser, parent string, namespaces []string, refs []*ua.ReferenceDescription) ([]browseReference, error) {
	out := make([]browseReference, 0, len(refs))
	variables := make([]int, 0)
	for _, ref := range refs {
		item := browseReference{
			ParentNodeID:   parent,
			NodeID:         expandedNodeID(ref.NodeID),
			BrowseName:     formatBrowseName(ref.BrowseName),
			DisplayName:    formatDisplayName(ref.DisplayName),
			NodeClass:      nodeClassName(ref.NodeClass),
			ReferenceType:  referenceTypeName(ref.ReferenceTypeID),
			TypeDefinition: expandedNodeID(ref.TypeDefinition),
		}
		if ref.BrowseName != nil {
			item.NamespaceURI = namespaceURI(namespaces, ref.BrowseName.NamespaceIndex)
		}
		if ref.NodeClass == ua.NodeClassVariable {
			variables = append(variables, len(out))
		}
		out = append(out, item)
	}
	if len(variables) == 0 {
		return out, nil
	}

	req := &ua.ReadRequest{
		TimestampsToReturn: ua.TimestampsToReturnNeither,
		NodesToRead:        make([]*ua.ReadValueID, 0, len(variables)*3),
	}
	for _, index := range variables {
		nodeID, err := ua.ParseNodeID(out[index].NodeID)
		if err != nil {
			continue
		}
		for _, attr := range []ua.AttributeID{ua.AttributeIDDataType, ua.AttributeIDValueRank, ua.AttributeIDAccessLevel} {
			req.NodesToRead = append(req.NodesToRead, &ua.ReadValueID{NodeID: nodeID, AttributeID: attr})
		}
	}
	if len(req.NodesToRead) == 0 {
		return out, nil
	}
	resp, err := browser.Read(ctx, req)
	if err != nil {
		return nil, callFailure(err)
	}
	if resp == nil || resp.Header() != nil && !isGoodStatus(resp.Header().ServiceResult) {
		if resp != nil && resp.Header() != nil {
			return nil, statusFailure(resp.Header().ServiceResult)
		}
		return nil, failBrowse(http.StatusBadGateway, "OPC UA attribute read returned no result")
	}
	if len(resp.Results) < len(req.NodesToRead) {
		return nil, failBrowse(http.StatusBadGateway, "OPC UA attribute read returned %d values, want %d", len(resp.Results), len(req.NodesToRead))
	}

	valueIndex := 0
	for _, index := range variables {
		if out[index].NodeID == "" {
			continue
		}
		if _, err := ua.ParseNodeID(out[index].NodeID); err != nil {
			continue
		}
		dataType := resp.Results[valueIndex]
		valueRank := resp.Results[valueIndex+1]
		access := resp.Results[valueIndex+2]
		valueIndex += 3
		applyVariableAttributes(&out[index], dataType, valueRank, access)
	}
	return out, nil
}

func applyVariableAttributes(item *browseReference, dataType *ua.DataValue, valueRank *ua.DataValue, access *ua.DataValue) {
	var typeNode *ua.NodeID
	if goodValue(dataType) {
		typeNode = variantNodeID(dataType.Value)
		if typeNode != nil {
			item.DataType = dataTypeName(typeNode)
			item.DataTypeID = typeNode.String()
		}
	}
	rank, hasRank := int32Value(valueRank)
	if hasRank {
		item.ValueRank = &rank
	}
	if goodValue(access) && access.Value != nil && access.Value.Type() == ua.TypeIDByte {
		bits := access.Value.Uint()
		if bits <= 0xff {
			item.AccessLevel = accessLevelString(byte(bits))
		}
	}
	if hasRank {
		item.EdgeXValueType = edgexValueType(typeNode, rank)
	}
}

func goodValue(value *ua.DataValue) bool {
	return value != nil && isGoodStatus(value.Status) && value.Value != nil
}

func variantNodeID(value *ua.Variant) *ua.NodeID {
	if value == nil {
		return nil
	}
	if nodeID := value.NodeID(); nodeID != nil {
		return nodeID
	}
	if expanded := value.ExpandedNodeID(); expanded != nil {
		return expanded.NodeID
	}
	return nil
}

func int32Value(value *ua.DataValue) (int32, bool) {
	if !goodValue(value) {
		return 0, false
	}
	switch value.Value.Type() {
	case ua.TypeIDSByte, ua.TypeIDInt16, ua.TypeIDInt32, ua.TypeIDInt64:
	default:
		return 0, false
	}
	n := value.Value.Int()
	if n > 1<<31-1 || n < -1<<31 {
		return 0, false
	}
	return int32(n), true
}

func dataTypeName(nodeID *ua.NodeID) string {
	if nodeID == nil {
		return ""
	}
	if nodeID.Namespace() == 0 {
		name := id.Name(nodeID.IntID())
		if name != strconv.FormatUint(uint64(nodeID.IntID()), 10) {
			return name
		}
	}
	return nodeID.String()
}

func edgexValueType(typeNode *ua.NodeID, valueRank int32) string {
	if valueRank >= 0 || typeNode == nil || typeNode.Namespace() != 0 {
		return ""
	}
	switch typeNode.IntID() {
	case id.Boolean:
		return "Bool"
	case id.SByte:
		return "Int8"
	case id.Byte:
		return "Uint8"
	case id.Int16:
		return "Int16"
	case id.UInt16:
		return "Uint16"
	case id.Int32:
		return "Int32"
	case id.UInt32:
		return "Uint32"
	case id.Int64:
		return "Int64"
	case id.UInt64:
		return "Uint64"
	case id.Float:
		return "Float32"
	case id.Double:
		return "Float64"
	case id.String:
		return "String"
	case id.ByteString:
		return "Binary"
	default:
		return ""
	}
}

func accessLevelString(bits byte) string {
	read := bits&accessCurrentRead != 0
	write := bits&accessCurrentWrite != 0
	switch {
	case read && write:
		return "RW"
	case read:
		return "R"
	case write:
		return "W"
	default:
		return ""
	}
}

func expandedNodeID(nodeID *ua.ExpandedNodeID) string {
	if nodeID == nil || nodeID.NodeID == nil {
		return ""
	}
	return nodeID.NodeID.String()
}

func formatBrowseName(name *ua.QualifiedName) string {
	if name == nil {
		return ""
	}
	return fmt.Sprintf("%d:%s", name.NamespaceIndex, name.Name)
}

func formatDisplayName(text *ua.LocalizedText) string {
	if text == nil {
		return ""
	}
	return text.Text
}

func namespaceURI(namespaces []string, index uint16) string {
	if int(index) < len(namespaces) {
		return namespaces[index]
	}
	return ""
}

func nodeClassName(class ua.NodeClass) string {
	switch class {
	case ua.NodeClassObject:
		return "Object"
	case ua.NodeClassVariable:
		return "Variable"
	case ua.NodeClassMethod:
		return "Method"
	case ua.NodeClassView:
		return "View"
	case ua.NodeClassObjectType:
		return "ObjectType"
	case ua.NodeClassVariableType:
		return "VariableType"
	case ua.NodeClassDataType:
		return "DataType"
	case ua.NodeClassReferenceType:
		return "ReferenceType"
	default:
		return class.String()
	}
}

func referenceTypeName(nodeID *ua.NodeID) string {
	if nodeID == nil {
		return ""
	}
	if nodeID.Namespace() == 0 {
		name := id.Name(nodeID.IntID())
		if name != strconv.FormatUint(uint64(nodeID.IntID()), 10) {
			return name
		}
	}
	return nodeID.String()
}
