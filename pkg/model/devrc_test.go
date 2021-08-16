package model

import (
	"reflect"
	"testing"

	"github.com/okteto/okteto/pkg/log"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func TestReadDevRC(t *testing.T) {
	var tests = []struct {
		name     string
		logLevel string
		manifest []byte
		expected *DevRC
	}{
		{
			name:     "none",
			logLevel: "info",
			manifest: []byte(``),
			expected: &DevRC{},
		},
		{
			name:     "read-info",
			logLevel: "info",
			manifest: []byte(`labels:
  app: test
annotations:
  db: mongodb
context: "test"
namespace: test
environment:
  OKTETO_HOME: /home/.okteto
sync:
  - /home/.vimrc:/home/.vimrc
resources:
  limits:
    memory: 500M
`),
			expected: &DevRC{
				Labels:      Labels{"app": "test"},
				Annotations: Annotations{"db": "mongodb"},
				Context:     "test",
				Namespace:   "test",
				Environment: Environment{
					EnvVar{
						Name:  "OKTETO_HOME",
						Value: "/home/.okteto",
					},
				},
				Sync: Sync{
					Verbose:        false,
					RescanInterval: 300,
					Folders: []SyncFolder{
						{
							LocalPath:  "/home/.vimrc",
							RemotePath: "/home/.vimrc",
						},
					},
				},
				Resources: ResourceRequirements{
					Limits: ResourceList{
						v1.ResourceMemory: resource.MustParse("500M"),
					},
				},
			},
		},
		{
			name:     "read-debug",
			logLevel: "debug",
			manifest: []byte(`labels:
  app: test
annotations:
  db: mongodb
context: "test"
namespace: test
environment:
  OKTETO_HOME: /home/.okteto
sync:
  - /home/.vimrc:/home/.vimrc
resources:
  limits:
    memory: 500M
`),
			expected: &DevRC{
				Labels:      Labels{"app": "test"},
				Annotations: Annotations{"db": "mongodb"},
				Context:     "test",
				Namespace:   "test",
				Environment: Environment{
					EnvVar{
						Name:  "OKTETO_HOME",
						Value: "/home/.okteto",
					},
				},
				Sync: Sync{
					Verbose:        true,
					RescanInterval: 300,
					Folders: []SyncFolder{
						{
							LocalPath:  "/home/.vimrc",
							RemotePath: "/home/.vimrc",
						},
					},
				},
				Resources: ResourceRequirements{
					Limits: ResourceList{
						v1.ResourceMemory: resource.MustParse("500M"),
					},
				},
			},
		},
	}

	defer log.SetLevel("info")
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			log.SetLevel(tt.logLevel)
			dev, err := ReadRC(tt.manifest)
			if err != nil {
				t.Fatalf("Parse readrc has failed: %s", err.Error())
			}

			if !reflect.DeepEqual(dev, tt.expected) {
				t.Fatalf("Expected %v but got %v", tt.expected, dev)
			}
		})
	}
}

func TestDevRCLabels(t *testing.T) {
	var tests = []struct {
		name     string
		dev      *Dev
		devRC    *DevRC
		expected Labels
	}{
		{
			name:     "not overwrite",
			dev:      &Dev{Labels: Labels{"app": "test"}},
			devRC:    &DevRC{},
			expected: Labels{"app": "test"},
		},
		{
			name:     "merge",
			dev:      &Dev{Labels: Labels{"app": "test"}},
			devRC:    &DevRC{Labels: Labels{"test": "app"}},
			expected: Labels{"app": "test", "test": "app"},
		},
		{
			name:     "overwrite",
			dev:      &Dev{Labels: Labels{"app": "test"}},
			devRC:    &DevRC{Labels: Labels{"app": "dev"}},
			expected: Labels{"app": "dev"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			MergeDevWithDevRc(tt.dev, tt.devRC)
			for key, value := range tt.dev.Labels {
				if val, ok := tt.expected[key]; ok {
					if val != value {
						t.Fatal("Not merged correctly")
					}
				} else {
					t.Fatal("Not merged correctly")
				}
			}

			for key, value := range tt.expected {
				if val, ok := tt.dev.Labels[key]; ok {
					if val != value {
						t.Fatal("Not merged correctly")
					}
				} else {
					t.Fatal("Not merged correctly")
				}
			}
		})
	}
}

func TestDevRCCommand(t *testing.T) {
	var tests = []struct {
		name     string
		dev      *Dev
		devRC    *DevRC
		expected Command
	}{
		{
			name:     "not overwrite",
			dev:      &Dev{Command: Command{Values: []string{"/bin/sh"}}},
			devRC:    &DevRC{},
			expected: Command{Values: []string{"/bin/sh"}},
		},
		{
			name:     "overwrite",
			dev:      &Dev{Command: Command{Values: []string{"/bin/sh"}}},
			devRC:    &DevRC{Command: Command{Values: []string{"/bin/bash"}}},
			expected: Command{Values: []string{"/bin/bash"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			MergeDevWithDevRc(tt.dev, tt.devRC)
			if !reflect.DeepEqual(tt.dev.Command, tt.expected) {
				t.Fatalf("Expected %v but got %v", tt.expected, tt.dev.Command)
			}
		})
	}
}

func TestDevRCAnnotations(t *testing.T) {
	var tests = []struct {
		name     string
		dev      *Dev
		devRC    *DevRC
		expected Annotations
	}{
		{
			name:     "not overwrite",
			dev:      &Dev{Annotations: Annotations{"app": "test"}},
			devRC:    &DevRC{},
			expected: Annotations{"app": "test"},
		},
		{
			name:     "merge",
			dev:      &Dev{Annotations: Annotations{"app": "test"}},
			devRC:    &DevRC{Annotations: Annotations{"test": "app"}},
			expected: Annotations{"app": "test", "test": "app"},
		},
		{
			name:     "overwrite",
			dev:      &Dev{Annotations: Annotations{"app": "test"}},
			devRC:    &DevRC{Annotations: Annotations{"app": "dev"}},
			expected: Annotations{"app": "dev"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			MergeDevWithDevRc(tt.dev, tt.devRC)
			for key, value := range tt.dev.Annotations {
				if val, ok := tt.expected[key]; ok {
					if val != value {
						t.Fatal("Not merged correctly")
					}
				} else {
					t.Fatal("Not merged correctly")
				}
			}

			for key, value := range tt.expected {
				if val, ok := tt.dev.Annotations[key]; ok {
					if val != value {
						t.Fatal("Not merged correctly")
					}
				} else {
					t.Fatal("Not merged correctly")
				}
			}
		})
	}
}

func TestDevRCContext(t *testing.T) {
	var tests = []struct {
		name     string
		dev      *Dev
		devRC    *DevRC
		expected string
	}{
		{
			name:     "not overwrite",
			dev:      &Dev{Context: ""},
			devRC:    &DevRC{Context: ""},
			expected: "",
		},
		{
			name:     "not overwrite2",
			dev:      &Dev{Context: "app"},
			devRC:    &DevRC{Context: "test"},
			expected: "test",
		},
		{
			name:     "not overwrite3",
			dev:      &Dev{Context: "app"},
			devRC:    &DevRC{Context: ""},
			expected: "app",
		},
		{
			name:     "overwrite",
			dev:      &Dev{Context: ""},
			devRC:    &DevRC{Context: "app"},
			expected: "app",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			MergeDevWithDevRc(tt.dev, tt.devRC)
			if tt.dev.Context != tt.expected {
				t.Fatal("Wrong merging")
			}
		})
	}
}

func TestDevRCNamespace(t *testing.T) {
	var tests = []struct {
		name     string
		dev      *Dev
		devRC    *DevRC
		expected string
	}{
		{
			name:     "not overwrite",
			dev:      &Dev{Namespace: ""},
			devRC:    &DevRC{Namespace: ""},
			expected: "",
		},
		{
			name:     "not overwrite2",
			dev:      &Dev{Namespace: "app"},
			devRC:    &DevRC{Namespace: "test"},
			expected: "test",
		},
		{
			name:     "not overwrite3",
			dev:      &Dev{Namespace: "app"},
			devRC:    &DevRC{Namespace: ""},
			expected: "app",
		},
		{
			name:     "overwrite",
			dev:      &Dev{Namespace: ""},
			devRC:    &DevRC{Namespace: "app"},
			expected: "app",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			MergeDevWithDevRc(tt.dev, tt.devRC)
			if tt.dev.Namespace != tt.expected {
				t.Fatal("Wrong merging")
			}
		})
	}
}

func TestDevRCSync(t *testing.T) {
	var tests = []struct {
		name     string
		dev      *Dev
		devRC    *DevRC
		expected Sync
	}{
		{
			name: "merge sync folder",
			dev: &Dev{Sync: Sync{
				Folders: []SyncFolder{
					{
						LocalPath:  "home",
						RemotePath: "var",
					},
				},
			},
			},
			devRC: &DevRC{Sync: Sync{
				Folders: []SyncFolder{
					{
						LocalPath:  "var",
						RemotePath: "home",
					},
				},
			}},
			expected: Sync{
				Folders: []SyncFolder{
					{
						LocalPath:  "home",
						RemotePath: "var",
					},
					{
						LocalPath:  "var",
						RemotePath: "home",
					},
				},
			},
		},
		{
			name: "not merge sync folder because same local",
			dev: &Dev{Sync: Sync{
				Folders: []SyncFolder{
					{
						LocalPath:  "home",
						RemotePath: "var",
					},
				},
			},
			},
			devRC: &DevRC{Sync: Sync{
				Folders: []SyncFolder{
					{
						LocalPath:  "home",
						RemotePath: "app",
					},
				},
			}},
			expected: Sync{
				Folders: []SyncFolder{
					{
						LocalPath:  "home",
						RemotePath: "var",
					},
				},
			},
		},
		{
			name: "not merge sync folder because same remote",
			dev: &Dev{Sync: Sync{
				Folders: []SyncFolder{
					{
						LocalPath:  "home",
						RemotePath: "var",
					},
				},
			},
			},
			devRC: &DevRC{Sync: Sync{
				Folders: []SyncFolder{
					{
						LocalPath:  "app",
						RemotePath: "var",
					},
				},
			}},
			expected: Sync{
				Folders: []SyncFolder{
					{
						LocalPath:  "home",
						RemotePath: "var",
					},
				},
			},
		},
		{
			name: "compression",
			dev: &Dev{Sync: Sync{
				Compression: false,
			},
			},
			devRC: &DevRC{Sync: Sync{
				Compression: true,
			}},
			expected: Sync{
				Compression: true,
			},
		},
		{
			name: "compression",
			dev: &Dev{Sync: Sync{
				Verbose: true,
			}},
			devRC: &DevRC{Sync: Sync{
				Verbose: false,
			}},

			expected: Sync{
				Compression: true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			MergeDevWithDevRc(tt.dev, tt.devRC)
			if reflect.DeepEqual(tt.dev, tt.expected) {
				t.Fatal("Wrong merging")
			}
		})
	}
}
