package up

import (
	"context"
	"testing"

	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/ssh"
	"github.com/stretchr/testify/assert"
)

func TestGlobalForwarderStartsWhenRequired(t *testing.T) {
	var tests = []struct {
		name             string
		globalFwdSection []model.GlobalForward
		expectedAnswer   bool
	}{
		{
			name: "is needed global forwarding",
			globalFwdSection: []model.GlobalForward{
				{
					Local:  8080,
					Remote: 8080,
				},
			},
			expectedAnswer: true,
		},
		{
			name:             "not needed global forwarding",
			globalFwdSection: []model.GlobalForward{},
			expectedAnswer:   false,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			answer := isNeededGlobalForwarder(tt.globalFwdSection)
			assert.Equal(t, answer, tt.expectedAnswer)
		})
	}
}

func TestGlobalForwarderAddsProperlyPortsToForward(t *testing.T) {
	f := ssh.NewForwardManager(context.Background(), ":8080", "0.0.0.0", "0.0.0.0", nil, "test")

	var tests = []struct {
		name        string
		upContext   *upContext
		expectedErr bool
	}{
		{
			name: "add one global forwarder",
			upContext: &upContext{
				Manifest: &model.Manifest{
					GlobalForward: []model.GlobalForward{
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
					GlobalForward: []model.GlobalForward{
						{
							Local:       8081,
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
					GlobalForward: []model.GlobalForward{},
				},
				Forwarder: f,
			},
			expectedErr: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := addGlobalForwards(tt.upContext)
			if !tt.expectedErr {
				assert.Nil(t, err)
			} else {
				assert.NotNil(t, err)
			}
		})
	}
}
