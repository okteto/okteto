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

package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_ExpandStackEnvs(t *testing.T) {
	t.Setenv("ENV2", "bye")
	tests := []struct {
		name          string
		envValue      string
		expectedStack string
		file          []byte
		expectedError bool
	}{
		{
			name: "oneline envs",
			file: []byte(`
services:
    myservice:
        build:
            context: vote
            args:
                - ENV=hello
                - CUSTOM2_ENV=$CUSTOM_ENV
                - EMPTY=
                - ENV2=
        ports:
            - 8080:8080
        environment:
            FLASK_ENV: development
            CUSTOM_ENV: $CUSTOM_ENV
            ENV:
        volumes:
            - ./vote:/src
            - $CUSTOM_ENV:/src
            - ${CUSTOM_ENV}:/src
            - ${ENV:-dev}:/src

    redis:
        image: redis
        ports:
            - 6379
        volumes:
            - redis:/data

volumes:
    redis:`),
			envValue: "MYVALUE",
			expectedStack: `services:
  myservice:
    build:
      context: vote
      args:
        - ENV=hello
        - CUSTOM2_ENV=MYVALUE
        - EMPTY=
        - ENV2=
    ports:
      - 8080:8080
    environment:
      FLASK_ENV: development
      CUSTOM_ENV: MYVALUE
      ENV:
    volumes:
      - ./vote:/src
      - MYVALUE:/src
      - MYVALUE:/src
      - dev:/src
  redis:
    image: redis
    ports:
      - 6379
    volumes:
      - redis:/data
volumes:
  redis:
`,
			expectedError: false,
		},
		{
			name: "multiline envs",
			file: []byte(`
services:
    myservice:
        build:
            context: vote
            args:
                - CUSTOM2_ENV=$CUSTOM_ENV
        ports:
            - 8080:8080
        environment:
            FLASK_ENV: development
            CUSTOM_ENV: $CUSTOM_ENV
            CUSTOM2_ENV:
        volumes:
            - ./myservice:/src

    redis:
        image: redis
        ports:
            - 6379
        volumes:
            - redis:/data

volumes:
    redis:`),
			envValue: "my first line\nmy second line",
			expectedStack: `services:
  myservice:
    build:
      context: vote
      args:
        - |-
          CUSTOM2_ENV=my first line
          my second line
    ports:
      - 8080:8080
    environment:
      FLASK_ENV: development
      CUSTOM_ENV: |-
        my first line
        my second line
      CUSTOM2_ENV:
    volumes:
      - ./myservice:/src
  redis:
    image: redis
    ports:
      - 6379
    volumes:
      - redis:/data
volumes:
  redis:
`,
			expectedError: false,
		},
	}

	for _, tt := range tests {

		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("CUSTOM_ENV", tt.envValue)
			result, err := ExpandStackEnvs(tt.file)
			if err != nil && !tt.expectedError {
				t.Fatalf("expected no error, but got error: %v", err)
			} else if err == nil && tt.expectedError {
				t.Fatalf("expected error, but got nil")
			}

			assert.Equal(t, tt.expectedStack, string(result))

		})
	}
}

func TestStackEscapedEnvironment(t *testing.T) {
	t.Setenv("ESCAPED_TARGET", "host-value")
	t.Setenv("EXPANSION_SOURCE", "${ESCAPED_TARGET}")
	t.Setenv("UNSET_TARGET", "")
	for name, environment := range map[string]string{
		"map":  "      LITERAL: $${ESCAPED_TARGET}\n      INDIRECT: ${EXPANSION_SOURCE}\n      NORMAL: ${ESCAPED_TARGET}\n      UNSET: $${UNSET_TARGET}\n      MULTILINE: |\n        prefix\n        $${ESCAPED_TARGET}\n",
		"list": "      - LITERAL=$${ESCAPED_TARGET}\n      - INDIRECT=${EXPANSION_SOURCE}\n      - NORMAL=${ESCAPED_TARGET}\n      - UNSET=$${UNSET_TARGET}\n      - |\n        MULTILINE=prefix\n        $${ESCAPED_TARGET}\n",
	} {
		for mode, compose := range map[string]bool{"compose": true, "stack": false} {
			t.Run(name+"/"+mode, func(t *testing.T) {
				s, err := ReadStack([]byte("services:\n  app:\n    image: nginx\n    environment:\n"+environment), compose)
				if !assert.NoError(t, err) {
					return
				}
				values := map[string]string{}
				for _, variable := range s.Services["app"].Environment {
					values[variable.Name] = variable.Value
				}
				assert.Equal(t, "${ESCAPED_TARGET}", values["LITERAL"])
				assert.Equal(t, "${ESCAPED_TARGET}", values["INDIRECT"])
				assert.Equal(t, "host-value", values["NORMAL"])
				assert.Equal(t, "${UNSET_TARGET}", values["UNSET"])
				assert.Equal(t, "prefix\n${ESCAPED_TARGET}\n", values["MULTILINE"])
			})
		}
	}
}
