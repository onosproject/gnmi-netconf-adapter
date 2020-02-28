// Copyright 2020-present Open Networking Foundation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package adapter

import (
	"context"
	"errors"
	"testing"

	assert "github.com/stretchr/testify/require"

	"github.com/damianoneill/net/v2/netconf/ops"
	"github.com/damianoneill/net/v2/netconf/ops/mocks"

	"github.com/golang/protobuf/proto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/openconfig/gnmi/proto/gnmi"

	"github.com/onosproject/gnmi-netconf-adapter/pkg/adapter/modeldata"
	"github.com/onosproject/gnmi-netconf-adapter/pkg/adapter/modeldata/gostruct"
)

var (
	// model is the model for test config server.
	model = NewModel(modeldata.ModelData, gostruct.SchemaTree["Device"])
)

type gnmiSetTestCase struct {
	desc        string                      // description of test case.
	op          gnmi.UpdateResult_Operation // operation type.
	textPrefix  string                      // Optional path prefix
	textPbPath  string                      // text format of gnmi Path proto.
	val         *gnmi.TypedValue            // value for UPDATE/REPLACE operations. always nil for DELETE.
	wantRetCode codes.Code                  // grpc return code.
	ncFilter    interface{}
	ncResponse  error
}

func TestUpdate(t *testing.T) {
	tests := []gnmiSetTestCase{{
		desc: "update leaf node",
		op:   gnmi.UpdateResult_UPDATE,
		textPbPath: `
			elem: <name: "configuration" >
			elem: <name: "system" >
			elem: <name: "services" >
			elem: <name: "ssh" >
			elem: <name: "max-sessions-per-connection" >
		`,
		val: &gnmi.TypedValue{
			Value: &gnmi.TypedValue_IntVal{IntVal: 64},
		},
		ncFilter:    `<configuration><system><services><ssh><max-sessions-per-connection operation="merge">64</max-sessions-per-connection></ssh></services></system></configuration>`,
		wantRetCode: codes.OK,
	}, {
		desc: "update subtree",
		op:   gnmi.UpdateResult_UPDATE,
		textPbPath: `
			elem: <name: "configuration" >
			elem: <name: "system" >
			elem: <name: "services" >
			elem: <name: "ssh" >
		`,
		val: &gnmi.TypedValue{
			Value: &gnmi.TypedValue_JsonVal{
				JsonVal: []byte(`{"max-sessions-per-connection": 16}`),
			},
		},
		ncFilter:    `<configuration><system><services><ssh operation="merge"><max-sessions-per-connection>16</max-sessions-per-connection></ssh></services></system></configuration>`,
		wantRetCode: codes.OK,
	}, {
		desc: "device fails to update",
		op:   gnmi.UpdateResult_UPDATE,
		textPbPath: `
			elem: <name: "configuration" >
		`,
		val: &gnmi.TypedValue{
			Value: &gnmi.TypedValue_JsonVal{
				JsonVal: []byte(`{"version": "XVZ"}`),
			},
		},
		ncFilter:    `<configuration operation="merge"><version>XVZ</version></configuration>`,
		ncResponse:  errors.New("netconf failure"),
		wantRetCode: codes.Unknown,
	}, {
		desc: "update with path prefix",
		op:   gnmi.UpdateResult_UPDATE,
		textPrefix: `
			elem: <name: "configuration" >
		`,
		textPbPath: `
			elem: <name: "version" >
		`,
		val: &gnmi.TypedValue{
			Value: &gnmi.TypedValue_StringVal{StringVal: "ABC"},
		},
		ncFilter:    `<configuration><version operation="merge">ABC</version></configuration>`,
		wantRetCode: codes.OK,
	}}

	for i := range tests {
		tc := tests[i]
		t.Run(tc.desc, func(t *testing.T) {
			runTestSet(t, model, tc)
		})
	}
}

func TestDelete(t *testing.T) {
	tests := []gnmiSetTestCase{{
		desc: "delete leaf node",
		op:   gnmi.UpdateResult_DELETE,
		textPbPath: `
			elem: <name: "configuration" >
			elem: <name: "system" >
			elem: <name: "services" >
			elem: <name: "ssh" >
			elem: <name: "max-sessions-per-connection" >
		`,
		ncFilter:    `<configuration><system><services><ssh><max-sessions-per-connection operation="delete"></max-sessions-per-connection></ssh></services></system></configuration>`,
		wantRetCode: codes.OK,
	}, {
		desc: "delete sub-tree",
		op:   gnmi.UpdateResult_DELETE,
		textPbPath: `
			elem: <name: "configuration" >
			elem: <name: "system" >
			elem: <name: "services" >
			elem: <name: "ssh" >
		`,
		ncFilter:    `<configuration><system><services><ssh operation="delete"></ssh></services></system></configuration>`,
		wantRetCode: codes.OK,
	}, {
		desc: "delete leaf node with attribute in its precedent path",
		op:   gnmi.UpdateResult_DELETE,
		textPbPath: `
					elem: <name: "configuration" >
					elem: <name: "interfaces" >
					elem: <
						name: "interface" 
						key: <key: "name" value: "0/3/0" >
					>
					elem: <name: "otn-options" >
					elem: <name: "rate" >
					`,
		ncFilter:    `<configuration><interfaces><interface><name>0/3/0</name><otn-options><rate operation="delete"></rate></otn-options></interface></interfaces></configuration>`,
		wantRetCode: codes.OK,
	}, {
		desc: "delete sub-tree with attribute in its precedent path",
		op:   gnmi.UpdateResult_DELETE,
		textPbPath: `
					elem: <name: "configuration" >
					elem: <name: "interfaces" >
					elem: <
						name: "interface" 
						key: <key: "name" value: "0/3/0" >
					>
					elem: <name: "otn-options" >
					`,
		ncFilter:    `<configuration><interfaces><interface><name>0/3/0</name><otn-options operation="delete"></otn-options></interface></interfaces></configuration>`,
		wantRetCode: codes.OK,
	}, {
		desc: "device fails to delete",
		op:   gnmi.UpdateResult_DELETE,
		textPbPath: `
			elem: <name: "configuration" >
			elem: <name: "version" >
		`,
		ncFilter:    `<configuration><version operation="delete"></version></configuration>`,
		ncResponse:  errors.New("netconf failure"),
		wantRetCode: codes.Unknown,
	}, {
		desc: "delete with path prefix",
		op:   gnmi.UpdateResult_DELETE,
		textPrefix: `
			elem: <name: "configuration" >
		`,
		textPbPath: `
			elem: <name: "version" >
		`,
		ncFilter:    `<configuration><version operation="delete"></version></configuration>`,
		wantRetCode: codes.OK,
	}}

	for i := range tests {
		tc := tests[i]
		t.Run(tc.desc, func(t *testing.T) {
			runTestSet(t, model, tc)
		})
	}
}

func TestReplace(t *testing.T) {

	tests := []gnmiSetTestCase{{
		desc: "replace a subtree",
		op:   gnmi.UpdateResult_REPLACE,
		textPbPath: `
			elem: <name: "configuration" >
		`,
		val: &gnmi.TypedValue{
			Value: &gnmi.TypedValue_JsonVal{
				JsonVal: []byte(`{"version": "XVZ"}`),
			},
		},
		ncFilter:    `<configuration operation="replace"><version>XVZ</version></configuration>`,
		wantRetCode: codes.OK,
	}, {
		desc: "replace a keyed list subtree",
		op:   gnmi.UpdateResult_REPLACE,
		textPbPath: `
			elem: <name: "configuration" >
			elem: <name: "system" >
			elem: <name: "services" >
		`,
		val: &gnmi.TypedValue{
			Value: &gnmi.TypedValue_JsonVal{
				JsonVal: []byte(`{"ssh": {"max-sessions-per-connection": 8}}`),
			},
		},
		ncFilter:    `<configuration><system><services operation="replace"><ssh><max-sessions-per-connection>8</max-sessions-per-connection></ssh></services></system></configuration>`,
		wantRetCode: codes.OK,
	}, {
		desc: "replace node with attribute in its precedent path",
		op:   gnmi.UpdateResult_REPLACE,
		textPbPath: `
					elem: <name: "configuration" >
					elem: <name: "interfaces" >
					elem: <
						name: "interface" 
						key: <key: "name" value: "0/3/0" >
					>
					elem: <name: "otn-options" >
					`,
		val: &gnmi.TypedValue{
			Value: &gnmi.TypedValue_JsonVal{
				JsonVal: []byte(`{"rate": "otu4", "laser-enable": ""}`),
			},
		},
		ncFilter:    `<configuration><interfaces><interface><name>0/3/0</name><otn-options operation="replace"><laser-enable></laser-enable><rate>otu4</rate></otn-options></interface></interfaces></configuration>`,
		wantRetCode: codes.OK,
	}, {
		desc: "replace a leaf node of int type",
		op:   gnmi.UpdateResult_REPLACE,
		textPbPath: `
			elem: <name: "configuration" >
			elem: <name: "system" >
			elem: <name: "services" >
			elem: <name: "ssh" >
			elem: <name: "max-sessions-per-connection" >
		`,
		val: &gnmi.TypedValue{
			Value: &gnmi.TypedValue_IntVal{IntVal: 64},
		},
		ncFilter:    `<configuration><system><services><ssh><max-sessions-per-connection operation="replace">64</max-sessions-per-connection></ssh></services></system></configuration>`,
		wantRetCode: codes.OK,
	}, {
		desc: "replace a leaf node of string type",
		op:   gnmi.UpdateResult_REPLACE,
		textPbPath: `
			elem: <name: "configuration" >
			elem: <name: "version" >
		`,
		val: &gnmi.TypedValue{
			Value: &gnmi.TypedValue_StringVal{StringVal: "ABC"},
		},
		ncFilter:    `<configuration><version operation="replace">ABC</version></configuration>`,
		wantRetCode: codes.OK,
	}, {
		desc: "replace an non-existing leaf node",
		op:   gnmi.UpdateResult_REPLACE,
		textPbPath: `
			elem: <name: "system" >
			elem: <name: "openflow" >
			elem: <name: "agent" >
			elem: <name: "config" >
			elem: <name: "foo-bar" >
		`,
		val: &gnmi.TypedValue{
			Value: &gnmi.TypedValue_StringVal{StringVal: "SECURE"},
		},
		wantRetCode: codes.NotFound,
	}, {
		desc: "device fails to replace",
		op:   gnmi.UpdateResult_REPLACE,
		textPbPath: `
			elem: <name: "configuration" >
		`,
		val: &gnmi.TypedValue{
			Value: &gnmi.TypedValue_JsonVal{
				JsonVal: []byte(`{"version": "XVZ"}`),
			},
		},
		ncFilter:    `<configuration operation="replace"><version>XVZ</version></configuration>`,
		ncResponse:  errors.New("netconf failure"),
		wantRetCode: codes.Unknown,
	}, {
		desc: "replace with path prefix",
		op:   gnmi.UpdateResult_REPLACE,
		textPrefix: `
			elem: <name: "configuration" >
		`,
		textPbPath: `
			elem: <name: "version" >
		`,
		val: &gnmi.TypedValue{
			Value: &gnmi.TypedValue_StringVal{StringVal: "ABC"},
		},
		ncFilter:    `<configuration><version operation="replace">ABC</version></configuration>`,
		wantRetCode: codes.OK,
	}}

	for i := range tests {
		tc := tests[i]
		t.Run(tc.desc, func(t *testing.T) {
			runTestSet(t, model, tc)
		})
	}
}

func runTestSet(t *testing.T, m *Model, tc gnmiSetTestCase) {

	mockNc := &mocks.OpSession{}
	mockNc.On("EditConfigCfg", ops.RunningCfg, tc.ncFilter).Return(tc.ncResponse)

	s, err := NewAdapter(model, mockNc)
	assert.NoError(t, err, "error in creating server: %v", err)

	// Send request
	var pbPath gnmi.Path
	if err := proto.UnmarshalText(tc.textPbPath, &pbPath); err != nil {
		t.Fatalf("error in unmarshaling path: %v", err)
	}

	var req *gnmi.SetRequest
	switch tc.op {
	case gnmi.UpdateResult_DELETE:
		req = &gnmi.SetRequest{Delete: []*gnmi.Path{&pbPath}, Prefix: getPathPrefix(tc.textPrefix)}
	case gnmi.UpdateResult_REPLACE:
		req = &gnmi.SetRequest{Replace: []*gnmi.Update{{Path: &pbPath, Val: tc.val}}, Prefix: getPathPrefix(tc.textPrefix)}
	case gnmi.UpdateResult_UPDATE:
		req = &gnmi.SetRequest{Update: []*gnmi.Update{{Path: &pbPath, Val: tc.val}}, Prefix: getPathPrefix(tc.textPrefix)}
	default:
		t.Fatalf("invalid op type: %v", tc.op)
	}
	_, err = s.Set(context.TODO(), req)

	// Check return code
	gotRetStatus, ok := status.FromError(err)
	if !ok {
		t.Fatal("got a non-grpc error from grpc call")
	}
	if gotRetStatus.Code() != tc.wantRetCode {
		t.Fatalf("got return code %v, want %v\nerror message: %v", gotRetStatus.Code(), tc.wantRetCode, err)
	}
}

func getPathPrefix(prefix string) *gnmi.Path {

	var prefPath *gnmi.Path
	if prefix != "" {
		var pfx gnmi.Path
		_ = proto.UnmarshalText(prefix, &pfx)
		prefPath = &pfx
	}
	return prefPath
}
