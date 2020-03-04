// Copyright 2020 The Okteto Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ssh

import (
	"context"
	"testing"

	"github.com/okteto/okteto/pkg/model"
)

func TestReverseManager_Add(t *testing.T) {
	tests := []struct {
		name     string
		add      *model.Reverse
		reverses map[int]*reverse
		wantErr  bool
	}{
		{
			name:     "single",
			add:      &model.Reverse{Local: 8080, Remote: 8081},
			reverses: map[int]*reverse{},
			wantErr:  false,
		},
		{
			name:     "existing",
			add:      &model.Reverse{Local: 8080, Remote: 8081},
			reverses: map[int]*reverse{8080: &reverse{localPort: 8080, remotePort: 8081}},
			wantErr:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &ReverseManager{
				reverses: tt.reverses,
				ctx:      context.TODO(),
				sshHost:  "localhost",
				sshPort:  22,
			}

			if err := r.Add(tt.add); (err != nil) != tt.wantErr {
				t.Errorf("ReverseManager.Add() error = %v, wantErr %v", err, tt.wantErr)
			}

			f := r.reverses[8080]
			if f.localPort != 8080 {
				t.Errorf("local port is not 8080, it is: %d", f.localPort)
			}

			if f.remotePort != 8081 {
				t.Errorf("remote port is not 8081, it is: %d", f.remotePort)
			}
		})
	}
}
