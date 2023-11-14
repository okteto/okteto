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

package syncthing

import (
	"testing"
)

func Test_needsDatabaseReset(t *testing.T) {
	tests := []struct {
		wfc                       *waitForCompletion
		name                      string
		previousLocalGlobalBytes  int64
		previousRemoteGlobalBytes int64
		globalBytesRetries        int64
		want                      bool
	}{
		{
			name: "global-bytes-ok",
			wfc: &waitForCompletion{
				localCompletion: &Completion{
					GlobalBytes: 10,
				},
				remoteCompletion: &Completion{
					GlobalBytes: 10,
				},
				previousLocalGlobalBytes:  0,
				previousRemoteGlobalBytes: 0,
				globalBytesRetries:        10,
			},
			previousLocalGlobalBytes:  10,
			previousRemoteGlobalBytes: 10,
			globalBytesRetries:        0,
			want:                      false,
		},
		{
			name: "local-global-bytes-changed",
			wfc: &waitForCompletion{
				localCompletion: &Completion{
					GlobalBytes: 10,
				},
				remoteCompletion: &Completion{
					GlobalBytes: 20,
				},
				previousLocalGlobalBytes:  1,
				previousRemoteGlobalBytes: 20,
				globalBytesRetries:        10,
			},
			previousLocalGlobalBytes:  10,
			previousRemoteGlobalBytes: 20,
			globalBytesRetries:        0,
			want:                      false,
		},
		{
			name: "remote-global-bytes-changed",
			wfc: &waitForCompletion{
				localCompletion: &Completion{
					GlobalBytes: 10,
				},
				remoteCompletion: &Completion{
					GlobalBytes: 20,
				},
				previousLocalGlobalBytes:  10,
				previousRemoteGlobalBytes: 2,
				globalBytesRetries:        10,
			},
			previousLocalGlobalBytes:  10,
			previousRemoteGlobalBytes: 20,
			globalBytesRetries:        0,
			want:                      false,
		},
		{
			name: "increment-global-bytes-retries",
			wfc: &waitForCompletion{
				localCompletion: &Completion{
					GlobalBytes: 10,
				},
				remoteCompletion: &Completion{
					GlobalBytes: 20,
				},
				previousLocalGlobalBytes:  10,
				previousRemoteGlobalBytes: 20,
				globalBytesRetries:        10,
			},
			previousLocalGlobalBytes:  10,
			previousRemoteGlobalBytes: 20,
			globalBytesRetries:        11,
			want:                      false,
		},
		{
			name: "reset",
			wfc: &waitForCompletion{
				localCompletion: &Completion{
					GlobalBytes: 10,
				},
				remoteCompletion: &Completion{
					GlobalBytes: 20,
				},
				previousLocalGlobalBytes:  10,
				previousRemoteGlobalBytes: 20,
				globalBytesRetries:        360,
			},
			previousLocalGlobalBytes:  10,
			previousRemoteGlobalBytes: 20,
			globalBytesRetries:        361,
			want:                      true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.wfc.needsDatabaseReset()
			if result != tt.want {
				t.Errorf("test '%s' wrong completed: %t vs %t", tt.name, result, tt.want)
			}
			if tt.wfc.previousLocalGlobalBytes != tt.previousLocalGlobalBytes {
				t.Errorf("test '%s' wrong previousLocalGlobalBytes: %d vs %d", tt.name, tt.wfc.previousLocalGlobalBytes, tt.previousLocalGlobalBytes)
			}
			if tt.wfc.previousRemoteGlobalBytes != tt.previousRemoteGlobalBytes {
				t.Errorf("test '%s' wrong previousRemoteGlobalBytes: %d vs %d", tt.name, tt.wfc.previousRemoteGlobalBytes, tt.previousRemoteGlobalBytes)
			}
			if tt.wfc.globalBytesRetries != tt.globalBytesRetries {
				t.Errorf("test '%s' wrong globalBytesRetries: %d vs %d", tt.name, tt.wfc.globalBytesRetries, tt.globalBytesRetries)
			}
		})
	}
}

func Test_isCompleted(t *testing.T) {
	tests := []struct {
		wfc                *waitForCompletion
		name               string
		needDeletesRetries int64
		want               bool
	}{
		{
			name: "need-bytes",
			wfc: &waitForCompletion{
				localCompletion: &Completion{
					NeedBytes: 10,
				},
				remoteCompletion: &Completion{
					NeedBytes: 10,
				},
			},
			needDeletesRetries: 0,
			want:               false,
		},
		{
			name: "not-matching-global-bytes",
			wfc: &waitForCompletion{
				localCompletion: &Completion{
					NeedBytes:   0,
					GlobalBytes: 10,
				},
				remoteCompletion: &Completion{
					NeedBytes:   0,
					GlobalBytes: 20,
				},
			},
			needDeletesRetries: 0,
			want:               false,
		},
		{
			name: "not-matching-need-bytes",
			wfc: &waitForCompletion{
				localCompletion: &Completion{
					NeedBytes: 0,
				},
				remoteCompletion: &Completion{
					NeedBytes: 10,
				},
			},
			needDeletesRetries: 0,
			want:               false,
		},
		{
			name: "need-deletes",
			wfc: &waitForCompletion{
				localCompletion: &Completion{
					NeedBytes:   0,
					GlobalBytes: 10,
					NeedDeletes: 10,
				},
				remoteCompletion: &Completion{
					NeedBytes:   0,
					GlobalBytes: 10,
				},
			},
			needDeletesRetries: 1,
			want:               false,
		},
		{
			name: "completed-retried-need-deletes",
			wfc: &waitForCompletion{
				localCompletion: &Completion{
					NeedBytes:   0,
					GlobalBytes: 10,
					NeedDeletes: 10,
				},
				remoteCompletion: &Completion{
					NeedBytes:   0,
					GlobalBytes: 10,
				},
				needDeletesRetries: 50,
				sy: &Syncthing{
					Folders: []*Folder{},
				},
			},
			needDeletesRetries: 51,
			want:               true,
		},
		{
			name: "not-overwritten",
			wfc: &waitForCompletion{
				localCompletion: &Completion{
					NeedBytes:   0,
					GlobalBytes: 10,
					NeedDeletes: 0,
				},
				remoteCompletion: &Completion{
					NeedBytes:   0,
					GlobalBytes: 10,
				},
				sy: &Syncthing{
					Folders: []*Folder{
						{
							Overwritten: false,
						},
					},
				},
			},
			needDeletesRetries: 0,
			want:               false,
		},
		{
			name: "completed",
			wfc: &waitForCompletion{
				localCompletion: &Completion{
					NeedBytes:   0,
					GlobalBytes: 10,
					NeedDeletes: 0,
				},
				remoteCompletion: &Completion{
					NeedBytes:   0,
					GlobalBytes: 10,
				},
				sy: &Syncthing{
					Folders: []*Folder{
						{
							Overwritten: true,
						},
					},
				},
			},
			needDeletesRetries: 0,
			want:               true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			completed := tt.wfc.isCompleted()
			if completed != tt.want {
				t.Errorf("test '%s' wrong completed: %t vs %t", tt.name, completed, tt.want)
			}
			if tt.wfc.needDeletesRetries != tt.needDeletesRetries {
				t.Errorf("test '%s' wrong needDeletesRetries: %d vs %d", tt.name, tt.wfc.needDeletesRetries, tt.needDeletesRetries)
			}
		})
	}
}
