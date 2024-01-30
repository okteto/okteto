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

package io

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSetOutputFormatOutput(t *testing.T) {
	buffer := bytes.NewBuffer([]byte{})
	l := newOutputController(buffer)
	require.IsType(t, &ttyFormatter{}, l.formatter)

	l.SetOutputFormat("plain")
	require.IsType(t, &plainFormatter{}, l.formatter)

	l.SetOutputFormat("json")
	require.IsType(t, &jsonFormatter{}, l.formatter)

	l.SetOutputFormat("tty")
	require.IsType(t, &ttyFormatter{}, l.formatter)

	l.SetOutputFormat("invalid")
	require.IsType(t, &ttyFormatter{}, l.formatter)
}

func TestPrintln(t *testing.T) {
	buffer := bytes.NewBuffer([]byte{})
	l := newOutputController(buffer)
	l.spinner = newNoSpinner("test", l)

	l.SetOutputFormat("tty")
	l.Println("test")
	require.Equal(t, "test\n", buffer.String())
	buffer.Reset()

	l.SetOutputFormat("plain")
	l.Println("test")
	require.Equal(t, "test\n", buffer.String())
	buffer.Reset()

	l.SetOutputFormat("json")
	l.SetStage("test")
	l.Println("test")
	jsonMessage := &jsonMessage{}
	err := json.Unmarshal(buffer.Bytes(), jsonMessage)
	require.NoError(t, err)
	require.Equal(t, "test", jsonMessage.Message)
	require.Equal(t, "test", jsonMessage.Stage)
	require.Equal(t, "info", jsonMessage.Level)
	require.NotEmpty(t, jsonMessage.Timestamp)
	buffer.Reset()

	l.SetStage("")
	l.Println("test")
	require.Equal(t, "", buffer.String())
}

func TestPrint(t *testing.T) {
	buffer := bytes.NewBuffer([]byte{})
	l := newOutputController(buffer)
	l.spinner = newNoSpinner("test", l)

	l.SetOutputFormat("tty")
	l.Print("test")
	require.Equal(t, "test", buffer.String())
	buffer.Reset()

	l.SetOutputFormat("plain")
	l.Print("test")
	require.Equal(t, "test", buffer.String())
	buffer.Reset()

	l.SetOutputFormat("json")
	l.SetStage("test")
	l.Print("test")
	jsonMessage := &jsonMessage{}
	err := json.Unmarshal(buffer.Bytes(), jsonMessage)
	require.NoError(t, err)
	require.Equal(t, "test", jsonMessage.Message)
	require.Equal(t, "test", jsonMessage.Stage)
	require.Equal(t, "info", jsonMessage.Level)
	require.NotEmpty(t, jsonMessage.Timestamp)
	buffer.Reset()

	l.SetStage("")
	l.Print("test")
	require.Equal(t, "", buffer.String())
}

func TestPrintf(t *testing.T) {
	buffer := bytes.NewBuffer([]byte{})
	l := newOutputController(buffer)

	l.spinner = newNoSpinner("test", l)

	l.SetOutputFormat("tty")
	l.Printf("%s", "test")
	require.Equal(t, "test", buffer.String())
	buffer.Reset()

	l.SetOutputFormat("plain")
	l.Printf("%s", "test")
	require.Equal(t, "test", buffer.String())
	buffer.Reset()

	l.SetOutputFormat("json")
	l.SetStage("test")
	l.Printf("%s", "test")
	jsonMessage := &jsonMessage{}
	err := json.Unmarshal(buffer.Bytes(), jsonMessage)
	require.NoError(t, err)
	require.Equal(t, "test", jsonMessage.Message)
	require.Equal(t, "test", jsonMessage.Stage)
	require.Equal(t, "info", jsonMessage.Level)
	require.NotEmpty(t, jsonMessage.Timestamp)
	buffer.Reset()

	l.SetStage("")
	l.Printf("%s", "test")
	require.Equal(t, "", buffer.String())
}

func TestInfof(t *testing.T) {
	buffer := bytes.NewBuffer([]byte{})
	l := newOutputController(buffer)

	l.spinner = newNoSpinner("test", l)

	l.SetOutputFormat("tty")
	l.Infof("%s", "test")
	require.Equal(t, " i  test\n", buffer.String())
	buffer.Reset()

	l.SetOutputFormat("plain")
	l.Infof("%s", "test")
	require.Equal(t, "INFO: test\n", buffer.String())
	buffer.Reset()

	l.SetOutputFormat("json")
	l.SetStage("test")
	l.Infof("%s", "test")
	jsonMessage := &jsonMessage{}
	err := json.Unmarshal(buffer.Bytes(), jsonMessage)
	require.NoError(t, err)
	require.Equal(t, "INFO: test", jsonMessage.Message)
	require.Equal(t, "test", jsonMessage.Stage)
	require.Equal(t, "info", jsonMessage.Level)
	require.NotEmpty(t, jsonMessage.Timestamp)
	buffer.Reset()

	l.SetStage("")
	l.Infof("%s", "test")
	require.Equal(t, "", buffer.String())
}

func TestSuccess(t *testing.T) {
	buffer := bytes.NewBuffer([]byte{})
	l := newOutputController(buffer)

	l.spinner = newNoSpinner("test", l)

	l.SetOutputFormat("tty")
	l.Success("%s", "test")
	require.Equal(t, " ✓  test\n", buffer.String())
	buffer.Reset()

	l.SetOutputFormat("plain")
	l.Success("%s", "test")
	require.Equal(t, "SUCCESS: test\n", buffer.String())
	buffer.Reset()

	l.SetOutputFormat("json")
	l.SetStage("test")
	l.Success("%s", "test")
	jsonMessage := &jsonMessage{}
	err := json.Unmarshal(buffer.Bytes(), jsonMessage)
	require.NoError(t, err)
	require.Equal(t, "SUCCESS: test", jsonMessage.Message)
	require.Equal(t, "test", jsonMessage.Stage)
	require.Equal(t, "info", jsonMessage.Level)
	require.NotEmpty(t, jsonMessage.Timestamp)
	buffer.Reset()

	l.SetStage("")
	l.Success("%s", "test")
	require.Equal(t, "", buffer.String())
}

type fakeSpinner struct {
	message string
	on      bool
}

func (s *fakeSpinner) Start() {
	s.on = true
}

func (s *fakeSpinner) Stop() {
	s.on = false
}

func (s *fakeSpinner) isActive() bool {
	return s.on
}

func (s *fakeSpinner) getMessage() string {
	return s.message
}

func TestSpinner(t *testing.T) {
	buffer := bytes.NewBuffer([]byte{})
	l := newOutputController(buffer)
	fakeSpinner := &fakeSpinner{}
	l.spinner = fakeSpinner

	sp := l.Spinner("")
	require.Equal(t, fakeSpinner, sp)

	sp = l.Spinner("test")
	require.IsType(t, &ttySpinner{}, sp)

	t.Setenv(OktetoDisableSpinnerEnvVar, "1")
	sp = l.Spinner("disabled")
	require.IsType(t, &noSpinner{}, sp)

	t.Setenv(OktetoDisableSpinnerEnvVar, "test")
	sp = l.Spinner("enabled")
	require.IsType(t, &ttySpinner{}, sp)
}
