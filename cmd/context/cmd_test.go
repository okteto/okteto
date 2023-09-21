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

package context

import (
	"testing"
)

func Test_NoArgsAcceptedCtx(t *testing.T) {
	cmd := Show()
	cmd.SetArgs([]string{"args"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("Args not supported")
	}
}

func Test_NoArgsAcceptedShow(t *testing.T) {
	cmd := Context(nil)
	cmd.SetArgs([]string{"args"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("Args not supported")
	}
}

func Test_NoArgsAcceptedList(t *testing.T) {
	cmd := List()
	cmd.SetArgs([]string{"args"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("Args not supported")
	}
}

func Test_NoArgsAcceptedUpdateKubeConfig(t *testing.T) {
	cmd := UpdateKubeconfigCMD(nil)
	cmd.SetArgs([]string{"args"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("MaxArgs not supported")
	}
}

func Test_MaxArgsCreate(t *testing.T) {
	cmd := CreateCMD()
	cmd.SetArgs([]string{"args", "args"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("MaxArgs not supported")
	}
}

func Test_MaxArgsUse(t *testing.T) {
	cmd := Use()
	cmd.SetArgs([]string{"args", "args"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("Args not supported")
	}
}

func Test_ExactArgsDelete(t *testing.T) {
	cmd := DeleteCMD()
	cmd.SetArgs([]string{})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("ExactArgs not supported")
	}
}
