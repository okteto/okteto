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

package model

import (
	"testing"

	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func Test_ReadStack(t *testing.T) {
	manifest := []byte(`name: voting-app
services:
  vote:
    public: true
    image: okteto/vote:1
    build: vote
    command: python app.py
    environment:
      - OPTION_A=Cats
      - OPTION_B=Dogs
    ports:
      - 80
    replicas: 2
    stop_grace_period: 5
    resources:
      cpu: 100m
      memory: 258Mi
  db:
    image: postgres:9.4
    resources:
      storage: 1Gi
    volumes:
      - /var/lib/postgresql/data`)
	s, err := ReadStack(manifest)
	if err != nil {
		t.Fatal(err)
	}

	if s.Name != "voting-app" {
		t.Errorf("wrong stack name '%s'", s.Name)
	}
	if len(s.Services) != 2 {
		t.Errorf("'services' was not parsed: %+v", s)
	}
	if _, ok := s.Services["vote"]; !ok {
		t.Errorf("'vote' was not parsed: %+v", s)
	}
	if !s.Services["vote"].Public {
		t.Errorf("'vote.public' was not parsed: %+v", s)
	}
	if s.Services["vote"].Image != "okteto/vote:1" {
		t.Errorf("'vote.image' was not parsed: %+v", s)
	}
	if s.Services["vote"].Build != "vote" {
		t.Errorf("'vote.build' was not parsed: %+v", s)
	}
	if s.Services["vote"].Command != "python app.py" {
		t.Errorf("'vote.command' was not parsed: %+v", s)
	}
	if s.Services["vote"].Replicas != 2 {
		t.Errorf("'vote.replicas' was not parsed: %+v", s)
	}
	if len(s.Services["vote"].Environment) != 2 {
		t.Errorf("'vote.env' was not parsed: %+v", s)
	}
	if s.Services["vote"].Environment[0].Name != "OPTION_A" || s.Services["vote"].Environment[0].Value != "Cats" {
		t.Errorf("'vote.env[0]' was not parsed: %+v", s)
	}
	if s.Services["vote"].Environment[1].Name != "OPTION_B" || s.Services["vote"].Environment[1].Value != "Dogs" {
		t.Errorf("'vote.env[1]' was not parsed: %+v", s)
	}
	if len(s.Services["vote"].Ports) != 1 {
		t.Errorf("'vote.ports' was not parsed: %+v", s)
	}
	if s.Services["vote"].Ports[0] != 80 {
		t.Errorf("'vote.ports[0]' was not parsed: %+v", s)
	}
	if s.Services["vote"].StopGracePeriod != 5 {
		t.Errorf("'vote.stop_graace_period' was not parsed: %+v", s)
	}
	cpu := s.Services["vote"].Resources[apiv1.ResourceCPU]
	if cpu.Cmp(resource.MustParse("100m")) != 0 {
		t.Errorf("'vote.resources.cpu' was not parsed: %+v", s)
	}
	memory := s.Services["vote"].Resources[apiv1.ResourceMemory]
	if memory.Cmp(resource.MustParse("258Mi")) != 0 {
		t.Errorf("'vote.resources.cpu' was not parsed: %+v", s)
	}
	if _, ok := s.Services["db"]; !ok {
		t.Errorf("'db' was not parsed: %+v", s)
	}
	if s.Services["db"].Image != "postgres:9.4" {
		t.Errorf("'db.image' was not parsed: %+v", s)
	}
	if s.Services["db"].Replicas != 1 {
		t.Errorf("'db.replicas' was not parsed: %+v", s)
	}
	if len(s.Services["db"].Volumes) != 1 {
		t.Errorf("'db.volumes' was not parsed: %+v", s)
	}
	if s.Services["db"].Volumes[0] != "/var/lib/postgresql/data" {
		t.Errorf("'db.volumes[0]' was not parsed: %+v", s)
	}
	storage := s.Services["db"].Resources[apiv1.ResourceStorage]
	if storage.Cmp(resource.MustParse("1Gi")) != 0 {
		t.Errorf("'db.resources.storage' was not parsed: %+v", s)
	}
}

func TestStack_validate(t *testing.T) {
	tests := []struct {
		name  string
		stack *Stack
	}{
		{
			name:  "empty-name",
			stack: &Stack{},
		},
		{
			name: "bad-name",
			stack: &Stack{
				Name: "-bad-name",
			},
		},
		{
			name: "empty-services",
			stack: &Stack{
				Name: "name",
			},
		},
		{
			name: "empty-service-name",
			stack: &Stack{
				Name: "name",
				Services: map[string]Service{
					"": Service{},
				},
			},
		},
		{
			name: "bad-service-name",
			stack: &Stack{
				Name: "name",
				Services: map[string]Service{
					"-bad-name": Service{},
				},
			},
		},
		{
			name: "empty-service-image",
			stack: &Stack{
				Name: "name",
				Services: map[string]Service{
					"name": Service{},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.stack.validate(); err == nil {
				t.Errorf("Stack.validate() not failed for test '%s'", tt.name)
			}
		})
	}
}

func Test_validateStackName(t *testing.T) {
	tests := []struct {
		name      string
		stackName string
		wantErr   bool
	}{
		{name: "empty", stackName: "", wantErr: true},
		{name: "starts-with-dash", stackName: "-bad-name", wantErr: true},
		{name: "ends-with-dash", stackName: "bad-name-", wantErr: true},
		{name: "symbols", stackName: "1$good-2", wantErr: true},
		{name: "alphanumeric", stackName: "good-2", wantErr: false},
		{name: "good", stackName: "good-name", wantErr: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := validateStackName(tt.stackName); (err != nil) != tt.wantErr {
				t.Errorf("Stack.validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
