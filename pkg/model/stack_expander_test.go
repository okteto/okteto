package model

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_ExpandStackEnvs(t *testing.T) {
	tests := []struct {
		name          string
		file          []byte
		envValue      string
		expectedStack string
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
            - $CUSTOM_ENV
            - CUSTOM2_ENV=$CUSTOM_ENV
        ports:
            - 8080:8080
        environment:
            FLASK_ENV: development
            CUSTOM_ENV: $CUSTOM_ENV
            $CUSTOM2_ENV:
        volumes:
            - ./vote:/src

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
      - CUSTOM_ENV=MYVALUE
      - CUSTOM2_ENV=MYVALUE
    ports:
      - 8080:8080
    environment:
      FLASK_ENV: development
      CUSTOM_ENV: MYVALUE
      CUSTOM2_ENV:
    volumes:
      - ./vote:/src
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
            - $CUSTOM_ENV
            - CUSTOM2_ENV=$CUSTOM_ENV
        ports:
            - 8080:8080
        environment:
            FLASK_ENV: development
            CUSTOM_ENV: $CUSTOM_ENV
            $CUSTOM2_ENV:
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
        CUSTOM_ENV=my first line
        my second line
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
			if err := os.Setenv("CUSTOM_ENV", tt.envValue); err != nil {
				t.Fatal(err)
			}
			result, err := ExpandStackEnvs(tt.file)
			if err != nil && !tt.expectedError {
				t.Fatalf("expected no error, but got error: %v", err)
			} else if err == nil && tt.expectedError {
				t.Fatalf("expected error, but got nil")
			}

			assert.Equal(t, tt.expectedStack, string(result))

			os.Unsetenv("CUSTOM_ENV")
		})
	}
}
