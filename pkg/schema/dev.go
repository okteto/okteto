// Copyright 2024 The Okteto Authors
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

package schema

import "github.com/kubeark/jsonschema"

type dev struct{}

func (dev) JSONSchema() *jsonschema.Schema {
	devProps := jsonschema.NewProperties()

	// Affinity
	devProps.Set("affinity", &jsonschema.Schema{
		Type:        &jsonschema.Type{Types: []string{"object"}},
		Title:       "affinity",
		Description: "Affinity allows you to constrain which nodes your development container is eligible to be scheduled on, based on labels on the node",
	})

	// Autocreate
	devProps.Set("autocreate", &jsonschema.Schema{
		Type:        &jsonschema.Type{Types: []string{"boolean"}},
		Title:       "autocreate",
		Description: "If set to true, okteto up creates a deployment if name doesn't match any existing deployment in the current namespace",
		Default:     false,
	})

	// Command
	devProps.Set("command", &jsonschema.Schema{
		Title:       "command",
		Description: "The command of your development container. If empty, it defaults to sh. The command can also be a list",
		OneOf: []*jsonschema.Schema{
			{
				Type:    &jsonschema.Type{Types: []string{"string"}},
				Default: "sh",
			},
			{
				Type: &jsonschema.Type{Types: []string{"array"}},
				Items: &jsonschema.Schema{
					Type: &jsonschema.Type{Types: []string{"string"}},
				},
			},
		},
	})

	// Container
	devProps.Set("container", &jsonschema.Schema{
		Type:        &jsonschema.Type{Types: []string{"string"}},
		Title:       "container",
		Description: "The name of the container in your deployment you want to put on development mode. By default, it takes the first one",
	})

	// Environment
	devProps.Set("environment", &jsonschema.Schema{
		Type:        &jsonschema.Type{Types: []string{"array"}},
		Title:       "environment",
		Description: "Add environment variables to your development container",
		Items: &jsonschema.Schema{
			Type: &jsonschema.Type{Types: []string{"string"}},
		},
	})

	// EnvFiles
	devProps.Set("envFiles", &jsonschema.Schema{
		Type:        &jsonschema.Type{Types: []string{"array"}},
		Title:       "envFiles",
		Description: "Add environment variables to your development container from files",
		Items: &jsonschema.Schema{
			Type: &jsonschema.Type{Types: []string{"string"}},
		},
	})

	// ExternalVolumes
	devProps.Set("externalVolumes", &jsonschema.Schema{
		Type:        &jsonschema.Type{Types: []string{"array"}},
		Title:       "externalVolumes",
		Description: "A list of persistent volume claims that you want to mount in your development container",
		Items: &jsonschema.Schema{
			Type: &jsonschema.Type{Types: []string{"string"}},
		},
	})

	// Forward
	forwardItemProps := jsonschema.NewProperties()
	forwardItemProps.Set("localPort", &jsonschema.Schema{
		Type: &jsonschema.Type{Types: []string{"integer"}},
	})
	forwardItemProps.Set("remotePort", &jsonschema.Schema{
		Type: &jsonschema.Type{Types: []string{"integer"}},
	})
	forwardItemProps.Set("name", &jsonschema.Schema{
		Type: &jsonschema.Type{Types: []string{"string"}},
	})
	forwardItemProps.Set("labels", &jsonschema.Schema{
		Type: &jsonschema.Type{Types: []string{"object"}},
		PatternProperties: map[string]*jsonschema.Schema{
			".*": {
				Type: &jsonschema.Type{Types: []string{"string"}},
			},
		},
	})

	devProps.Set("forward", &jsonschema.Schema{
		Type:        &jsonschema.Type{Types: []string{"array"}},
		Title:       "forward",
		Description: "A list of ports to forward from your development container",
		Items: &jsonschema.Schema{
			OneOf: []*jsonschema.Schema{
				{
					Type:    &jsonschema.Type{Types: []string{"string"}},
					Pattern: "^[0-9]+:([a-zA-Z0-9-]+:)?[0-9]+$",
				},
				{
					Type:                 &jsonschema.Type{Types: []string{"object"}},
					Properties:           forwardItemProps,
					Required:             []string{"localPort", "remotePort"},
					AdditionalProperties: jsonschema.FalseSchema,
				},
			},
		},
	})

	// InitContainer
	initContainerProps := jsonschema.NewProperties()
	initContainerProps.Set("image", &jsonschema.Schema{
		Type: &jsonschema.Type{Types: []string{"string"}},
	})
	initContainerProps.Set("resources", &jsonschema.Schema{
		Type: &jsonschema.Type{Types: []string{"object"}},
	})

	devProps.Set("initContainer", &jsonschema.Schema{
		Type:                 &jsonschema.Type{Types: []string{"object"}},
		Title:                "initContainer",
		Description:          "Configuration for the okteto init container",
		Properties:           initContainerProps,
		AdditionalProperties: jsonschema.FalseSchema,
	})

	// Interface
	devProps.Set("interface", &jsonschema.Schema{
		Type:        &jsonschema.Type{Types: []string{"string"}},
		Title:       "interface",
		Description: "Address to bind port forwards and reverse tunnels to",
		Default:     "localhost",
	})

	// Image
	devProps.Set("image", &jsonschema.Schema{
		Type:        &jsonschema.Type{Types: []string{"string"}},
		Title:       "image",
		Description: "Docker image of your development container",
	})

	// ImagePullPolicy
	devProps.Set("imagePullPolicy", &jsonschema.Schema{
		Type:        &jsonschema.Type{Types: []string{"string"}},
		Title:       "imagePullPolicy",
		Description: "Image pull policy of your development container",
		Default:     "Always",
	})

	// Lifecycle
	lifecycleEventProps := jsonschema.NewProperties()
	lifecycleEventProps.Set("enabled", &jsonschema.Schema{
		Type:    &jsonschema.Type{Types: []string{"boolean"}},
		Default: false,
	})
	lifecycleEventProps.Set("command", &jsonschema.Schema{
		Type: &jsonschema.Type{Types: []string{"string"}},
	})

	lifecycleProps := jsonschema.NewProperties()
	lifecycleProps.Set("postStart", &jsonschema.Schema{
		Type:                 &jsonschema.Type{Types: []string{"object"}},
		Properties:           lifecycleEventProps,
		AdditionalProperties: jsonschema.FalseSchema,
	})
	lifecycleProps.Set("preStop", &jsonschema.Schema{
		Type:                 &jsonschema.Type{Types: []string{"object"}},
		Properties:           lifecycleEventProps,
		AdditionalProperties: jsonschema.FalseSchema,
	})

	devProps.Set("lifecycle", &jsonschema.Schema{
		OneOf: []*jsonschema.Schema{
			{
				Type:    &jsonschema.Type{Types: []string{"boolean"}},
				Default: false,
			},
			{
				Type:                 &jsonschema.Type{Types: []string{"object"}},
				Properties:           lifecycleProps,
				AdditionalProperties: jsonschema.FalseSchema,
			},
		},
	})

	// Metadata
	metadataProps := jsonschema.NewProperties()
	metadataProps.Set("annotations", &jsonschema.Schema{
		Type: &jsonschema.Type{Types: []string{"object"}},
		PatternProperties: map[string]*jsonschema.Schema{
			".*": {
				Type: &jsonschema.Type{Types: []string{"string"}},
			},
		},
	})
	metadataProps.Set("labels", &jsonschema.Schema{
		Type: &jsonschema.Type{Types: []string{"object"}},
		PatternProperties: map[string]*jsonschema.Schema{
			".*": {
				Type: &jsonschema.Type{Types: []string{"string"}},
			},
		},
	})

	devProps.Set("metadata", &jsonschema.Schema{
		Type:                 &jsonschema.Type{Types: []string{"object"}},
		Properties:           metadataProps,
		AdditionalProperties: jsonschema.FalseSchema,
	})

	// Mode
	devProps.Set("mode", &jsonschema.Schema{
		Type:        &jsonschema.Type{Types: []string{"string"}},
		Title:       "mode",
		Description: "Development mode (sync, hybrid)",
		Enum:        []any{"sync", "hybrid"},
		Default:     "sync",
	})

	// NodeSelector
	devProps.Set("nodeSelector", &jsonschema.Schema{
		Type:        &jsonschema.Type{Types: []string{"object"}},
		Title:       "nodeSelector",
		Description: "Labels that the node must have to schedule the development container",
		PatternProperties: map[string]*jsonschema.Schema{
			".*": {
				Type: &jsonschema.Type{Types: []string{"string"}},
			},
		},
	})

	// PersistentVolume
	persistentVolumeProps := jsonschema.NewProperties()
	persistentVolumeProps.Set("enabled", &jsonschema.Schema{
		Type:    &jsonschema.Type{Types: []string{"boolean"}},
		Default: true,
	})
	persistentVolumeProps.Set("accessMode", &jsonschema.Schema{
		Type:    &jsonschema.Type{Types: []string{"string"}},
		Default: "ReadWriteOnce",
	})
	persistentVolumeProps.Set("size", &jsonschema.Schema{
		Type:    &jsonschema.Type{Types: []string{"string"}},
		Default: "5Gi",
	})
	persistentVolumeProps.Set("storageClass", &jsonschema.Schema{
		Type: &jsonschema.Type{Types: []string{"string"}},
	})
	persistentVolumeProps.Set("annotations", &jsonschema.Schema{
		Type: &jsonschema.Type{Types: []string{"object"}},
		PatternProperties: map[string]*jsonschema.Schema{
			".*": {
				Type: &jsonschema.Type{Types: []string{"string"}},
			},
		},
	})
	persistentVolumeProps.Set("labels", &jsonschema.Schema{
		Type: &jsonschema.Type{Types: []string{"object"}},
		PatternProperties: map[string]*jsonschema.Schema{
			".*": {
				Type: &jsonschema.Type{Types: []string{"string"}},
			},
		},
	})

	devProps.Set("persistentVolume", &jsonschema.Schema{
		Type:                 &jsonschema.Type{Types: []string{"object"}},
		Properties:           persistentVolumeProps,
		AdditionalProperties: jsonschema.FalseSchema,
	})

	// PriorityClassName
	devProps.Set("priorityClassName", &jsonschema.Schema{
		Type:        &jsonschema.Type{Types: []string{"string"}},
		Title:       "priorityClassName",
		Description: "Priority class name for the development container",
	})

	// Probes
	probesProps := jsonschema.NewProperties()
	probesProps.Set("liveness", &jsonschema.Schema{
		Type:    &jsonschema.Type{Types: []string{"boolean"}},
		Default: false,
	})
	probesProps.Set("readiness", &jsonschema.Schema{
		Type:    &jsonschema.Type{Types: []string{"boolean"}},
		Default: false,
	})
	probesProps.Set("startup", &jsonschema.Schema{
		Type:    &jsonschema.Type{Types: []string{"boolean"}},
		Default: false,
	})

	devProps.Set("probes", &jsonschema.Schema{
		OneOf: []*jsonschema.Schema{
			{
				Type:    &jsonschema.Type{Types: []string{"boolean"}},
				Default: false,
			},
			{
				Type:                 &jsonschema.Type{Types: []string{"object"}},
				Properties:           probesProps,
				AdditionalProperties: jsonschema.FalseSchema,
			},
		},
	})

	// Resources
	// resourcesProps := jsonschema.NewProperties()
	// resourcesProps.Set("requests", &jsonschema.Schema{
	// 	Type: &jsonschema.Type{Types: []string{"object"}},
	// 	Properties: map[string]*jsonschema.Schema{
	// 		"cpu":               {Type: &jsonschema.Type{Types: []string{"string"}}},
	// 		"memory":            {Type: &jsonschema.Type{Types: []string{"string"}}},
	// 		"ephemeral-storage": {Type: &jsonschema.Type{Types: []string{"string"}}},
	// 	},
	// })
	// resourcesProps.Set("limits", &jsonschema.Schema{
	// 	Type: &jsonschema.Type{Types: []string{"object"}},
	// 	Properties: map[string]*jsonschema.Schema{
	// 		"cpu":               {Type: &jsonschema.Type{Types: []string{"string"}}},
	// 		"memory":            {Type: &jsonschema.Type{Types: []string{"string"}}},
	// 		"ephemeral-storage": {Type: &jsonschema.Type{Types: []string{"string"}}},
	// 	},
	// })

	// devProps.Set("resources", &jsonschema.Schema{
	// 	Type:                 &jsonschema.Type{Types: []string{"object"}},
	// 	Properties:           resourcesProps,
	// 	AdditionalProperties: jsonschema.FalseSchema,
	// })

	// Remote
	devProps.Set("remote", &jsonschema.Schema{
		Type:        &jsonschema.Type{Types: []string{"integer"}},
		Title:       "remote",
		Description: "Local port for SSH communication",
	})

	// Reverse
	devProps.Set("reverse", &jsonschema.Schema{
		Type:        &jsonschema.Type{Types: []string{"array"}},
		Title:       "reverse",
		Description: "Ports to reverse forward from your development container",
		Items: &jsonschema.Schema{
			Type:    &jsonschema.Type{Types: []string{"string"}},
			Pattern: "^[0-9]+:[0-9]+$",
		},
	})

	// Secrets
	devProps.Set("secrets", &jsonschema.Schema{
		Type:        &jsonschema.Type{Types: []string{"array"}},
		Title:       "secrets",
		Description: "List of secrets to be injected",
		Items: &jsonschema.Schema{
			Type: &jsonschema.Type{Types: []string{"string"}},
		},
	})

	// SecurityContext
	securityContextProps := jsonschema.NewProperties()
	securityContextProps.Set("runAsUser", &jsonschema.Schema{
		Type: &jsonschema.Type{Types: []string{"integer"}},
	})
	securityContextProps.Set("runAsGroup", &jsonschema.Schema{
		Type: &jsonschema.Type{Types: []string{"integer"}},
	})
	securityContextProps.Set("fsGroup", &jsonschema.Schema{
		Type: &jsonschema.Type{Types: []string{"integer"}},
	})
	//securityContextProps.Set("capabilities", &jsonschema.Schema{
	//	Type: &jsonschema.Type{Types: []string{"object"}},
	//	Properties: map[string]*jsonschema.Schema{
	//		"add": {
	//			Type: &jsonschema.Type{Types: []string{"array"}},
	//			Items: &jsonschema.Schema{
	//				Type: &jsonschema.Type{Types: []string{"string"}},
	//			},
	//		},
	//	},
	//})

	devProps.Set("securityContext", &jsonschema.Schema{
		Type:                 &jsonschema.Type{Types: []string{"object"}},
		Properties:           securityContextProps,
		AdditionalProperties: jsonschema.FalseSchema,
	})

	// Selector
	devProps.Set("selector", &jsonschema.Schema{
		Type:        &jsonschema.Type{Types: []string{"object"}},
		Title:       "selector",
		Description: "Labels to identify the deployment/statefulset",
		PatternProperties: map[string]*jsonschema.Schema{
			".*": {
				Type: &jsonschema.Type{Types: []string{"string"}},
			},
		},
	})

	// ServiceAccount
	devProps.Set("serviceAccount", &jsonschema.Schema{
		Type:        &jsonschema.Type{Types: []string{"string"}},
		Title:       "serviceAccount",
		Description: "Service account for the development container",
	})

	// Services
	serviceProps := jsonschema.NewProperties()
	serviceProps.Set("annotations", &jsonschema.Schema{
		Type: &jsonschema.Type{Types: []string{"object"}},
	})
	serviceProps.Set("command", &jsonschema.Schema{
		Type: &jsonschema.Type{Types: []string{"array"}},
		Items: &jsonschema.Schema{
			Type: &jsonschema.Type{Types: []string{"string"}},
		},
	})
	serviceProps.Set("container", &jsonschema.Schema{
		Type: &jsonschema.Type{Types: []string{"string"}},
	})
	serviceProps.Set("environment", &jsonschema.Schema{
		Type: &jsonschema.Type{Types: []string{"array"}},
		Items: &jsonschema.Schema{
			Type: &jsonschema.Type{Types: []string{"string"}},
		},
	})
	serviceProps.Set("image", &jsonschema.Schema{
		Type: &jsonschema.Type{Types: []string{"string"}},
	})
	serviceProps.Set("labels", &jsonschema.Schema{
		Type: &jsonschema.Type{Types: []string{"object"}},
	})
	serviceProps.Set("name", &jsonschema.Schema{
		Type: &jsonschema.Type{Types: []string{"string"}},
	})
	serviceProps.Set("resources", &jsonschema.Schema{
		Type: &jsonschema.Type{Types: []string{"object"}},
	})
	serviceProps.Set("sync", &jsonschema.Schema{
		Type: &jsonschema.Type{Types: []string{"array"}},
		Items: &jsonschema.Schema{
			Type: &jsonschema.Type{Types: []string{"string"}},
		},
	})
	serviceProps.Set("workdir", &jsonschema.Schema{
		Type: &jsonschema.Type{Types: []string{"string"}},
	})
	serviceProps.Set("replicas", &jsonschema.Schema{
		Type: &jsonschema.Type{Types: []string{"integer"}},
	})

	devProps.Set("services", &jsonschema.Schema{
		Type:        &jsonschema.Type{Types: []string{"array"}},
		Title:       "services",
		Description: "Additional services to run in development mode",
		Items: &jsonschema.Schema{
			Type:                 &jsonschema.Type{Types: []string{"object"}},
			Properties:           serviceProps,
			AdditionalProperties: jsonschema.FalseSchema,
		},
	})

	// Sync
	syncProps := jsonschema.NewProperties()
	syncProps.Set("folders", &jsonschema.Schema{
		Type: &jsonschema.Type{Types: []string{"array"}},
		Items: &jsonschema.Schema{
			Type: &jsonschema.Type{Types: []string{"string"}},
		},
	})
	syncProps.Set("verbose", &jsonschema.Schema{
		Type:    &jsonschema.Type{Types: []string{"boolean"}},
		Default: true,
	})
	syncProps.Set("compression", &jsonschema.Schema{
		Type:    &jsonschema.Type{Types: []string{"boolean"}},
		Default: false,
	})
	syncProps.Set("rescanInterval", &jsonschema.Schema{
		Type:    &jsonschema.Type{Types: []string{"integer"}},
		Default: 300,
	})

	devProps.Set("sync", &jsonschema.Schema{
		OneOf: []*jsonschema.Schema{
			{
				Type: &jsonschema.Type{Types: []string{"array"}},
				Items: &jsonschema.Schema{
					Type:    &jsonschema.Type{Types: []string{"string"}},
					Pattern: "^.*:.*$",
				},
			},
			{
				Type:                 &jsonschema.Type{Types: []string{"object"}},
				Properties:           syncProps,
				Required:             []string{"folders"},
				AdditionalProperties: jsonschema.FalseSchema,
			},
		},
	})

	// Timeout
	timeoutProps := jsonschema.NewProperties()
	timeoutProps.Set("default", &jsonschema.Schema{
		Type:    &jsonschema.Type{Types: []string{"string"}},
		Pattern: "^[0-9]+(h|m|s)$",
	})
	timeoutProps.Set("resources", &jsonschema.Schema{
		Type:    &jsonschema.Type{Types: []string{"string"}},
		Pattern: "^[0-9]+(h|m|s)$",
	})

	devProps.Set("timeout", &jsonschema.Schema{
		OneOf: []*jsonschema.Schema{
			{
				Type:    &jsonschema.Type{Types: []string{"string"}},
				Pattern: "^[0-9]+(h|m|s)$",
			},
			{
				Type:                 &jsonschema.Type{Types: []string{"object"}},
				Properties:           timeoutProps,
				AdditionalProperties: jsonschema.FalseSchema,
			},
		},
	})

	// Tolerations
	devProps.Set("tolerations", &jsonschema.Schema{
		Type:        &jsonschema.Type{Types: []string{"array"}},
		Title:       "tolerations",
		Description: "Pod tolerations",
		Items: &jsonschema.Schema{
			Type: &jsonschema.Type{Types: []string{"object"}},
		},
	})

	// Volumes
	devProps.Set("volumes", &jsonschema.Schema{
		Type:        &jsonschema.Type{Types: []string{"array"}},
		Title:       "volumes",
		Description: "List of paths to persist",
		Items: &jsonschema.Schema{
			Type: &jsonschema.Type{Types: []string{"string"}},
		},
	})

	// Workdir
	devProps.Set("workdir", &jsonschema.Schema{
		Type:        &jsonschema.Type{Types: []string{"string"}},
		Title:       "workdir",
		Description: "Working directory of your development container",
	})

	return &jsonschema.Schema{
		Type:                 &jsonschema.Type{Types: []string{"object"}},
		AdditionalProperties: jsonschema.FalseSchema,
		PatternProperties: map[string]*jsonschema.Schema{
			".*": {
				Type:                 &jsonschema.Type{Types: []string{"object"}},
				Properties:           devProps,
				AdditionalProperties: jsonschema.FalseSchema,
			},
		},
	}
}
