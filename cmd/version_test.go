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

package cmd

import (
	"bytes"
	"testing"
)

func Test_printVersion(t *testing.T) {
	type args struct {
		version string
		commit  string
		date    string
	}
	tests := []struct {
		name  string
		args  args
		wantW string
	}{
		{
			"valid version output",
			args{"0.3.0", "c26cfaca0e38465935c48b13ae99d12fbf5d7cb1", "2019-11-05T21:16:06Z"},
			"0.3.0, commit c26cfaca0e38465935c48b13ae99d12fbf5d7cb1, built at 2019-11-05T21:16:06Z \n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := &bytes.Buffer{}
			printVersion(w, tt.args.version, tt.args.commit, tt.args.date)
			if gotW := w.String(); gotW != tt.wantW {
				t.Errorf("printVersion() = %v, want %v", gotW, tt.wantW)
			}
		})
	}
}
