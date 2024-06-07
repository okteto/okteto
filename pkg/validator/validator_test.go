// Copyright 2024 The Okteto Authors
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

package validator

import (
	"testing"
)

func Test_isForbiddenVariableName(t *testing.T) {
	type args struct {
		name string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "OKTETO_NAMESPACE should not be allowed",
			args: args{
				name: "OKTETO_NAMESPACE",
			},
			want: true,
		},
		{
			name: "OKTETO_CONTEXT should not be allowed",
			args: args{
				name: "OKTETO_CONTEXT",
			},
			want: true,
		},
		{
			name: "OKTETO_URL should not be allowed",
			args: args{
				name: "OKTETO_URL",
			},
			want: true,
		},
		{
			name: "lowercase or uppercase should have the same output",
			args: args{
				name: "okteto_url",
			},
			want: true,
		},
		{
			name: "any value not listed should be allowed",
			args: args{
				name: "ANY",
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isForbiddenVariableName(tt.args.name); got != tt.want {
				t.Errorf("isForbiddenVariableName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCheckVariablesFlag(t *testing.T) {
	type args struct {
		variables []string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "malformed variable string should return nil",
			args: args{
				variables: []string{"not", "=value"},
			},
			wantErr: false,
		},
		{
			name: "all variables are forbidden",
			args: args{
				variables: []string{"OKTETO_CONTEXT=value", "OKTETO_NAMESPACE=value"},
			},
			wantErr: true,
		},
		{
			name: "some variables are forbidden",
			args: args{
				variables: []string{"OKTETO_CONTEXT=value", "VARIABLENAME=value"},
			},
			wantErr: true,
		},
		{
			name: "valid variables should not return error",
			args: args{
				variables: []string{"VALID1=value", "VALID2=value"},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := CheckVariablesFlag(tt.args.variables); (err != nil) != tt.wantErr {
				t.Errorf("CheckVariablesFlag() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
