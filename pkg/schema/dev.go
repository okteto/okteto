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

	devProps.Set("affinity", &jsonschema.Schema{
		Type:        &jsonschema.Type{Types: []string{"object"}},
		Title:       "affinity",
		Description: withManifestRefDocLink("Affinity allows you to constrain which nodes your development container is eligible to be scheduled on, based on labels on the node.", "affinity-affinity-optional"),
	})

	devProps.Set("autocreate", &jsonschema.Schema{
		Type:        &jsonschema.Type{Types: []string{"boolean"}},
		Title:       "autocreate",
		Description: withManifestRefDocLink("If set to true, okteto up creates a deployment if name doesn't match any existing deployment in the current namespace.", "autocreate-bool-optional"),
		Default:     false,
	})

	devProps.Set("command", &jsonschema.Schema{
		Title:       "command",
		Description: withManifestRefDocLink("The command of your development container. If empty, it defaults to sh. The command can also be a list.", "command-string-optional"),
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

	devProps.Set("container", &jsonschema.Schema{
		Type:        &jsonschema.Type{Types: []string{"string"}},
		Title:       "container",
		Description: withManifestRefDocLink("The name of the container in your deployment you want to put on development mode. By default, it takes the first one.", "container-string-optional"),
	})

	devProps.Set("environment", &jsonschema.Schema{
		Title:       "environment",
		Description: withManifestRefDocLink("Add environment variables to your development container. If a variable already exists on your deployment, it will be overridden with the value specified on the manifest. Environment variables with only a key, or with a value with a $ sign resolve to their values on the machine Okteto is running on", "environment-string-optional"),
		OneOf: []*jsonschema.Schema{
			{
				Type: &jsonschema.Type{Types: []string{"object"}},
				PatternProperties: map[string]*jsonschema.Schema{
					".*": {
						Type: &jsonschema.Type{Types: []string{"string", "boolean", "number"}},
					},
				},
			},
			{
				Type: &jsonschema.Type{Types: []string{"array"}},
				Items: &jsonschema.Schema{
					Type: &jsonschema.Type{Types: []string{"string"}},
				},
			},
		},
	})

	devProps.Set("envFiles", &jsonschema.Schema{
		Type:        &jsonschema.Type{Types: []string{"array"}},
		Title:       "envFiles",
		Description: withManifestRefDocLink("Add environment variables to your development container from files", "envfiles"),
		Items: &jsonschema.Schema{
			Type: &jsonschema.Type{Types: []string{"string"}},
		},
	})

	devProps.Set("externalVolumes", &jsonschema.Schema{
		Type:        &jsonschema.Type{Types: []string{"array"}},
		Title:       "externalVolumes",
		Description: withManifestRefDocLink("A list of persistent volume claims that you want to mount in your development container", "externalvolumes-string-optional"),
		Items: &jsonschema.Schema{
			Type: &jsonschema.Type{Types: []string{"string"}},
		},
	})

	forwardItemProps := jsonschema.NewProperties()
	forwardItemProps.Set("localPort", &jsonschema.Schema{
		Type:  &jsonschema.Type{Types: []string{"integer"}},
		Title: "localPort",
	})
	forwardItemProps.Set("remotePort", &jsonschema.Schema{
		Type:  &jsonschema.Type{Types: []string{"integer"}},
		Title: "remotePort",
	})
	forwardItemProps.Set("name", &jsonschema.Schema{
		Type:  &jsonschema.Type{Types: []string{"string"}},
		Title: "name",
	})
	forwardItemProps.Set("labels", &jsonschema.Schema{
		Type:  &jsonschema.Type{Types: []string{"object"}},
		Title: "labels",
		PatternProperties: map[string]*jsonschema.Schema{
			".*": {
				Type: &jsonschema.Type{Types: []string{"string"}},
			},
		},
	})

	devProps.Set("forward", &jsonschema.Schema{
		Type:        &jsonschema.Type{Types: []string{"array"}},
		Title:       "forward",
		Description: withManifestRefDocLink("A list of ports to forward from your development container", "forward-string-optional"),
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

	initContainerProps := jsonschema.NewProperties()
	initContainerProps.Set("image", &jsonschema.Schema{
		Type:  &jsonschema.Type{Types: []string{"string"}},
		Title: "image",
	})
	initContainerProps.Set("resources", &jsonschema.Schema{
		Type:  &jsonschema.Type{Types: []string{"object"}},
		Title: "resources",
	})

	devProps.Set("initContainer", &jsonschema.Schema{
		Type:                 &jsonschema.Type{Types: []string{"object"}},
		Title:                "initContainer",
		Description:          withManifestRefDocLink("Allows you to override the okteto init container configuration of your development container.", "initcontainer-object-optional"),
		Properties:           initContainerProps,
		AdditionalProperties: jsonschema.FalseSchema,
	})

	devProps.Set("interface", &jsonschema.Schema{
		Type:        &jsonschema.Type{Types: []string{"string"}},
		Title:       "interface",
		Description: withManifestRefDocLink("Port forwards and reverse tunnels will be bound to this address.", "interface-string-optional"),
		Default:     "localhost",
	})

	devProps.Set("image", &jsonschema.Schema{
		Type:        &jsonschema.Type{Types: []string{"string"}},
		Title:       "image",
		Description: withManifestRefDocLink("Sets the docker image of your development container. Defaults to the image specified in your deployment.", "image-string-optional"),
	})

	devProps.Set("imagePullPolicy", &jsonschema.Schema{
		Type:        &jsonschema.Type{Types: []string{"string"}},
		Title:       "imagePullPolicy",
		Description: withManifestRefDocLink("Image pull policy of your development container", "imagepullpolicy-string-optional"),
		Default:     "Always",
	})

	lifecycleEventProps := jsonschema.NewProperties()
	lifecycleEventProps.Set("enabled", &jsonschema.Schema{
		Type:    &jsonschema.Type{Types: []string{"boolean"}},
		Title:   "enabled",
		Default: false,
	})
	lifecycleEventProps.Set("command", &jsonschema.Schema{
		Type:  &jsonschema.Type{Types: []string{"string"}},
		Title: "command",
	})

	lifecycleProps := jsonschema.NewProperties()
	lifecycleProps.Set("postStart", &jsonschema.Schema{
		Type:                 &jsonschema.Type{Types: []string{"object"}},
		Title:                "postStart",
		Properties:           lifecycleEventProps,
		AdditionalProperties: jsonschema.FalseSchema,
	})
	lifecycleProps.Set("preStop", &jsonschema.Schema{
		Type:                 &jsonschema.Type{Types: []string{"object"}},
		Title:                "preStop",
		Properties:           lifecycleEventProps,
		AdditionalProperties: jsonschema.FalseSchema,
	})

	devProps.Set("lifecycle", &jsonschema.Schema{
		Title:       "lifecycle",
		Description: withManifestRefDocLink("Configures lifecycle hooks for your development container. Lifecycle hooks allow you to execute commands when your container starts or stops, enabling you to automate setup or cleanup tasks.", "lifecycle-boolean-optional"),
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

	metadataProps := jsonschema.NewProperties()
	metadataProps.Set("annotations", &jsonschema.Schema{
		Type:  &jsonschema.Type{Types: []string{"object"}},
		Title: "annotations",
		PatternProperties: map[string]*jsonschema.Schema{
			".*": {
				Type: &jsonschema.Type{Types: []string{"string"}},
			},
		},
	})
	metadataProps.Set("labels", &jsonschema.Schema{
		Type:  &jsonschema.Type{Types: []string{"object"}},
		Title: "labels",
		PatternProperties: map[string]*jsonschema.Schema{
			".*": {
				Type: &jsonschema.Type{Types: []string{"string"}},
			},
		},
	})

	devProps.Set("metadata", &jsonschema.Schema{
		Type:                 &jsonschema.Type{Types: []string{"object"}},
		Title:                "metadata",
		Description:          withManifestRefDocLink("The metadata field allows to inject labels and annotations into your development container.", "metadata-object-optional"),
		Properties:           metadataProps,
		AdditionalProperties: jsonschema.FalseSchema,
	})

	devProps.Set("mode", &jsonschema.Schema{
		Type:        &jsonschema.Type{Types: []string{"string"}},
		Title:       "mode",
		Description: withManifestRefDocLink("Development mode (sync, hybrid)", "mode-string-optional"),
		Enum:        []any{"sync", "hybrid"},
		Default:     "sync",
	})

	devProps.Set("nodeSelector", &jsonschema.Schema{
		Type:        &jsonschema.Type{Types: []string{"object"}},
		Title:       "nodeSelector",
		Description: withManifestRefDocLink("Labels that the node must have to schedule the development container", "nodeselector-mapstringstring-optional"),
		PatternProperties: map[string]*jsonschema.Schema{
			".*": {
				Type: &jsonschema.Type{Types: []string{"string"}},
			},
		},
	})

	persistentVolumeProps := jsonschema.NewProperties()
	// TODO: enforce persistentVolume.enabled must be true if you use services and volumes
	persistentVolumeProps.Set("enabled", &jsonschema.Schema{
		Type:        &jsonschema.Type{Types: []string{"boolean"}},
		Title:       "enabled",
		Default:     true,
		Description: "Enable/disable the use of persistent volumes. Must be true if using services, volumes, or to share command history.",
	})
	persistentVolumeProps.Set("accessMode", &jsonschema.Schema{
		Type:        &jsonschema.Type{Types: []string{"string"}},
		Title:       "accessMode",
		Default:     "ReadWriteOnce",
		Description: "The Okteto persistent volume access mode",
	})
	persistentVolumeProps.Set("size", &jsonschema.Schema{
		Type:        &jsonschema.Type{Types: []string{"string"}},
		Title:       "size",
		Default:     "5Gi",
		Description: "The size of the Okteto persistent volume",
	})
	persistentVolumeProps.Set("storageClass", &jsonschema.Schema{
		Type:        &jsonschema.Type{Types: []string{"string"}},
		Title:       "storageClass",
		Description: "The storage class of the Okteto persistent volume. Defaults to cluster's default storage class",
	})
	persistentVolumeProps.Set("volumeMode", &jsonschema.Schema{
		Type:        &jsonschema.Type{Types: []string{"string"}},
		Title:       "volumeMode",
		Default:     "Filesystem",
		Description: "The Okteto persistent volume mode",
	})
	persistentVolumeProps.Set("annotations", &jsonschema.Schema{
		Type:        &jsonschema.Type{Types: []string{"object"}},
		Title:       "annotations",
		Description: "Add annotations to the Okteto persistent volume",
		PatternProperties: map[string]*jsonschema.Schema{
			".*": {
				Type: &jsonschema.Type{Types: []string{"string"}},
			},
		},
	})
	persistentVolumeProps.Set("labels", &jsonschema.Schema{
		Type:        &jsonschema.Type{Types: []string{"object"}},
		Title:       "labels",
		Description: "Add labels to the Okteto persistent volume",
		PatternProperties: map[string]*jsonschema.Schema{
			".*": {
				Type: &jsonschema.Type{Types: []string{"string"}},
			},
		},
	})
	devProps.Set("persistentVolume", &jsonschema.Schema{
		Type:                 &jsonschema.Type{Types: []string{"object"}},
		Title:                "persistentVolume",
		Description:          withManifestRefDocLink("Allows you to configure a persistent volume for your development container.", "persistentvolume-object-optional"),
		Properties:           persistentVolumeProps,
		AdditionalProperties: jsonschema.FalseSchema,
	})

	devProps.Set("priorityClassName", &jsonschema.Schema{
		Type:        &jsonschema.Type{Types: []string{"string"}},
		Title:       "priorityClassName",
		Description: withManifestRefDocLink("Priority class name for the development container", "priorityclassname-string-optional"),
	})

	probesProps := jsonschema.NewProperties()
	probesProps.Set("liveness", &jsonschema.Schema{
		Type:    &jsonschema.Type{Types: []string{"boolean"}},
		Title:   "liveness",
		Default: false,
	})
	probesProps.Set("readiness", &jsonschema.Schema{
		Type:    &jsonschema.Type{Types: []string{"boolean"}},
		Title:   "readiness",
		Default: false,
	})
	probesProps.Set("startup", &jsonschema.Schema{
		Type:    &jsonschema.Type{Types: []string{"boolean"}},
		Title:   "startup",
		Default: false,
	})

	devProps.Set("probes", &jsonschema.Schema{
		Title:       "probes",
		Description: withManifestRefDocLink("Used to enable or disable the Kubernetes probes of your development container. If set to 'true' ", "probes-boolean-optional"),
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

	devProps.Set("remote", &jsonschema.Schema{
		Type:        &jsonschema.Type{Types: []string{"integer"}},
		Title:       "remote",
		Description: "Local port for SSH communication",
	})

	devProps.Set("reverse", &jsonschema.Schema{
		Type:        &jsonschema.Type{Types: []string{"array"}},
		Title:       "reverse",
		Description: "Ports to reverse forward from your development container",
		Items: &jsonschema.Schema{
			Type:    &jsonschema.Type{Types: []string{"string"}},
			Pattern: "^[0-9]+:[0-9]+$",
		},
	})

	devProps.Set("secrets", &jsonschema.Schema{
		Type:        &jsonschema.Type{Types: []string{"array"}},
		Title:       "secrets",
		Description: "List of secrets to be injected",
		Items: &jsonschema.Schema{
			Type: &jsonschema.Type{Types: []string{"string"}},
		},
	})

	securityContextProps := jsonschema.NewProperties()
	securityContextProps.Set("runAsUser", &jsonschema.Schema{
		Type:  &jsonschema.Type{Types: []string{"integer"}},
		Title: "runAsUser",
	})
	securityContextProps.Set("runAsGroup", &jsonschema.Schema{
		Type:  &jsonschema.Type{Types: []string{"integer"}},
		Title: "runAsGroup",
	})
	securityContextProps.Set("fsGroup", &jsonschema.Schema{
		Type:  &jsonschema.Type{Types: []string{"integer"}},
		Title: "fsGroup",
	})
	securityContextProps.Set("runAsNonRoot", &jsonschema.Schema{
		Type:  &jsonschema.Type{Types: []string{"boolean"}},
		Title: "runAsNonRoot",
	})
	securityContextProps.Set("allowPrivilegeEscalation", &jsonschema.Schema{
		Type:  &jsonschema.Type{Types: []string{"boolean"}},
		Title: "allowPrivilegeEscalation",
	})
	securityContextProps.Set("readOnlyRootFilesystem", &jsonschema.Schema{
		Type:  &jsonschema.Type{Types: []string{"boolean"}},
		Title: "readOnlyRootFilesystem",
	})

	capabilitiesProps := jsonschema.NewProperties()
	capabilitiesProps.Set("add", &jsonschema.Schema{
		Type:  &jsonschema.Type{Types: []string{"array"}},
		Title: "add",
		Items: &jsonschema.Schema{
			Type: &jsonschema.Type{Types: []string{"string"}},
		},
	})

	securityContextProps.Set("capabilities", &jsonschema.Schema{
		Type:                 &jsonschema.Type{Types: []string{"object"}},
		Title:                "capabilities",
		Properties:           capabilitiesProps,
		AdditionalProperties: jsonschema.FalseSchema,
	})

	devProps.Set("securityContext", &jsonschema.Schema{
		Type:                 &jsonschema.Type{Types: []string{"object"}},
		Title:                "securityContext",
		Properties:           securityContextProps,
		AdditionalProperties: jsonschema.FalseSchema,
	})

	devProps.Set("selector", &jsonschema.Schema{
		Type:        &jsonschema.Type{Types: []string{"object"}},
		Title:       "selector",
		Description: withManifestRefDocLink("The labels of the Kubernetes deployment/statefulset you want to put on development mode. They must identify a single Kubernetes deployment/statefulset.", "selector-mapstringstring-optional"),
		PatternProperties: map[string]*jsonschema.Schema{
			".*": {
				Type: &jsonschema.Type{Types: []string{"string"}},
			},
		},
	})

	devProps.Set("serviceAccount", &jsonschema.Schema{
		Type:        &jsonschema.Type{Types: []string{"string"}},
		Title:       "serviceAccount",
		Description: withManifestRefDocLink("Service account for the development container", "serviceaccount-string-optional"),
	})

	serviceProps := jsonschema.NewProperties()
	serviceProps.Set("annotations", &jsonschema.Schema{
		Type:  &jsonschema.Type{Types: []string{"object"}},
		Title: "annotations",
	})
	serviceProps.Set("command", &jsonschema.Schema{
		Type:  &jsonschema.Type{Types: []string{"array"}},
		Title: "command",
		Items: &jsonschema.Schema{
			Type: &jsonschema.Type{Types: []string{"string"}},
		},
	})
	serviceProps.Set("container", &jsonschema.Schema{
		Type:  &jsonschema.Type{Types: []string{"string"}},
		Title: "container",
	})
	serviceProps.Set("environment", &jsonschema.Schema{
		Type:  &jsonschema.Type{Types: []string{"object"}},
		Title: "environment",
		PatternProperties: map[string]*jsonschema.Schema{
			".*": {
				Type: &jsonschema.Type{Types: []string{"string"}},
			},
		},
	})
	serviceProps.Set("image", &jsonschema.Schema{
		Type:  &jsonschema.Type{Types: []string{"string"}},
		Title: "image",
	})
	serviceProps.Set("labels", &jsonschema.Schema{
		Type:  &jsonschema.Type{Types: []string{"object"}},
		Title: "labels",
	})
	serviceProps.Set("name", &jsonschema.Schema{
		Type:  &jsonschema.Type{Types: []string{"string"}},
		Title: "name",
	})
	serviceProps.Set("resources", &jsonschema.Schema{
		Type:  &jsonschema.Type{Types: []string{"object"}},
		Title: "resources",
	})
	serviceProps.Set("sync", &jsonschema.Schema{
		Type:  &jsonschema.Type{Types: []string{"array"}},
		Title: "sync",
		Items: &jsonschema.Schema{
			Type: &jsonschema.Type{Types: []string{"string"}},
		},
	})
	serviceProps.Set("workdir", &jsonschema.Schema{
		Type:  &jsonschema.Type{Types: []string{"string"}},
		Title: "workdir",
	})
	serviceProps.Set("replicas", &jsonschema.Schema{
		Type:  &jsonschema.Type{Types: []string{"integer"}},
		Title: "replicas",
	})

	devProps.Set("services", &jsonschema.Schema{
		Type:        &jsonschema.Type{Types: []string{"array"}},
		Title:       "services",
		Description: withManifestRefDocLink("A list of services that you want to put on developer mode along your development container. The services work just like the development container, with one exception: they won't be able to start an interactive session.", "services-object-optional"),
		Items: &jsonschema.Schema{
			Type:                 &jsonschema.Type{Types: []string{"object"}},
			Properties:           serviceProps,
			AdditionalProperties: jsonschema.FalseSchema,
		},
	})

	syncProps := jsonschema.NewProperties()
	syncProps.Set("folders", &jsonschema.Schema{
		Type:  &jsonschema.Type{Types: []string{"array"}},
		Title: "folders",
		Items: &jsonschema.Schema{
			Type: &jsonschema.Type{Types: []string{"string"}},
		},
	})
	syncProps.Set("verbose", &jsonschema.Schema{
		Type:    &jsonschema.Type{Types: []string{"boolean"}},
		Title:   "verbose",
		Default: true,
	})
	syncProps.Set("compression", &jsonschema.Schema{
		Type:    &jsonschema.Type{Types: []string{"boolean"}},
		Title:   "compression",
		Default: false,
	})
	syncProps.Set("rescanInterval", &jsonschema.Schema{
		Type:    &jsonschema.Type{Types: []string{"integer"}},
		Title:   "rescanInterval",
		Default: 300,
	})

	devProps.Set("sync", &jsonschema.Schema{
		Title:       "sync",
		Description: withManifestRefDocLink("Specifies local folders that must be synchronized to the development container.", "sync-string-required"),
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

	timeoutProps := jsonschema.NewProperties()
	timeoutProps.Set("default", &jsonschema.Schema{
		Type:    &jsonschema.Type{Types: []string{"string"}},
		Title:   "default",
		Pattern: "^[0-9]+(h|m|s)$",
	})
	timeoutProps.Set("resources", &jsonschema.Schema{
		Type:    &jsonschema.Type{Types: []string{"string"}},
		Title:   "resources",
		Pattern: "^[0-9]+(h|m|s)$",
	})

	devProps.Set("timeout", &jsonschema.Schema{
		Title:       "timeout",
		Description: withManifestRefDocLink("Maximum time to be waiting for creating a development container until an error is returned.", "timeout-time-optional"),
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

	devProps.Set("tolerations", &jsonschema.Schema{
		Type:        &jsonschema.Type{Types: []string{"array"}},
		Title:       "tolerations",
		Description: withManifestRefDocLink("A list of tolerations that will be injected into your development container.", "tolerations-object-optional"),
		Items: &jsonschema.Schema{
			Type: &jsonschema.Type{Types: []string{"object"}},
		},
	})

	devProps.Set("volumes", &jsonschema.Schema{
		Type:        &jsonschema.Type{Types: []string{"array"}},
		Title:       "volumes",
		Description: withManifestRefDocLink("A list of paths in your development container that you want to associate to persistent volumes. This is useful to persist information between okteto up executions, like downloaded libraries or cache information. ", "volumes-string-optional"),
		Items: &jsonschema.Schema{
			Type: &jsonschema.Type{Types: []string{"string"}},
		},
	})

	devProps.Set("workdir", &jsonschema.Schema{
		Type:        &jsonschema.Type{Types: []string{"string"}},
		Title:       "workdir",
		Description: withManifestRefDocLink("Sets the working directory of your development container.", "workdir-string-optional"),
	})

	resourcesProps := jsonschema.NewProperties()

	resourceValuesProps := jsonschema.NewProperties()
	resourceValuesProps.Set("cpu", &jsonschema.Schema{
		Type:  &jsonschema.Type{Types: []string{"string"}},
		Title: "cpu",
	})
	resourceValuesProps.Set("memory", &jsonschema.Schema{
		Type:  &jsonschema.Type{Types: []string{"string"}},
		Title: "memory",
	})
	resourceValuesProps.Set("ephemeral-storage", &jsonschema.Schema{
		Type:  &jsonschema.Type{Types: []string{"string"}},
		Title: "ephemeral-storage",
	})

	resourcesProps.Set("requests", &jsonschema.Schema{
		Type:                 &jsonschema.Type{Types: []string{"object"}},
		Title:                "requests",
		Properties:           resourceValuesProps,
		AdditionalProperties: jsonschema.FalseSchema,
	})
	resourcesProps.Set("limits", &jsonschema.Schema{
		Type:                 &jsonschema.Type{Types: []string{"object"}},
		Title:                "limits",
		Properties:           resourceValuesProps,
		AdditionalProperties: jsonschema.FalseSchema,
	})

	devProps.Set("resources", &jsonschema.Schema{
		Type:                 &jsonschema.Type{Types: []string{"object"}},
		Title:                "resources",
		Description:          withManifestRefDocLink("Resource requests and limits for the development container", "resources-object-optional"),
		Properties:           resourcesProps,
		AdditionalProperties: jsonschema.FalseSchema,
	})

	return &jsonschema.Schema{
		Type:                 &jsonschema.Type{Types: []string{"object"}},
		AdditionalProperties: jsonschema.FalseSchema,
		PatternProperties: map[string]*jsonschema.Schema{
			".*": {
				Description:          withManifestRefDocLink("The name of each development container must match the name of the Kubernetes Deployment or Statefulset that you want to put on development mode. If the name of your Deployment or Statefulset is dynamically generated, use the selector field to match the Deployment or Statefulset by labels.", "dev-object-optional"),
				Type:                 &jsonschema.Type{Types: []string{"object"}},
				Properties:           devProps,
				AdditionalProperties: jsonschema.FalseSchema,
			},
		},
	}
}
