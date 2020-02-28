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
	"github.com/damianoneill/net/v2/netconf/ops"

	gnmi "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ygot/ygot"
)

// ConfigCallback is the signature of the function to apply a validated config to the physical device.
type ConfigCallback func(ygot.ValidatedGoStruct) error

var (
	supportedEncodings = []gnmi.Encoding{gnmi.Encoding_JSON}
)

// Adapter struct implements the interface of gnmi server. It supports Capabilities, Get, and Set APIs.
// Typical usage:
//  netconfSession, err := ops.NewSession(ctx, sshConfig, "10.228.63.5:830")
//	g := grpc.NewServer()
//	s, err := adapter.NewAdapter(model, netconfSession)
//	gnmi.NewServer(g, s)
//	reflection.Register(g)
//	listen, err := net.Listen("tcp", ":8080")
//	g.Serve(listen)
//
type Adapter struct {
	model *Model
	ncs   ops.OpSession
}
