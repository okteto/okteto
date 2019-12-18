package ssh

import (
	"context"
	"testing"

	"github.com/okteto/okteto/pkg/model"
)

func TestReverseManager_Add(t *testing.T) {
	tests := []struct {
		name     string
		add      *model.Reverse
		reverses map[int]*reverse
		wantErr  bool
	}{
		{
			name:     "single",
			add:      &model.Reverse{Local: 8080, Remote: 8081},
			reverses: map[int]*reverse{},
			wantErr:  false,
		},
		{
			name:     "existing",
			add:      &model.Reverse{Local: 8080, Remote: 8081},
			reverses: map[int]*reverse{8080: &reverse{localPort: 8080, remotePort: 8081}},
			wantErr:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &ReverseManager{
				reverses: tt.reverses,
				ctx:      context.TODO(),
				sshUser:  "root",
				sshHost:  "localhost",
				sshPort:  22,
			}

			if err := r.Add(tt.add); (err != nil) != tt.wantErr {
				t.Errorf("ReverseManager.Add() error = %v, wantErr %v", err, tt.wantErr)
			}

			f := r.reverses[8080]
			if f.localPort != 8080 {
				t.Errorf("local port is not 8080, it is: %d", f.localPort)
			}

			if f.remotePort != 8081 {
				t.Errorf("remote port is not 8081, it is: %d", f.remotePort)
			}
		})
	}
}
