// Copyright 2019-present Open Networking Foundation.
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

package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/damianoneill/net/v2/netconf/ops"
	adapter "github.com/onosproject/gnmi-netconf-adapter/pkg/adapter"
	"github.com/onosproject/gnmi-netconf-adapter/pkg/adapter/modeldata"
	"github.com/onosproject/gnmi-netconf-adapter/pkg/adapter/modeldata/gostruct"
	"golang.org/x/crypto/ssh"

	pb "github.com/openconfig/gnmi/proto/gnmi"
)

var (
	model = adapter.NewModel(modeldata.ModelData, gostruct.SchemaTree["Device"])
)

// newGnmiServer creates a new gNMI server for a model and a device instance
func newGnmiServer(model *adapter.Model, ipAddress, username, password string) (pb.GNMIServer, error) {
	s, err := ncDeviceSessionForDemo(ipAddress, username, password)
	if err != nil {
		return nil, err
	}
	return adapter.NewAdapter(model, s)
}

func ncDeviceSessionForDemo(ipAddress, username, password string) (ops.OpSession, error) {
	sshConfig := &ssh.ClientConfig{
		User:            username,
		Auth:            []ssh.AuthMethod{ssh.Password(password)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}
	if !strings.Contains(ipAddress, ":") {
		ipAddress = fmt.Sprintf("%s:%d", ipAddress, 830)
	}
	return ops.NewSession(context.Background(), sshConfig, ipAddress)
}
