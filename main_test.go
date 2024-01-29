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

package main

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

func TestGetCurrentCmdWithUsedFlags(t *testing.T) {
	type expected struct {
		cmdName string
		flags   string
	}
	tests := []struct {
		name     string
		setupCmd func() *cobra.Command
		expected expected
	}{
		{
			name: "no flags set",
			setupCmd: func() *cobra.Command {
				cmd := &cobra.Command{Use: "testcmd"}
				return cmd
			},
			expected: expected{
				cmdName: "testcmd",
			},
		},
		{
			name: "one flag set",
			setupCmd: func() *cobra.Command {
				var flag1, flag2, flag3 string
				cmd := &cobra.Command{Use: "testcmd"}
				cmd.Flags().StringVar(&flag1, "flag1", "", "test flag 1")
				cmd.Flags().StringVar(&flag2, "flag2", "", "test flag 2")
				cmd.Flags().StringVar(&flag3, "flag3", "", "test flag 3")
				_ = cmd.Flags().Set("flag1", "value1")
				return cmd
			},
			expected: expected{
				cmdName: "testcmd",
				flags:   "--flag1=value1",
			},
		},
		{
			name: "two flags set",
			setupCmd: func() *cobra.Command {
				var flag1, flag2, flag3 string
				cmd := &cobra.Command{Use: "testcmd"}
				cmd.Flags().StringVar(&flag1, "flag1", "", "test flag 1")
				cmd.Flags().StringVar(&flag2, "flag2", "", "test flag 2")
				cmd.Flags().StringVar(&flag3, "flag3", "", "test flag 3")
				_ = cmd.Flags().Set("flag1", "value1")
				_ = cmd.Flags().Set("flag2", "value2")
				return cmd
			},
			expected: expected{
				cmdName: "testcmd",
				flags:   "--flag1=value1 --flag2=value2",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := tt.setupCmd()
			cmdName, flags := getCurrentCmdWithUsedFlags(cmd)
			assert.Equal(t, tt.expected.cmdName, cmdName)
			assert.Equal(t, tt.expected.flags, flags)
		})
	}
}
