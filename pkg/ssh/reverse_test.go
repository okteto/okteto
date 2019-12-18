package ssh

import (
	"context"
	"testing"

	"github.com/okteto/okteto/pkg/model"
)

func TestRemoteForwardManager_Add(t *testing.T) {
	tests := []struct {
		name           string
		add            *model.RemoteForward
		remoteForwards map[int]*remoteForward
		wantErr        bool
	}{
		{
			name:           "single",
			add:            &model.RemoteForward{Local: 8080, Remote: 8081},
			remoteForwards: map[int]*remoteForward{},
			wantErr:        false,
		},
		{
			name:           "existing",
			add:            &model.RemoteForward{Local: 8080, Remote: 8081},
			remoteForwards: map[int]*remoteForward{8080: &remoteForward{localPort: 8080, remotePort: 8081}},
			wantErr:        true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &RemoteForwardManager{
				remoteForwards: tt.remoteForwards,
				ctx:            context.TODO(),
				sshUser:        "root",
				sshHost:        "localhost",
				sshPort:        22,
			}

			if err := r.Add(tt.add); (err != nil) != tt.wantErr {
				t.Errorf("RemoteForwardManager.Add() error = %v, wantErr %v", err, tt.wantErr)
			}

			f := r.remoteForwards[8080]
			if f.localPort != 8080 {
				t.Errorf("local port is not 8080, it is: %d", f.localPort)
			}

			if f.remotePort != 8081 {
				t.Errorf("remote port is not 8081, it is: %d", f.remotePort)
			}
		})
	}
}
