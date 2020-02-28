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

// Package adapter implements a gnmi server that adapts to a netconf device.
package adapter

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"sort"

	"github.com/openconfig/goyang/pkg/yang"

	"github.com/damianoneill/net/v2/netconf/ops"
	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/gnmi/value"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Set implements the Set RPC in gNMI spec.
func (a *Adapter) Set(ctx context.Context, req *gnmi.SetRequest) (*gnmi.SetResponse, error) {

	prefix := req.GetPrefix()
	var results []*gnmi.UpdateResult

	// Execute operations in order.
	// Reference: https://github.com/openconfig/reference/blob/master/rpc/gnmi/gnmi-specification.md#34-modifying-state

	// Execute Deletes
	for _, path := range req.GetDelete() {
		res, grpcStatusError := a.executeOperation(gnmi.UpdateResult_DELETE, prefix, path, nil)
		if grpcStatusError != nil {
			return nil, grpcStatusError
		}
		results = append(results, res)
	}

	// Execute Replaces
	for _, upd := range req.GetReplace() {
		res, grpcStatusError := a.executeOperation(gnmi.UpdateResult_REPLACE, prefix, upd.GetPath(), upd.GetVal())
		if grpcStatusError != nil {
			return nil, grpcStatusError
		}
		results = append(results, res)
	}

	// Execute Updates
	for _, upd := range req.GetUpdate() {
		res, grpcStatusError := a.executeOperation(gnmi.UpdateResult_UPDATE, prefix, upd.GetPath(), upd.GetVal())
		if grpcStatusError != nil {
			return nil, grpcStatusError
		}
		results = append(results, res)
	}

	return &gnmi.SetResponse{
		Prefix:   prefix,
		Response: results,
	}, nil
}

// executeOperation executes a gNMI Set operation mapping it to a netconf edit-config operation.
func (a *Adapter) executeOperation(op gnmi.UpdateResult_Operation, prefix, path *gnmi.Path, val *gnmi.TypedValue) (*gnmi.UpdateResult, error) {

	request, err := a.gnmiToNetconfOperation(op, prefix, path, val)
	if err != nil {
		return nil, err
	}

	err = a.ncs.EditConfigCfg(ops.RunningCfg, request)
	if err != nil {
		return nil, status.Errorf(codes.Unknown, "edit failed %s", err)
	}

	return &gnmi.UpdateResult{
		Path: path,
		Op:   op,
	}, nil
}

// gnmiToNetconfOperation maps a gNMI set operation to a netconfig edit-config operation.
func (a *Adapter) gnmiToNetconfOperation(op gnmi.UpdateResult_Operation, prefix, path *gnmi.Path, inval *gnmi.TypedValue) (interface{}, error) {

	fullPath := path
	if prefix != nil {
		fullPath = gnmiFullPath(prefix, path)
	}

	entry := a.getSchemaEntryForPath(fullPath)
	if entry == nil {
		return nil, status.Errorf(codes.NotFound, "path %v not found (Test)", fullPath)
	}

	var buf bytes.Buffer
	enc := xml.NewEncoder(&buf)

	mapPathToNetconf(fullPath, op, enc)

	if op != gnmi.UpdateResult_DELETE {
		err := a.mapSetValueToNetconf(enc, entry, inval)
		if err != nil {
			return nil, err
		}
	}

	// Close off the XML elements defined by the path.
	for i := len(fullPath.Elem) - 1; i >= 0; i-- {
		_ = enc.EncodeToken(xml.EndElement{Name: xml.Name{Local: fullPath.Elem[i].Name}})
	}

	// Return the XML document.
	_ = enc.Flush()
	filter := buf.String()
	if len(filter) == 0 {
		return nil, nil
	}
	return filter, nil
}

// mapPathToNetconf converts a path for a gNMI Set operation to a netconf XML document using the supplied encoder.
func mapPathToNetconf(fullPath *gnmi.Path, op gnmi.UpdateResult_Operation, enc *xml.Encoder) {
	for i, elem := range fullPath.Elem {
		startEl := xml.StartElement{Name: xml.Name{Local: elem.Name}}

		// Add the relevant operation attribute in the last element of the path.
		if i == len(fullPath.Elem)-1 {
			startEl.Attr = []xml.Attr{{Name: xml.Name{Local: "operation"}, Value: mapOperation(op)}}
		}
		_ = enc.EncodeToken(startEl)

		// Add in any key values.
		for k, v := range elem.Key {
			_ = enc.EncodeToken(xml.StartElement{Name: xml.Name{Local: k}})
			_ = enc.EncodeToken(xml.CharData(v))
			_ = enc.EncodeToken(xml.EndElement{Name: xml.Name{Local: k}})
		}
	}
}

// mapSetValueToNetconf converts a value defined by a Set update/replace operation to the corresponding
// netconf XML elements, using the specified encoder.
func (a *Adapter) mapSetValueToNetconf(enc *xml.Encoder, schema *yang.Entry, inval *gnmi.TypedValue) error {

	editValue, err := mapValue(schema, inval)
	if err != nil {
		return err
	}
	if schema.IsDir() {
		_ = jsonMapToXML(editValue.(map[string]interface{}), enc)
	} else {
		_ = enc.EncodeToken(xml.CharData(fmt.Sprintf("%v", editValue)))
	}
	return nil
}

// Convert the value supplied on a Set operation, delivering:
// - a map for a container
// - scalar value for a leaf.
func mapValue(entry *yang.Entry, inval *gnmi.TypedValue) (interface{}, error) {
	var editValue interface{}
	if entry.IsDir() {
		editValue = make(map[string]interface{})
		err := json.Unmarshal(inval.GetJsonVal(), &editValue)
		if err != nil {
			return nil, status.Errorf(codes.Unknown, "invalid value %s", err)
		}
	} else {
		var err error
		editValue, err = value.ToScalar(inval)
		if err != nil {
			return nil, status.Errorf(codes.Unknown, "invalid value %s", err)
		}
	}
	return editValue, nil
}

func mapOperation(op gnmi.UpdateResult_Operation) string {
	opdesc := ""
	switch op {
	case gnmi.UpdateResult_DELETE:
		opdesc = "delete"
	case gnmi.UpdateResult_REPLACE:
		opdesc = "replace"
	case gnmi.UpdateResult_UPDATE:
		opdesc = "merge"
	default:
		panic(fmt.Sprintf("unexpected operation %s", op))
	}
	return opdesc
}

// Converts a map generated from a JSON value supplied on a Set operation to XML, using the supplied encoder.
func jsonMapToXML(input map[string]interface{}, enc *xml.Encoder) error {

	// Sort the map keys (makes for deterministic test behaviour).
	keys := sortMapKeys(input)

	for _, k := range keys {
		v := input[k]
		err := enc.EncodeToken(xml.StartElement{Name: xml.Name{Local: k}})
		if err != nil {
			return err
		}
		switch val := v.(type) {
		case map[string]interface{}:
			err = jsonMapToXML(val, enc)
			if err != nil {
				return err
			}
		default:
			_ = enc.EncodeToken(xml.CharData(fmt.Sprintf("%v", val)))
		}
		err = enc.EncodeToken(xml.EndElement{Name: xml.Name{Local: k}})
		if err != nil {
			return err
		}
	}
	return nil
}

func sortMapKeys(input map[string]interface{}) []string {
	keys := []string{}
	for k := range input {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		return keys[i] < keys[j]
	})
	return keys
}
