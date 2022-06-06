package up

import (
	"context"
	"testing"

	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/ssh"
)

func TestGlobalForwarderStartsWhenRequired(t *testing.T) {
	var tests = []struct {
		name           string
		manifestInfo   []model.Forward
		expectedAnswer bool
	}{
		{
			name: "is needed global forwarding",
			manifestInfo: []model.Forward{
				{
					Local:  8080,
					Remote: 8080,
				},
			},
			expectedAnswer: true,
		},
		{
			name:           "not needed global forwarding",
			manifestInfo:   []model.Forward{},
			expectedAnswer: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			answer := isNeededGlobalForwarder(tt.manifestInfo)
			if answer != tt.expectedAnswer {
				t.Fatalf("isNeededGlobalForwarder() '%s' fail: expexted %t, got %t", tt.name, tt.expectedAnswer, answer)
			}
		})
	}
}

func TestGlobalForwarderAddsProperlyPortsToForward(t *testing.T) {
	f := ssh.NewForwardManager(context.Background(), ":8080", "0.0.0.0", "0.0.0.0", nil, "test")

	var tests = []struct {
		name          string
		upContext     *upContext
		expectedErr   bool
		expectedAdded []model.Forward
	}{
		{
			name: "add one global forwarder",
			upContext: &upContext{
				Manifest: &model.Manifest{
					GlobalForward: []model.Forward{
						{
							Local:  8080,
							Remote: 8080,
						},
					},
				},
				Forwarder: f,
			},
			expectedErr: false,
		},
		{
			name: "add two global forwarder",
			upContext: &upContext{
				Manifest: &model.Manifest{
					GlobalForward: []model.Forward{
						{
							Local:       8081,
							Service:     true,
							ServiceName: "api",
							Remote:      8080,
						},
						{
							Local:  8082,
							Remote: 8080,
						},
					},
				},
				Forwarder: f,
			},
			expectedErr: false,
		},
		{
			name: "add none global forwarder",
			upContext: &upContext{
				Manifest: &model.Manifest{
					GlobalForward: []model.Forward{},
				},
				Forwarder: f,
			},
			expectedErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := addGlobalForwards(tt.upContext)
			if err != nil && !tt.expectedErr {
				t.Fatalf("addGlobalForwards() '%s' fail: not expected err but got: %s", tt.name, err.Error())
			}

			if err == nil && tt.expectedErr {
				t.Fatalf("addGlobalForwards() '%s' fail: expected err but got 'nil'", tt.name)
			}
		})
	}
}
