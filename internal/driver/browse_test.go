// -*- Mode: Go; indent-tabs-mode: t -*-
//
// Copyright (C) 2026 Encentive Energy
//
// SPDX-License-Identifier: Apache-2.0

package driver

import (
	"context"
	"encoding/base64"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"

	sdkmocks "github.com/edgexfoundry/device-sdk-go/v4/pkg/interfaces/mocks"
	"github.com/edgexfoundry/go-mod-core-contracts/v4/models"
	"github.com/gopcua/opcua"
	"github.com/gopcua/opcua/id"
	"github.com/gopcua/opcua/ua"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
)

type scriptedBrowser struct {
	browseFn func(*ua.BrowseRequest) (*ua.BrowseResponse, error)
	nextFn   func(*ua.BrowseNextRequest) (*ua.BrowseNextResponse, error)
	readFn   func(*ua.ReadRequest) (*ua.ReadResponse, error)
}

func (s *scriptedBrowser) Browse(_ context.Context, req *ua.BrowseRequest) (*ua.BrowseResponse, error) {
	return s.browseFn(req)
}

func (s *scriptedBrowser) BrowseNext(_ context.Context, req *ua.BrowseNextRequest) (*ua.BrowseNextResponse, error) {
	if s.nextFn == nil {
		return nil, fmt.Errorf("unexpected BrowseNext")
	}
	return s.nextFn(req)
}

func (s *scriptedBrowser) Read(_ context.Context, req *ua.ReadRequest) (*ua.ReadResponse, error) {
	if s.readFn == nil {
		return nil, fmt.Errorf("unexpected Read")
	}
	return s.readFn(req)
}

func dataValue(v interface{}) *ua.DataValue {
	return &ua.DataValue{
		EncodingMask: ua.DataValueValue | ua.DataValueStatusCode,
		Value:        ua.MustVariant(v),
		Status:       ua.StatusOK,
	}
}

func testReference(refType uint32, nodeID string, ns uint16, browseName, display string, class ua.NodeClass, typeDef string) *ua.ReferenceDescription {
	parsed, err := ua.ParseNodeID(nodeID)
	if err != nil {
		panic(err)
	}
	var typeNode *ua.ExpandedNodeID
	if typeDef != "" {
		typeID, err := ua.ParseNodeID(typeDef)
		if err != nil {
			panic(err)
		}
		typeNode = ua.NewExpandedNodeID(typeID, "", 0)
	}
	return &ua.ReferenceDescription{
		ReferenceTypeID: ua.NewNumericNodeID(0, refType),
		IsForward:       true,
		NodeID:          ua.NewExpandedNodeID(parsed, "", 0),
		BrowseName:      &ua.QualifiedName{NamespaceIndex: ns, Name: browseName},
		DisplayName:     ua.NewLocalizedText(display),
		NodeClass:       class,
		TypeDefinition:  typeNode,
	}
}

func namespaceResponse() *ua.ReadResponse {
	return &ua.ReadResponse{Results: []*ua.DataValue{dataValue([]string{
		"http://opcfoundation.org/UA/",
		"urn:site:unused",
		"urn:site:plc",
	})}}
}

func attributeResponse(req *ua.ReadRequest, values []*ua.DataValue) *ua.ReadResponse {
	out := make([]*ua.DataValue, 0, len(req.NodesToRead))
	for i := 0; i < len(req.NodesToRead); i += 3 {
		out = append(out, values...)
	}
	return &ua.ReadResponse{Results: out}
}

func floatAttributes() []*ua.DataValue {
	return []*ua.DataValue{
		dataValue(ua.NewNumericNodeID(0, id.Float)),
		dataValue(int32(-1)),
		dataValue(byte(accessCurrentRead)),
	}
}

func TestEdgeXValueTypeScalars(t *testing.T) {
	cases := []struct {
		id   uint32
		want string
	}{
		{id.Boolean, "Bool"},
		{id.SByte, "Int8"},
		{id.Byte, "Uint8"},
		{id.Int16, "Int16"},
		{id.UInt16, "Uint16"},
		{id.Int32, "Int32"},
		{id.UInt32, "Uint32"},
		{id.Int64, "Int64"},
		{id.UInt64, "Uint64"},
		{id.Float, "Float32"},
		{id.Double, "Float64"},
		{id.String, "String"},
		{id.ByteString, "Binary"},
		{id.DateTime, ""},
		{id.Structure, ""},
		{id.Enumeration, ""},
	}
	for _, tc := range cases {
		got := edgexValueType(ua.NewNumericNodeID(0, tc.id), -1)
		require.Equal(t, tc.want, got, id.Name(tc.id))
	}
	require.Empty(t, edgexValueType(ua.NewNumericNodeID(0, id.Float), 1))
	require.Empty(t, edgexValueType(ua.NewNumericNodeID(2, id.Float), -1))
}

func TestCollectChildrenOneLevel(t *testing.T) {
	object := testReference(id.Organizes, "ns=2;s=EMS", 2, "EMS", "EMS", ua.NodeClassObject, "i=58")
	variable := testReference(id.HasComponent, "ns=2;s=Temperature", 2, "Temperature", "Temperature", ua.NodeClassVariable, "i=63")
	// A grandchild is not part of this response. The fake only returns the requested level.
	browser := &scriptedBrowser{
		browseFn: func(req *ua.BrowseRequest) (*ua.BrowseResponse, error) {
			require.Equal(t, uint32(200), req.RequestedMaxReferencesPerNode)
			require.Equal(t, "ns=2;s=DeviceSet", req.NodesToBrowse[0].NodeID.String())
			require.Equal(t, uint32(id.HierarchicalReferences), req.NodesToBrowse[0].ReferenceTypeID.IntID())
			require.True(t, req.NodesToBrowse[0].IncludeSubtypes)
			require.Equal(t, ua.BrowseDirectionForward, req.NodesToBrowse[0].BrowseDirection)
			require.Equal(t, uint32(ua.NodeClassAll), req.NodesToBrowse[0].NodeClassMask)
			return &ua.BrowseResponse{Results: []*ua.BrowseResult{{
				StatusCode: ua.StatusOK,
				References: []*ua.ReferenceDescription{object, variable},
			}}}, nil
		},
		readFn: func(req *ua.ReadRequest) (*ua.ReadResponse, error) {
			if len(req.NodesToRead) == 1 {
				return namespaceResponse(), nil
			}
			require.Equal(t, "ns=2;s=Temperature", req.NodesToRead[0].NodeID.String())
			return attributeResponse(req, floatAttributes()), nil
		},
	}

	got, err := collectChildren(context.Background(), browser, "plc-1", browseQuery{
		nodeID:   "ns=2;s=DeviceSet",
		maxNodes: defaultMaxNodes,
	})
	require.NoError(t, err)
	require.Equal(t, "plc-1", got.DeviceName)
	require.Equal(t, "ns=2;s=DeviceSet", got.StartNodeID)
	require.Equal(t, []string{"http://opcfoundation.org/UA/", "urn:site:unused", "urn:site:plc"}, got.Namespaces)
	require.False(t, got.Truncated)
	require.Empty(t, got.ContinuationToken)
	require.Len(t, got.References, 2)

	require.Equal(t, "Object", got.References[0].NodeClass)
	require.Equal(t, "ns=2;s=DeviceSet", got.References[0].ParentNodeID)
	require.Equal(t, "ns=2;s=EMS", got.References[0].NodeID)
	require.Equal(t, "2:EMS", got.References[0].BrowseName)
	require.Equal(t, "urn:site:plc", got.References[0].NamespaceURI)
	require.Equal(t, "Organizes", got.References[0].ReferenceType)
	require.Empty(t, got.References[0].EdgeXValueType)

	require.Equal(t, "Variable", got.References[1].NodeClass)
	require.Equal(t, "Float", got.References[1].DataType)
	require.Equal(t, "i=10", got.References[1].DataTypeID)
	require.NotNil(t, got.References[1].ValueRank)
	require.Equal(t, int32(-1), *got.References[1].ValueRank)
	require.Equal(t, "R", got.References[1].AccessLevel)
	require.Equal(t, "Float32", got.References[1].EdgeXValueType)
}

func TestCollectChildrenTruncatesAndContinues(t *testing.T) {
	first := testReference(id.Organizes, "ns=2;s=EMS", 2, "EMS", "EMS", ua.NodeClassObject, "i=58")
	second := testReference(id.Organizes, "ns=2;s=Server", 0, "Server", "Server", ua.NodeClassObject, "i=58")
	browser := &scriptedBrowser{
		browseFn: func(req *ua.BrowseRequest) (*ua.BrowseResponse, error) {
			require.Equal(t, uint32(1), req.RequestedMaxReferencesPerNode)
			return &ua.BrowseResponse{Results: []*ua.BrowseResult{{
				StatusCode:        ua.StatusOK,
				ContinuationPoint: []byte("page-2"),
				References:        []*ua.ReferenceDescription{first},
			}}}, nil
		},
		nextFn: func(req *ua.BrowseNextRequest) (*ua.BrowseNextResponse, error) {
			require.Equal(t, [][]byte{[]byte("page-2")}, req.ContinuationPoints)
			require.False(t, req.ReleaseContinuationPoints)
			return &ua.BrowseNextResponse{Results: []*ua.BrowseResult{{
				StatusCode: ua.StatusOK,
				References: []*ua.ReferenceDescription{second},
			}}}, nil
		},
		readFn: func(req *ua.ReadRequest) (*ua.ReadResponse, error) {
			return namespaceResponse(), nil
		},
	}

	page, err := collectChildren(context.Background(), browser, "mill", browseQuery{nodeID: "i=85", maxNodes: 1})
	require.NoError(t, err)
	require.True(t, page.Truncated)
	require.Equal(t, base64.StdEncoding.EncodeToString([]byte("page-2")), page.ContinuationToken)
	require.Len(t, page.References, 1)
	require.Equal(t, "ns=2;s=EMS", page.References[0].NodeID)

	token, err := base64.StdEncoding.DecodeString(page.ContinuationToken)
	require.NoError(t, err)
	rest, err := collectChildren(context.Background(), browser, "mill", browseQuery{
		nodeID:       "i=85",
		maxNodes:     1,
		continuation: token,
	})
	require.NoError(t, err)
	require.False(t, rest.Truncated)
	require.Empty(t, rest.ContinuationToken)
	require.Equal(t, "ns=2;s=Server", rest.References[0].NodeID)
}

func TestCollectChildrenDropsHasProperty(t *testing.T) {
	variable := testReference(id.HasComponent, "ns=2;s=Temperature", 2, "Temperature", "Temperature", ua.NodeClassVariable, "i=63")
	property := testReference(id.HasProperty, "ns=2;s=EURange", 2, "EURange", "EURange", ua.NodeClassVariable, "i=68")
	browser := &scriptedBrowser{
		browseFn: func(*ua.BrowseRequest) (*ua.BrowseResponse, error) {
			return &ua.BrowseResponse{Results: []*ua.BrowseResult{{
				StatusCode: ua.StatusOK,
				References: []*ua.ReferenceDescription{variable, property},
			}}}, nil
		},
		readFn: func(req *ua.ReadRequest) (*ua.ReadResponse, error) {
			if len(req.NodesToRead) == 1 {
				return namespaceResponse(), nil
			}
			return attributeResponse(req, floatAttributes()), nil
		},
	}
	got, err := collectChildren(context.Background(), browser, "plc-1", browseQuery{nodeID: "i=85", maxNodes: 20})
	require.NoError(t, err)
	require.Len(t, got.References, 1)
	require.Equal(t, "ns=2;s=Temperature", got.References[0].NodeID)

	browser.browseFn = func(*ua.BrowseRequest) (*ua.BrowseResponse, error) {
		return &ua.BrowseResponse{Results: []*ua.BrowseResult{{
			StatusCode: ua.StatusOK,
			References: []*ua.ReferenceDescription{variable, property},
		}}}, nil
	}
	withProperties, err := collectChildren(context.Background(), browser, "plc-1", browseQuery{
		nodeID:            "i=85",
		maxNodes:          20,
		includeProperties: true,
	})
	require.NoError(t, err)
	require.Len(t, withProperties.References, 2)
	require.Equal(t, "HasProperty", withProperties.References[1].ReferenceType)
}

func TestCollectChildrenOmitsEdgeXTypeForArrayAndStructure(t *testing.T) {
	arrayNode := testReference(id.HasComponent, "ns=2;s=Samples", 2, "Samples", "Samples", ua.NodeClassVariable, "i=63")
	structNode := testReference(id.HasComponent, "ns=2;s=Status", 2, "Status", "Status", ua.NodeClassVariable, "i=63")
	calls := 0
	browser := &scriptedBrowser{
		browseFn: func(*ua.BrowseRequest) (*ua.BrowseResponse, error) {
			refs := []*ua.ReferenceDescription{arrayNode}
			if calls > 0 {
				refs = []*ua.ReferenceDescription{structNode}
			}
			return &ua.BrowseResponse{Results: []*ua.BrowseResult{{StatusCode: ua.StatusOK, References: refs}}}, nil
		},
		readFn: func(req *ua.ReadRequest) (*ua.ReadResponse, error) {
			if len(req.NodesToRead) == 1 {
				return namespaceResponse(), nil
			}
			if calls == 0 {
				return &ua.ReadResponse{Results: []*ua.DataValue{
					dataValue(ua.NewNumericNodeID(0, id.Float)),
					dataValue(int32(1)),
					dataValue(byte(accessCurrentRead)),
				}}, nil
			}
			return &ua.ReadResponse{Results: []*ua.DataValue{
				dataValue(ua.NewNumericNodeID(0, id.Structure)),
				dataValue(int32(-1)),
				dataValue(byte(accessCurrentRead | accessCurrentWrite)),
			}}, nil
		},
	}
	array, err := collectChildren(context.Background(), browser, "plc-1", browseQuery{nodeID: "i=85", maxNodes: 10})
	require.NoError(t, err)
	require.Empty(t, array.References[0].EdgeXValueType)
	require.Equal(t, int32(1), *array.References[0].ValueRank)

	calls = 1
	structure, err := collectChildren(context.Background(), browser, "plc-1", browseQuery{nodeID: "i=85", maxNodes: 10})
	require.NoError(t, err)
	require.Equal(t, "Structure", structure.References[0].DataType)
	require.Empty(t, structure.References[0].EdgeXValueType)
	require.Equal(t, "RW", structure.References[0].AccessLevel)
}

func TestCollectChildrenRejectsBadContinuation(t *testing.T) {
	browser := &scriptedBrowser{
		nextFn: func(*ua.BrowseNextRequest) (*ua.BrowseNextResponse, error) {
			return &ua.BrowseNextResponse{Results: []*ua.BrowseResult{{
				StatusCode: ua.StatusBadContinuationPointInvalid,
			}}}, nil
		},
	}
	_, err := collectChildren(context.Background(), browser, "plc-1", browseQuery{
		nodeID:       "i=85",
		maxNodes:     10,
		continuation: []byte("stale"),
	})
	var failure *browseFailure
	require.ErrorAs(t, err, &failure)
	require.Equal(t, http.StatusConflict, failure.status)
}

func TestCollectChildrenWrapsTransportError(t *testing.T) {
	browser := &scriptedBrowser{
		browseFn: func(*ua.BrowseRequest) (*ua.BrowseResponse, error) {
			return nil, fmt.Errorf("connection reset")
		},
	}
	_, err := collectChildren(context.Background(), browser, "plc-1", browseQuery{nodeID: "i=85", maxNodes: 10})
	require.Error(t, err)
	require.True(t, isTransportError(err))
}

func TestParseBrowseQuery(t *testing.T) {
	_, err := parseBrowseQuery(url.Values{"depth": {"2"}})
	require.EqualError(t, err, "depth is not accepted; one request returns the direct children of one node")

	_, err = parseBrowseQuery(url.Values{"maxNodes": {"1001"}})
	require.Error(t, err)
	_, err = parseBrowseQuery(url.Values{"maxNodes": {"0"}})
	require.Error(t, err)
	_, err = parseBrowseQuery(url.Values{"nodeId": {"ns=nope;s=EMS"}})
	require.Error(t, err)
	_, err = parseBrowseQuery(url.Values{"nodeClass": {"Foo"}})
	require.Error(t, err)
	_, err = parseBrowseQuery(url.Values{"includeProperties": {"maybe"}})
	require.Error(t, err)
	_, err = parseBrowseQuery(url.Values{"continuationToken": {"***"}})
	require.Error(t, err)

	query, err := parseBrowseQuery(url.Values{})
	require.NoError(t, err)
	require.Equal(t, objectsNodeID, query.nodeID)
	require.Equal(t, defaultMaxNodes, query.maxNodes)
	require.False(t, query.includeProperties)
	require.Nil(t, query.classes)

	query, err = parseBrowseQuery(url.Values{
		"nodeId":            {"ns=2;s=EMS"},
		"maxNodes":          {"10"},
		"nodeClass":         {"Object, Variable"},
		"includeProperties": {"true"},
	})
	require.NoError(t, err)
	require.Equal(t, "ns=2;s=EMS", query.nodeID)
	require.Equal(t, 10, query.maxNodes)
	require.True(t, query.includeProperties)
	require.Contains(t, query.classes, ua.NodeClassObject)
	require.Contains(t, query.classes, ua.NodeClassVariable)
}

func TestHandleBrowseRejectsDepth(t *testing.T) {
	d := testDriver()
	c, rec := newBrowseContext("/api/v3/opcua/device/mill/browse?depth=1", "mill")
	require.NoError(t, d.handleBrowse(c))
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "depth")
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, float64(http.StatusBadRequest), body["statusCode"])
	require.NotEmpty(t, body["apiVersion"])
}

func TestHandleBrowseUnknownDeviceAndMissingEndpoint(t *testing.T) {
	d := testDriver()
	sdk := sdkmocks.NewDeviceServiceSDK(t)
	sdk.On("GetDeviceByName", "missing").Return(models.Device{}, fmt.Errorf("not found")).Once()
	sdk.On("GetDeviceByName", "mill").Return(models.Device{Name: "mill"}, nil).Once()
	d.sdkService = sdk

	c, rec := newBrowseContext("/api/v3/opcua/device/missing/browse", "missing")
	require.NoError(t, d.handleBrowse(c))
	require.Equal(t, http.StatusNotFound, rec.Code)

	c, rec = newBrowseContext("/api/v3/opcua/device/mill/browse", "mill")
	require.NoError(t, d.handleBrowse(c))
	require.Equal(t, http.StatusNotFound, rec.Code)
}

func TestAcquireStatusRejectsNonStringProtocolProperty(t *testing.T) {
	d := testDriver()
	protocols := endpointProtocols("opc.tcp://127.0.0.1:4840")
	protocols[Protocol][Policy] = 1
	_, _, err := d.opcuaClient(context.Background(), "dev", protocols)
	require.Error(t, err)
	require.Equal(t, http.StatusBadRequest, acquireStatus(err))
	require.Contains(t, err.Error(), "cannot convert")
}

func TestOpcuaClientDropsSessionOnTransportError(t *testing.T) {
	d := testDriver()
	var dials atomic.Int32
	d.dial = func(context.Context, sessionInfo) (uaSession, error) {
		dials.Add(1)
		client, err := opcua.NewClient("opc.tcp://127.0.0.1:4840")
		require.NoError(t, err)
		return client, nil
	}
	protocols := endpointProtocols("opc.tcp://127.0.0.1:4840")
	_, release, err := d.opcuaClient(context.Background(), "dev", protocols)
	require.NoError(t, err)
	release(transportFailure(stderrors.New("connection reset")))

	_, release, err = d.opcuaClient(context.Background(), "dev", protocols)
	require.NoError(t, err)
	release(nil)
	require.Equal(t, int32(2), dials.Load())
}

func newBrowseContext(target, deviceName string) (echo.Context, *httptest.ResponseRecorder) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("deviceName")
	c.SetParamValues(deviceName)
	return c, rec
}
