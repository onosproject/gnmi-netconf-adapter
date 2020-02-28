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

// Package modeldata contains the following model data in gnmi proto struct:
//	openconfig-interfaces 2.0.0,
//	openconfig-openflow 0.1.0,
//	openconfig-platform 0.5.0,
//	openconfig-system 0.2.0.
package modeldata

import (
	"github.com/openconfig/gnmi/proto/gnmi"
)

// This file is based on a modelplugin/Junos-19.3.1.8/modelmain.go file generated locally for JUNOS 19.3R1.8
// JUNOS models and plugins will be incorporated into µONOS at a future date.

// ModelData represents a subset of Junos-19.3R1.8
var ModelData = []*gnmi.ModelData{
	{Name: "junos-conf-interfaces", Organization: "Juniper", Version: "2019-01-01"},
	{Name: "junos-conf-system", Organization: "Juniper", Version: "2019-01-01"},
}
