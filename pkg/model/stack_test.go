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
      storage: 1Gi
  db:
    image: postgres:9.4
    resources:
      requests:
        memory: 128Mi
        storage:
          size: 1Gi
          class: standard
    command: e
    args: c
    volumes:
      - /var/lib/postgresql/data`)
	s, err := ReadStack(manifest, false)
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
	if s.Services["vote"].Build.Context != "vote" {
		t.Errorf("'vote.build' was not parsed: %+v", s.Services["vote"].Build)
	}
	if len(s.Services["vote"].Entrypoint.Values) != 2 {
		t.Errorf("'vote.entrypoint' was not parsed: %+v", s.Services["vote"].Entrypoint.Values)
	}
	if s.Services["vote"].Entrypoint.Values[0] != "python" && s.Services["vote"].Entrypoint.Values[0] != "app.py" {
		t.Errorf("'vote.entrypoint' was not parsed: %+v", s.Services["vote"].Entrypoint.Values)
	}
	if s.Services["vote"].Replicas != 2 {
		t.Errorf("'vote.deploy.replicas' was not parsed: %+v", s)
	}
	if len(s.Services["vote"].Environment) != 2 {
		t.Errorf("'vote.env' was not parsed: %+v", s)
	}
	for _, env := range s.Services["vote"].Environment {
		if env.Name == "OPTION_A" && env.Value == "Cats" {
			continue
		} else if env.Name == "OPTION_B" && env.Value == "Dogs" {
			continue
		} else {
			t.Errorf("'vote.env' was not parsed correctly: %+v", s.Services["vote"].Environment)
		}
	}
	if len(s.Services["vote"].Ports) != 1 {
		t.Errorf("'vote.ports' was not parsed: %+v", s)
	}
	if s.Services["vote"].Ports[0].Port != 80 {
		t.Errorf("'vote.ports[0]' was not parsed: %+v", s)
	}
	if s.Services["vote"].StopGracePeriod != 5 {
		t.Errorf("'vote.stop_grace_period' was not parsed: %+v", s)
	}

	cpu := s.Services["vote"].Resources.Limits.CPU.Value
	if cpu.Cmp(resource.MustParse("100m")) != 0 {
		t.Errorf("'vote.deploy.limits.cpu' was not parsed: %+v", s)
	}

	memory := s.Services["vote"].Resources.Limits.Memory.Value
	if memory.Cmp(resource.MustParse("258Mi")) != 0 {
		t.Errorf("'vote.deploy.limits.memory' was not parsed: %+v", s)
	}
	storage := s.Services["vote"].Resources.Requests.Storage.Size.Value
	if storage.Cmp(resource.MustParse("1Gi")) != 0 {
		t.Errorf("'vote.resources.storage' was not parsed: %+v", s)
	}
	if _, ok := s.Services["db"]; !ok {
		t.Errorf("'db' was not parsed: %+v", s)
	}
	if s.Services["db"].Image != "postgres:9.4" {
		t.Errorf("'db.image' was not parsed: %+v", s)
	}
	if s.Services["db"].Replicas != 1 {
		t.Errorf("'db.deploy.replicas' was not parsed: %+v", s)
	}
	if len(s.Services["db"].Entrypoint.Values) != 1 {
		t.Errorf("'db.entrypoint' was not parsed: %+v", s.Services["db"].Entrypoint.Values)
	}
	if s.Services["db"].Entrypoint.Values[0] != "e" {
		t.Errorf("'db.entrypoint' was not parsed: %+v", s.Services["db"].Entrypoint.Values)
	}
	if len(s.Services["db"].Command.Values) != 1 {
		t.Errorf("'db.command' was not parsed: %+v", s.Services["db"].Command.Values)
	}
	if s.Services["db"].Command.Values[0] != "c" {
		t.Errorf("'db.command' was not parsed: %+v", s.Services["db"].Command.Values)
	}

	if len(s.Services["db"].Volumes) != 1 {
		t.Errorf("'db.volumes' was not parsed: %+v", s)
	}
	if s.Services["db"].Volumes[0].RemotePath != "/var/lib/postgresql/data" {
		t.Errorf("'db.volumes[0]' was not parsed: %+v", s)
	}
	storage = s.Services["db"].Resources.Requests.Storage.Size.Value
	if storage.Cmp(resource.MustParse("1Gi")) != 0 {
		t.Errorf("'db.resources.storage.size' was not parsed: %+v", s)
	}
	if s.Services["db"].Resources.Requests.Storage.Class != "standard" {
		t.Errorf("'db.resources.storage.class' was not parsed: %+v", s)
	}
	memory = s.Services["db"].Resources.Requests.Memory.Value
	if memory.Cmp(resource.MustParse("128Mi")) != 0 {
		t.Errorf("'vote.resources.memory' was not parsed: %+v", s)
	}
}

func Test_ReadStackCompose(t *testing.T) {
	manifest := []byte(`name: voting-app
services:
  vote:
    public: true
    image: okteto/vote:1
    build: vote
    entrypoint: python app.py
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
      storage: 1Gi
    labels:
      - traeffick.routes=Path("/")
  db:
    image: postgres:9.4
    resources:
      requests:
        memory: 128Mi
        storage:
          size: 1Gi
          class: standard
    entrypoint: e
    command: c
    volumes:
      - /var/lib/postgresql/data`)
	s, err := ReadStack(manifest, true)
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
	if s.Services["vote"].Build.Context != "vote" {
		t.Errorf("'vote.build' was not parsed: %+v", s.Services["vote"].Build)
	}
	if len(s.Services["vote"].Entrypoint.Values) != 2 {
		t.Errorf("'vote.entrypoint' was not parsed: %+v", s.Services["vote"].Entrypoint.Values)
	}
	if s.Services["vote"].Entrypoint.Values[0] != "python" && s.Services["vote"].Entrypoint.Values[0] != "app.py" {
		t.Errorf("'vote.entrypoint' was not parsed: %+v", s.Services["vote"].Entrypoint.Values)
	}
	if s.Services["vote"].Replicas != 2 {
		t.Errorf("'vote.deploy.replicas' was not parsed: %+v", s)
	}
	if len(s.Services["vote"].Environment) != 2 {
		t.Errorf("'vote.env' was not parsed: %+v", s)
	}
	for _, env := range s.Services["vote"].Environment {
		if env.Name == "OPTION_A" && env.Value == "Cats" {
			continue
		} else if env.Name == "OPTION_B" && env.Value == "Dogs" {
			continue
		} else {
			t.Errorf("'vote.env' was not parsed correctly: %+v", s.Services["vote"].Environment)
		}
	}
	if len(s.Services["vote"].Ports) != 1 {
		t.Errorf("'vote.ports' was not parsed: %+v", s)
	}
	if s.Services["vote"].Ports[0].Port != 80 {
		t.Errorf("'vote.ports[0]' was not parsed: %+v", s)
	}
	if s.Services["vote"].StopGracePeriod != 5 {
		t.Errorf("'vote.stop_grace_period' was not parsed: %+v", s)
	}

	cpu := s.Services["vote"].Resources.Limits.CPU.Value
	if cpu.Cmp(resource.MustParse("100m")) != 0 {
		t.Errorf("'vote.deploy.limits.cpu' was not parsed: %+v", s)
	}

	memory := s.Services["vote"].Resources.Limits.Memory.Value
	if memory.Cmp(resource.MustParse("258Mi")) != 0 {
		t.Errorf("'vote.deploy.limits.memory' was not parsed: %+v", s)
	}
	storage := s.Services["vote"].Resources.Requests.Storage.Size.Value
	if storage.Cmp(resource.MustParse("1Gi")) != 0 {
		t.Errorf("'vote.resources.storage' was not parsed: %+v", s)
	}
	for key, value := range s.Services["vote"].Annotations {
		if key == "traeffick.routes" && value == `Path("/")` {
			continue
		} else {
			t.Errorf("'vote.annotations' was not parsed correctly: %+v", s.Services["vote"].Annotations)
		}
	}

	if len(s.Services["vote"].Labels) > 0 {
		t.Errorf("'vote.labels' has labels inside")
	}
	if _, ok := s.Services["db"]; !ok {
		t.Errorf("'db' was not parsed: %+v", s)
	}
	if s.Services["db"].Image != "postgres:9.4" {
		t.Errorf("'db.image' was not parsed: %+v", s)
	}
	if s.Services["db"].Replicas != 1 {
		t.Errorf("'db.deploy.replicas' was not parsed: %+v", s)
	}
	if len(s.Services["db"].Entrypoint.Values) != 1 {
		t.Errorf("'db.entrypoint' was not parsed: %+v", s.Services["db"].Entrypoint.Values)
	}
	if s.Services["db"].Entrypoint.Values[0] != "e" {
		t.Errorf("'db.entrypoint' was not parsed: %+v", s.Services["db"].Entrypoint.Values)
	}
	if len(s.Services["db"].Command.Values) != 1 {
		t.Errorf("'db.command' was not parsed: %+v", s.Services["db"].Command.Values)
	}
	if s.Services["db"].Command.Values[0] != "c" {
		t.Errorf("'db.command' was not parsed: %+v", s.Services["db"].Command.Values)
	}

	if len(s.Services["db"].Volumes) != 1 {
		t.Errorf("'db.volumes' was not parsed: %+v", s)
	}
	if s.Services["db"].Volumes[0].RemotePath != "/var/lib/postgresql/data" {
		t.Errorf("'db.volumes[0]' was not parsed: %+v", s)
	}
	storage = s.Services["db"].Resources.Requests.Storage.Size.Value
	if storage.Cmp(resource.MustParse("1Gi")) != 0 {
		t.Errorf("'db.resources.storage.size' was not parsed: %+v", s)
	}
	if s.Services["db"].Resources.Requests.Storage.Class != "standard" {
		t.Errorf("'db.resources.storage.class' was not parsed: %+v", s)
	}
	memory = s.Services["db"].Resources.Requests.Memory.Value
	if memory.Cmp(resource.MustParse("128Mi")) != 0 {
		t.Errorf("'vote.resources.memory' was not parsed: %+v", s)
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
				Services: map[string]*Service{
					"": {},
				},
			},
		},
		{
			name: "bad-service-name",
			stack: &Stack{
				Name: "name",
				Services: map[string]*Service{
					"-bad-name": {},
				},
			},
		},
		{
			name: "empty-service-image",
			stack: &Stack{
				Name: "name",
				Services: map[string]*Service{
					"name": {},
				},
			},
		},
		{
			name: "relative-volume-path",
			stack: &Stack{
				Name: "name",
				Services: map[string]*Service{
					"name": {
						Volumes: []StackVolume{{RemotePath: "relative"}},
					},
				},
			},
		},
		{
			name: "volume-bind-mount",
			stack: &Stack{
				Name: "name",
				Services: map[string]*Service{
					"name": {
						Volumes: []StackVolume{{LocalPath: "/source", RemotePath: "/dest"}},
					},
				},
			},
		},
		{
			name: "endpoint-of-unexported-port",
			stack: &Stack{
				Name: "name",
				Endpoints: map[string]Endpoint{
					"endpoint1": {
						Rules: []EndpointRule{
							{Service: "app",
								Port: 80},
						},
					},
				},
				Services: map[string]*Service{
					"app": {Image: "test",
						Ports: []Port{
							{
								Port: 8080,
							},
						}},
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
