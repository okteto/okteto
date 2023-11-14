// Copyright 2023 The Okteto Authors
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

package init

import (
	"reflect"
	"testing"

	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/model/forward"
)

func TestDiscardReverseIfCollision(t *testing.T) {
	tests := []struct {
		devInput  *model.Dev
		devOutput *model.Dev
		name      string
	}{
		{
			name: "no collision",
			devInput: &model.Dev{
				Forward: []forward.Forward{
					{
						Local:  8000,
						Remote: 8000,
					},
				},
				Reverse: []model.Reverse{
					{
						Local:  8001,
						Remote: 8001,
					},
				},
			},
			devOutput: &model.Dev{
				Forward: []forward.Forward{
					{
						Local:  8000,
						Remote: 8000,
					},
				},
				Reverse: []model.Reverse{
					{
						Local:  8001,
						Remote: 8001,
					},
				},
			},
		},
		{
			name: "collision",
			devInput: &model.Dev{
				Forward: []forward.Forward{
					{
						Local:  8000,
						Remote: 8000,
					},
				},
				Reverse: []model.Reverse{
					{
						Local:  8000,
						Remote: 8000,
					},
				},
			},
			devOutput: &model.Dev{
				Forward: []forward.Forward{
					{
						Local:  8000,
						Remote: 8000,
					},
				},
				Reverse: []model.Reverse{},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			discardReverseIfCollision(test.devInput)
			if !reflect.DeepEqual(test.devInput, test.devOutput) {
				t.Errorf("expected %v got %v", test.devOutput, test.devInput)
			}
		})
	}
}
