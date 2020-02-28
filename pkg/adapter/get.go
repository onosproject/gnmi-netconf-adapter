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
	"encoding/json"
	"encoding/xml"
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/openconfig/gnmi/value"

	"github.com/openconfig/goyang/pkg/yang"

	"github.com/damianoneill/net/v2/netconf/ops"

	"github.com/openconfig/gnmi/proto/gnmi"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	log "k8s.io/klog"
)

// Get implements the Get RPC in gNMI spec.
func (a *Adapter) Get(ctx context.Context, req *gnmi.GetRequest) (*gnmi.GetResponse, error) {

	if err := a.checkEncodingAndModel(req.GetEncoding(), req.GetUseModels()); err != nil {
		return nil, status.Error(codes.Unimplemented, err.Error())
	}

	// Create a default path to deliver the complete tree if there are no supplied paths.
	if len(req.Path) == 0 {
		req.Path = []*gnmi.Path{{}}
	}

	paths := req.GetPath()

	notifications := make([]*gnmi.Notification, len(paths))

	for i, path := range paths {
		n, err := a.processPath(ctx, req, path)
		if err != nil {
			return nil, err
		}
		notifications[i] = n
	}

	return &gnmi.GetResponse{Notification: notifications}, nil
}

// Exexcutes a gNMI Get for a single path
func (a *Adapter) processPath(ctx context.Context, req *gnmi.GetRequest, path *gnmi.Path) (*gnmi.Notification, error) {

	// Resolve the full path using the prefix if there is one.
	prefix := req.GetPrefix()
	fullPath := path
	if prefix != nil {
		fullPath = gnmiFullPath(prefix, path)
	}

	// Check that the requested path is defined in the schema
	entry := a.getSchemaEntryForPath(fullPath)
	if entry == nil {
		return nil, status.Errorf(codes.NotFound, "path %v not found (Test)", fullPath)
	}

	// Convert the request path to a netconf subtree filter and execute a get-config.
	netconfTree, err := a.executeGetConfig(pathToNetconfSubtree(fullPath), fullPath)
	if err != nil {
		return nil, err
	}

	// Convert the netconf response to a gNMI notification
	return a.netconfValueToGnmi(entry, netconfTree, fullPath, prefix)
}

// pathToNetconfSubtree converts a gNMI path to an XML string holding an equivalent netconf subtree filter.
// If there are no elements in the path, nil is returned (meaning the complete tree is returned).
func pathToNetconfSubtree(path *gnmi.Path) interface{} {

	if len(path.Elem) == 0 {
		return nil
	}
	var buf bytes.Buffer
	enc := xml.NewEncoder(&buf)
	for _, elem := range path.Elem {
		_ = enc.EncodeToken(xml.StartElement{Name: xml.Name{Local: elem.Name}})
		for k, v := range elem.Key {
			_ = enc.EncodeToken(xml.StartElement{Name: xml.Name{Local: k}})
			_ = enc.EncodeToken(xml.CharData(v))
			_ = enc.EncodeToken(xml.EndElement{Name: xml.Name{Local: k}})
		}
	}

	for i := len(path.Elem) - 1; i >= 0; i-- {
		_ = enc.EncodeToken(xml.EndElement{Name: xml.Name{Local: path.Elem[i].Name}})
	}

	_ = enc.Flush()
	return buf.String()
}

// executeGetConfig issues a netconfig get-config request using the specified subtree filter, returning the
// response as an XML string.
func (a *Adapter) executeGetConfig(filter interface{}, path *gnmi.Path) (string, error) {
	result := ""
	err := a.ncs.GetConfigSubtree(filter, ops.RunningCfg, &result)
	if err != nil {
		return "", status.Errorf(codes.Unknown, "failed to get config for %v %v", path, err)
	}
	return result, nil
}

// netconfValueToGnmi converts the netconf XML response to a gNMI notification.
func (a *Adapter) netconfValueToGnmi(entry *yang.Entry, result string, path *gnmi.Path, prefix *gnmi.Path) (*gnmi.Notification, error) {

	// The conversion is a 3-step process:
	// 1 - transform the netconf XML to a regular map, using the schema to create slices for lists and to convert
	//     leaf values correctly.
	// 2 - Extract the requested node from the map. Netconf will have returned the requested node with all its
	//     antecedent nodes; the antecedents are not included in the gnmi response.
	// 3 - Build the gnmi notification from the requested node.
	// Note that the first two steps could be merged into a single operation, so that the netconf to transformation only
	// took place for the requested node.

	netconfMap := a.netconfXMLtoMap(result)

	requestedValue, err := getRequestedNode(netconfMap, path)
	if err != nil {
		return nil, err
	}
	return a.buildGnmiNotification(entry, requestedValue, path, prefix)
}

// eldesc is used to track the state of XML element decoding.
// The in-scope elements are held in a stack, with each element pointing to its parent.
type eldesc struct {
	// schema entry corresponding to this element, (nil -> schema does not include it)
	schema *yang.Entry
	// map of child values for the container, keyed by child leaf/container name.
	// If the child is a simple container, the value will be another map[string] interface{}
	// If the child is a simple list, the value will be its scalar value.
	// If the child is a list, the value will be an array of map[string] interface{}
	children map[string]interface{}
	// reference to the descriptor for this element's parent
	parent *eldesc
}

// netconfXMLtoMap converts a well-formed netconf XML response to a map.
// map keys are element names, and values are either
// - scalars for leaf values
// - nested maps for container values
// - arrays of scalars/maps for leaf/container lists
// If netconf elements are not defined in the schema, they are not included in the map.
func (a *Adapter) netconfXMLtoMap(result string) map[string]interface{} {
	dec := xml.NewDecoder(strings.NewReader(result))

	top := make(map[string]interface{})
	cureld := &eldesc{schema: a.model.schemaTreeRoot, children: top}

	for {
		tk, _ := dec.Token()
		if tk == nil {
			return top
		}

		switch v := tk.(type) {

		case xml.StartElement:
			var nschema *yang.Entry
			if cureld.schema != nil {
				nschema = getChildSchema(v.Name.Local, cureld.schema)
			}
			cureld = &eldesc{schema: nschema, children: make(map[string]interface{}), parent: cureld}

			if nschema == nil {
				// Schema does not recognise this element name.
				break
			}

			linkNodeToParent(nschema, cureld)

		case xml.EndElement:
			// Pop the parent of the ending element from the stack
			cureld = cureld.parent

		case xml.CharData:
			// Only interested in the character data for an element that corresponds to a leaf/leaf-list.
			if cureld.schema != nil && (cureld.schema.IsLeaf() || cureld.schema.IsLeafList()) {
				addLeafValueToParent(v, cureld)
			}

		case xml.ProcInst:
		case xml.Comment:
		case xml.Directive:
			// None are expected but can be ignored if they are encountered.
		}
	}
}

// addLeafValueToParent adds a leaf value to the parent container's map.
func addLeafValueToParent(input xml.CharData, cureld *eldesc) {
	value := getLeafValue(input, cureld.schema)
	tag := cureld.schema.Name
	if cureld.schema.IsLeaf() {
		cureld.parent.children[tag] = value
	} else {
		cureld.parent.children[tag] = append(cureld.parent.children[tag].([]interface{}), value)
	}
}

// linkNodeToParent links a container/leaf to its parent node.
func linkNodeToParent(nschema *yang.Entry, cureld *eldesc) {
	tag := nschema.Name
	if nschema.IsList() || nschema.IsLeafList() {
		if _, ok := cureld.parent.children[tag]; !ok {
			// Create an array to hold the values for this list/leaf-list.
			cureld.parent.children[tag] = []interface{}{}
		}
		if nschema.IsList() {
			// Append this container's value map to the parent's list for this tag.
			cureld.parent.children[tag] = append(cureld.parent.children[tag].([]interface{}), cureld.children)
		}
	} else if nschema.IsContainer() {
		// Store this container's value map in the parent's map, keyed by this container's tag.
		cureld.parent.children[tag] = cureld.children
	}
}

// getRequestedNode delivers the node requested by the specified gNMI path from the
// input (a map delivered by the netconfXMLtoMap method)
func getRequestedNode(input interface{}, path *gnmi.Path) (interface{}, error) {

	for _, elem := range path.Elem {
		var nextmap map[string]interface{}
		switch v := input.(type) {
		case map[string]interface{}:
			nextmap = v
		case []interface{}:
			nextmap = v[0].(map[string]interface{})
		}
		input = nextmap[elem.Name]
		if input == nil {
			return nil, status.Errorf(codes.NotFound, "failed to find path: %v", path)
		}
	}
	if v, ok := input.([]interface{}); ok {
		input = v[0].(map[string]interface{})
	}
	return input, nil
}

// buildGnmiNotification maps the netconf returned value to a gNMI notification
func (a *Adapter) buildGnmiNotification(entry *yang.Entry, requestedValue interface{}, path *gnmi.Path, prefix *gnmi.Path) (*gnmi.Notification, error) {

	if entry.IsLeaf() {
		val, err := value.FromScalar(reflect.ValueOf(&requestedValue).Elem().Interface())
		if err != nil {
			return nil, status.Error(codes.Internal, fmt.Sprintf("leaf node %v does not contain a scalar type value: %v", path, err))
		}
		return notification(prefix, &gnmi.Update{Path: path, Val: val}), nil
	}
	if entry.IsDir() {
		jsonDump, err := json.Marshal(requestedValue)
		if err != nil {
			return nil, status.Error(codes.Internal, fmt.Sprintf("error in marshaling %s JSON tree to bytes: %v", "Internal", err))
		}
		return notification(prefix, &gnmi.Update{Path: path, Val: &gnmi.TypedValue{Value: &gnmi.TypedValue_JsonVal{JsonVal: jsonDump}}}), nil
	}
	panic(fmt.Sprintf("unexpected schema entry type %s", entry.Name))
}

// notification returns a new Notification with the specified prefix, update and the current time.
func notification(prefix *gnmi.Path, update *gnmi.Update) *gnmi.Notification {
	return &gnmi.Notification{
		Timestamp: time.Now().UnixNano(),
		Prefix:    prefix,
		Update:    []*gnmi.Update{update},
	}
}

// getSchemaEntryForPath delivers the schema entry associated with the last element of the supplied path,
// returning nil if the schema does not include the path.
func (a *Adapter) getSchemaEntryForPath(path *gnmi.Path) *yang.Entry {
	entry := a.model.schemaTreeRoot
	for _, elem := range path.Elem {
		entry = entry.Dir[elem.Name]
		if entry == nil {
			break
		}
	}
	return entry
}

func getChildSchema(name string, parent *yang.Entry) *yang.Entry {
	return parent.Dir[name]
}

// Delivers the value of leaf, using the type defined by the associated schema entry.
func getLeafValue(v xml.CharData, schema *yang.Entry) interface{} {

	switch schema.Type.Kind {
	case yang.Ystring:
		return strings.TrimSpace(string(v))
	case yang.Yunion:
		val, _ := getUnionValue(strings.TrimSpace(string(v)), schema.Type.Type)
		return val
	case yang.Yuint32:
		val, _ := strconv.ParseUint(strings.TrimSpace(string(v)), 10, 64)
		return val
	case yang.Yenum:
		return strings.TrimSpace(string(v))
	}
	// TODO Handle other kinds
	log.Errorf("Leaf kind %s still to be supported", schema.Type.Kind)
	return strings.TrimSpace(string(v))
}

func getUnionValue(v string, types []*yang.YangType) (interface{}, error) {
	for _, t := range types {
		switch t.Kind {
		case yang.Ystring:
			if isValidString(v, t) {
				return v, nil
			}
		case yang.Yint32:
			val := isValidInt(v, t)
			if val != nil {
				return val, nil
			}
		}
		// TODO Add other kinds.
	}
	return nil, status.Errorf(codes.NotFound, "failed to set union value: %s", v)
}

func isValidString(v string, t *yang.YangType) bool {
	return anyPatternMatches(v, t.Pattern)
	// TODO Range checks?
}

func isValidInt(v string, t *yang.YangType) interface{} {
	val, err := strconv.ParseInt(v, 10, 32)
	if err != nil {
		return nil
	}

	for _, r := range t.Range {
		if val >= int64(r.Min.Value) && val <= int64(r.Max.Value) {
			return val
		}
	}

	return nil
}

func anyPatternMatches(v string, patterns []string) bool {
	for _, p := range patterns {
		if !patternMatches(v, p) {
			return false
		}
	}
	return true
}

func patternMatches(v string, p string) bool {
	// fixYangRegexp adds ^(...)$ around the pattern - the result is
	// equivalent to a full match of whole string.
	r, err := regexp.Compile(fixYangRegexp(p))
	return err != nil && r.MatchString(v)
}

// Following function is lifted unchanged from https://github.com/openconfig/ygot/blob/master/ytypes/string_type.go

// fixYangRegexp takes a pattern regular expression from a YANG module and
// returns it into a format which can be used by the Go regular expression
// library. YANG uses a W3C standard that is defined to be implicitly anchored
// at the head or tail of the expression. See
// https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/#regexs for details.
func fixYangRegexp(pattern string) string {
	var buf bytes.Buffer
	var inEscape bool
	var prevChar rune
	addParens := false

	for i, ch := range pattern {
		if i == 0 && ch != '^' {
			buf.WriteRune('^')
			// Add parens around entire expression to prevent logical
			// subexpressions associating with leading/trailing ^ / $.
			buf.WriteRune('(')
			addParens = true
		}

		switch ch {
		case '$':
			// Dollar signs need to be escaped unless they are at
			// the end of the pattern, or are already escaped.
			if !inEscape && i != len(pattern)-1 {
				buf.WriteRune('\\')
			}
		case '^':
			// Carets need to be escaped unless they are already
			// escaped, indicating set negation ([^.*]) or at the
			// start of the string.
			if !inEscape && prevChar != '[' && i != 0 {
				buf.WriteRune('\\')
			}
		}

		// If the previous character was an escape character, then we
		// leave the escape, otherwise check whether this is an escape
		// char and if so, then enter escape.
		inEscape = !inEscape && ch == '\\'

		buf.WriteRune(ch)

		if i == len(pattern)-1 {
			if addParens {
				buf.WriteRune(')')
			}
			if ch != '$' {
				buf.WriteRune('$')
			}
		}

		prevChar = ch
	}

	return buf.String()
}
