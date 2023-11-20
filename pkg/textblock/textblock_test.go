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

package textblock

import (
	"reflect"
	"testing"
)

func IsErrorNil(err error) bool { return err == nil }

func Test_Read(t *testing.T) {
	tests := []struct {
		isValidErrFunc func(err error) bool
		name           string
		data           string
		header         string
		footer         string
		want           []string
	}{
		{
			name:           "no-blocks",
			data:           "something here\nsomething there",
			header:         "---- BEGIN ----",
			footer:         "---- END ----",
			want:           []string{},
			isValidErrFunc: IsErrorNil,
		},
		{
			name:           "single-block",
			data:           "something here\n---- BEGIN ----\nhello world\n---- END ----\nsomething there",
			header:         "---- BEGIN ----",
			footer:         "---- END ----",
			want:           []string{"hello world"},
			isValidErrFunc: IsErrorNil,
		},
		{
			name:           "multiple-blocks",
			data:           "---- BEGIN ----\nhello world\n---- END ----\nsomething in the middle\n---- BEGIN ----\nhow are you today?\n---- END ----",
			header:         "---- BEGIN ----",
			footer:         "---- END ----",
			want:           []string{"hello world", "how are you today?"},
			isValidErrFunc: IsErrorNil,
		},
		{
			name:           "multiline-single-block",
			data:           "---- BEGIN ----\nthis\n\tis\n\t\ta\n\t\t\ttest\n---- END ----",
			header:         "---- BEGIN ----",
			footer:         "---- END ----",
			want:           []string{"this\n\tis\n\t\ta\n\t\t\ttest"},
			isValidErrFunc: IsErrorNil,
		},
		{
			name:           "error-unexpected-start",
			data:           "---- BEGIN ----\nthis may\n---- BEGIN ----\nbe wrong\n---- END ----",
			header:         "---- BEGIN ----",
			footer:         "---- END ----",
			want:           []string{},
			isValidErrFunc: IsErrorUnexpectedStart,
		},
		{
			name:           "error-unexpected-end",
			data:           "---- END ----\n---- BEGIN ----\nthis may\nbe wrong\n---- END ----",
			header:         "---- BEGIN ----",
			footer:         "---- END ----",
			want:           []string{},
			isValidErrFunc: IsErrorUnexpectedEnd,
		},
		{
			name:           "error-missing-end",
			data:           "---- BEGIN ----\nthis may\nbe wrong",
			header:         "---- BEGIN ----",
			footer:         "---- END ----",
			want:           []string{},
			isValidErrFunc: IsErrorMissingEnd,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewTextBlock(tt.header, tt.footer)
			blocks, err := parser.FindBlocks(tt.data)
			if !tt.isValidErrFunc(err) {
				t.Errorf("error got: %v", err)
			}
			if !reflect.DeepEqual(blocks, tt.want) {
				t.Errorf("got: %v, expected: %v", blocks, tt.want)
			}
		})
	}
}

func Test_Write(t *testing.T) {
	tests := []struct {
		name   string
		data   string
		header string
		footer string
		want   string
	}{
		{
			name:   "empty-content",
			data:   "",
			header: "---- BEGIN ----",
			footer: "---- END ----",
			want:   "---- BEGIN ----\n---- END ----",
		},
		{
			name:   "multiline-content",
			data:   "hello\nworld",
			header: "---- BEGIN ----",
			footer: "---- END ----",
			want:   "---- BEGIN ----\nhello\nworld\n---- END ----",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewTextBlock(tt.header, tt.footer)
			output := parser.WriteBlock(tt.data)
			if !reflect.DeepEqual(output, tt.want) {
				t.Errorf("got: %v, expected: %v", output, tt.want)
			}
		})
	}
}
