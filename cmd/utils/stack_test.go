// Copyright 2022 The Okteto Authors
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

package utils

import (
	"log"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/okteto/okteto/pkg/model"
)

const (
	firstStack = `services:
  app:
    environment:
      a: b
    image: ${OKTETO_BUILD_APP_IMAGE}
`
	secondStack = `services:
  app:
    labels:
      a: b
    image: ${OKTETO_BUILD_APP_IMAGE}
`
)

func Test_multipleStack(t *testing.T) {
	dir, err := os.MkdirTemp("", "")
	if err != nil {
		t.Fatal(err)
	}
	log.Printf("created tempdir: %s", dir)

	path, err := createFile(dir, "docker-compose.yml", firstStack)
	if err != nil {
		t.Fatal(err)
	}
	paths := []string{path}

	path, err = createFile(dir, "okteto-stack.yml", secondStack)
	if err != nil {
		t.Fatal(err)
	}
	paths = append(paths, path)

	stack, err := model.LoadStack("", paths, false)
	if err != nil {
		t.Fatal(err)
	}
	var svcResult = &model.Service{
		Environment: model.Environment{
			model.EnvVar{
				Name:  "a",
				Value: "b",
			},
		},
		Labels: model.Labels{
			"a": "b",
		},
		Image: "",
	}
	svc := stack.Services["app"]

	if !reflect.DeepEqual(svc.Environment, svcResult.Environment) {
		t.Fatalf("Expected %v but got %v", svcResult.Environment, svc.Environment)
	}
	if !reflect.DeepEqual(svc.Labels, svcResult.Labels) {
		t.Fatalf("Expected %v but got %v", svcResult.Labels, svc.Labels)
	}
	if svc.Image != svcResult.Image {
		t.Fatalf("Expected %v but got %v", svcResult.Image, svc.Image)
	}

	os.Setenv("OKTETO_BUILD_APP_IMAGE", "test")
	svcResult.Image = "test"

	stack, err = model.LoadStack("", paths, true)
	if err != nil {
		t.Fatal(err)
	}

	svc = stack.Services["app"]

	if !reflect.DeepEqual(svc.Environment, svcResult.Environment) {
		t.Fatalf("Expected %v but got %v", svcResult.Environment, svc.Environment)
	}
	if !reflect.DeepEqual(svc.Labels, svcResult.Labels) {
		t.Fatalf("Expected %v but got %v", svcResult.Labels, svc.Labels)
	}
	if svc.Image != svcResult.Image {
		t.Fatalf("Expected %v but got %v", svcResult.Image, svc.Image)
	}
}

func Test_overrideFileStack(t *testing.T) {
	dir, err := os.MkdirTemp("", "")
	if err != nil {
		t.Fatal(err)
	}
	log.Printf("created tempdir: %s", dir)

	path, err := createFile(dir, "docker-compose.yml", firstStack)
	if err != nil {
		t.Fatal(err)
	}
	paths := []string{path}

	_, err = createFile(dir, "docker-compose.override.yml", secondStack)
	if err != nil {
		t.Fatal(err)
	}

	stack, err := model.LoadStack("", paths, true)
	if err != nil {
		t.Fatal(err)
	}

	var svcResult = &model.Service{
		Environment: model.Environment{
			model.EnvVar{
				Name:  "a",
				Value: "b",
			},
		},
		Annotations: model.Annotations{
			"a": "b",
		},
		Image: "test",
	}

	svc := stack.Services["app"]

	if !reflect.DeepEqual(svc.Environment, svcResult.Environment) {
		t.Fatalf("Expected %v but got %v", svcResult.Environment, svc.Environment)
	}
	if !reflect.DeepEqual(svc.Annotations, svcResult.Annotations) {
		t.Fatalf("Expected %v but got %v", svcResult.Labels, svc.Labels)
	}
	if svc.Image != svcResult.Image {
		t.Fatalf("Expected %v but got %v", svcResult.Image, svc.Image)
	}
}

func createFile(dir, name, content string) (string, error) {
	dockerfilePath := filepath.Join(dir, name)
	dockerfileContent := []byte(content)
	if err := os.WriteFile(dockerfilePath, dockerfileContent, 0644); err != nil {
		return dockerfilePath, err
	}
	return dockerfilePath, nil
}
