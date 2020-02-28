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
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	assert "github.com/stretchr/testify/require"

	"github.com/damianoneill/net/v2/netconf/ops/mocks"
	"github.com/stretchr/testify/mock"

	"github.com/damianoneill/net/v2/netconf/ops"

	"github.com/golang/protobuf/proto"
	"github.com/openconfig/gnmi/value"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/openconfig/gnmi/proto/gnmi"

	"github.com/onosproject/gnmi-netconf-adapter/pkg/adapter/modeldata"
	"github.com/onosproject/gnmi-netconf-adapter/pkg/adapter/modeldata/gostruct"
)

type getTestCase struct {
	nilPath     bool
	desc        string
	textPrefix  string
	textPbPath  string
	modelData   []*gnmi.ModelData
	wantRetCode codes.Code
	wantRespVal interface{}
	ncFilter    interface{}
	ncResponse  error
	ncResult    string
}

func TestGet(t *testing.T) {

	tests := []*getTestCase{
		{
			desc: "get valid but non-existing node",
			textPbPath: `
			elem: <name: "configuration" >
			elem: <name: "system" >
			elem: <name: "services" >
		`,
			modelData:   modeldata.ModelData,
			ncFilter:    `<configuration><system><services></services></system></configuration>`,
			wantRetCode: codes.NotFound,
		}, {
			desc:        "nil Request",
			nilPath:     true,
			ncResult:    `<configuration><system><services><ssh><max-sessions-per-connection>32</max-sessions-per-connection></ssh></services></system></configuration>`,
			wantRetCode: codes.OK,
			wantRespVal: `{
						"configuration": {
							"system": {
								"services": {
									"ssh": {
										"max-sessions-per-connection": 32
									}
								}
							}
						}
					}`,
		}, {
			desc:        "root node",
			ncResult:    `<configuration><system><services><ssh><max-sessions-per-connection>32</max-sessions-per-connection></ssh></services></system></configuration>`,
			wantRetCode: codes.OK,
			wantRespVal: `{
						"configuration": {
							"system": {
								"services": {
									"ssh": {
										"max-sessions-per-connection": 32
									}
								}
							}
						}
					}`,
		}, {
			desc: "get non-enum type",
			textPbPath: `
					elem: <name: "configuration" >
					elem: <name: "system" >
					elem: <name: "services" >
					elem: <name: "ssh" >
					elem: <name: "max-sessions-per-connection" >
				`,
			ncFilter:    `<configuration><system><services><ssh><max-sessions-per-connection></max-sessions-per-connection></ssh></services></system></configuration>`,
			ncResult:    `<configuration><system><services><ssh><max-sessions-per-connection>32</max-sessions-per-connection></ssh></services></system></configuration>`,
			wantRetCode: codes.OK,
			wantRespVal: int64(32),
		}, {
			desc: "get enum type",
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
			ncFilter:    `<configuration><interfaces><interface><name>0/3/0</name><otn-options><rate></rate></otn-options></interface></interfaces></configuration>`,
			ncResult:    `<configuration><interfaces><interface><name>0/3/0</name><otn-options><rate>otu4</rate></otn-options></interface></interfaces></configuration>`,
			wantRetCode: codes.OK,
			wantRespVal: "otu4",
		}, {
			desc:        "root child node",
			textPbPath:  `elem: <name: "configuration" >`,
			ncFilter:    `<configuration></configuration>`,
			ncResult:    `<configuration><system><services><ssh><max-sessions-per-connection>32</max-sessions-per-connection></ssh></services></system></configuration>`,
			wantRetCode: codes.OK,
			wantRespVal: `{
						"system": {
							"services": {
								"ssh": {
									"max-sessions-per-connection": 32
								}
							}
						}
					}`,
		}, {
			desc: "node with attribute",
			textPbPath: `
					elem: <name: "configuration" >
					elem: <name: "interfaces" >
					elem: <
						name: "interface" 
						key: <key: "name" value: "0/3/0" >
					>`,
			ncFilter:    `<configuration><interfaces><interface><name>0/3/0</name></interface></interfaces></configuration>`,
			ncResult:    `<configuration><interfaces><interface><name>0/3/0</name><otn-options><rate>otu4</rate></otn-options></interface></interfaces></configuration>`,
			wantRetCode: codes.OK,
			wantRespVal: `{
						"name": "0/3/0",
						"otn-options": { "rate": "otu4" }
						}`,
		}, {
			desc: "node with attribute in its parent",
			textPbPath: `
					elem: <name: "configuration" >
					elem: <name: "interfaces" >
					elem: <
						name: "interface" 
						key: <key: "name" value: "0/3/0" >
					>
					elem: <name: "otn-options" >
					`,
			ncFilter:    `<configuration><interfaces><interface><name>0/3/0</name><otn-options></otn-options></interface></interfaces></configuration>`,
			ncResult:    `<configuration><interfaces><interface><name>0/3/0</name><otn-options><rate>otu4</rate></otn-options></interface></interfaces></configuration>`,
			wantRetCode: codes.OK,
			wantRespVal: `{"rate": "otu4" }`,
		}, {
			desc: "non-existing node: wrong path name",
			textPbPath: `
								elem: <name: "components" >
								elem: <
									name: "component"
									key: <key: "foo" value: "swpri1-1-1" >
								>
								elem: <name: "bar" >`,
			wantRetCode: codes.NotFound,
		}, {
			desc:        "use of model data not supported",
			modelData:   []*gnmi.ModelData{{}},
			wantRetCode: codes.Unimplemented,
		}, {
			desc: "device fails to get",
			textPbPath: `
			elem: <name: "configuration" >
		`,
			ncFilter:    `<configuration></configuration>`,
			ncResponse:  errors.New("netconf failure"),
			wantRetCode: codes.Unknown,
		}, {
			desc: "prefxed path",
			textPrefix: `
			elem: <name: "configuration" >
		`,
			textPbPath: `
			elem: <name: "version" >
		`,
			ncFilter:    `<configuration><version></version></configuration>`,
			ncResult:    `<configuration><version>ABC</version></configuration>`,
			wantRetCode: codes.OK,
			wantRespVal: `ABC`,
		}, {
			desc: "ignore nodes not in the schema",
			textPbPath: `
			elem: <name: "configuration" >
		`,
			ncFilter:    `<configuration></configuration>`,
			ncResult:    `<configuration><version>ABC</version><notintheschema>XYZ</notintheschema></configuration>`,
			wantRetCode: codes.OK,
			wantRespVal: `{

								"version": "ABC"

					}`,
		}, {
			desc: "ignore unexpected xml elements in netconf",
			textPbPath: `
			elem: <name: "configuration" >
		`,
			ncFilter:    `<configuration></configuration>`,
			ncResult:    `<configuration><version>ABC</version><!-- comment -->></configuration>`,
			wantRetCode: codes.OK,
			wantRespVal: `{

								"version": "ABC"

					}`,
		}}
	for i := range tests {
		td := tests[i]
		t.Run(td.desc, func(t *testing.T) {
			runTestGet(t, td)
		})
	}
}

// runTestGet requests a path from the server by Get grpc call, and compares if
// the return code and response value are expected.
func runTestGet(t *testing.T, tc *getTestCase) {

	mockNc := &mocks.OpSession{}
	mockNc.On("GetConfigSubtree", tc.ncFilter, ops.RunningCfg, mock.Anything).Return(
		func(filter interface{}, source string, result interface{}) error {
			*result.(*string) = tc.ncResult
			return tc.ncResponse
		})

	model = NewModel(modeldata.ModelData, gostruct.SchemaTree["Device"])
	s, err := NewAdapter(model, mockNc)
	assert.NoError(t, err, "error in creating server: %v", err)

	pbPaths := []*gnmi.Path{}
	if !tc.nilPath {
		pbPath := &gnmi.Path{}
		if err := proto.UnmarshalText(tc.textPbPath, pbPath); err != nil {
			t.Fatalf("error in unmarshaling path: %v", err)
		}
		pbPaths = append(pbPaths, pbPath)
	}

	req := &gnmi.GetRequest{
		Path:      pbPaths,
		Encoding:  gnmi.Encoding_JSON,
		UseModels: tc.modelData,
		Prefix:    getPathPrefix(tc.textPrefix),
	}

	// Send request
	resp, err := s.Get(context.TODO(), req)

	// Check return code
	gotRetStatus, ok := status.FromError(err)
	assert.True(t, ok, "got a non-grpc error from grpc call")
	assert.Equal(t, tc.wantRetCode, gotRetStatus.Code(), "Unexpected return code")

	// Check response value
	var gotVal interface{}
	if resp != nil {
		notifs := resp.GetNotification()
		if len(notifs) != 1 {
			t.Fatalf("got %d notifications, want 1", len(notifs))
		}
		updates := notifs[0].GetUpdate()
		if len(updates) != 1 {
			t.Fatalf("got %d updates in the notification, want 1", len(updates))
		}
		val := updates[0].GetVal()
		if val.GetJsonVal() == nil {
			gotVal, err = value.ToScalar(val)
			if err != nil {
				t.Errorf("got: %v, want a scalar value", gotVal)
			}
		} else {
			// Unmarshal json data to gotVal container for comparison
			if err := json.Unmarshal(val.GetJsonVal(), &gotVal); err != nil {
				t.Fatalf("error in unmarshaling JSON data to json container: %v", err)
			}
			var wantJSONStruct interface{}
			if err := json.Unmarshal([]byte(tc.wantRespVal.(string)), &wantJSONStruct); err != nil {
				t.Fatalf("error in unmarshaling IETF JSON data to json container: %v", err)
			}
			tc.wantRespVal = wantJSONStruct
		}
	}

	if !reflect.DeepEqual(gotVal, tc.wantRespVal) {
		t.Errorf("got: %v (%T),\nwant %v (%T)", gotVal, gotVal, tc.wantRespVal, tc.wantRespVal)
	}
}
