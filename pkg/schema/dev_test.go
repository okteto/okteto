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

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_Dev(t *testing.T) {
	tests := []struct {
		name      string
		manifest  string
		wantError bool
	}{
		{
			name: "empty",
			manifest: `
dev: {}`,
		},
		{
			name: "basic dev configuration",
			manifest: `
dev:
  api:
    command: ["bash"]
    forward:
      - 8080:8080
      - 9229:9229
    sync:
      - api:/usr/src/app
  frontend:
    command: yarn start
    sync:
      - frontend:/usr/src/app
`,
		},
		{
			name: "complex dev configuration",
			manifest: `
dev:
  api:
    affinity:
      podAffinity:
        requiredDuringSchedulingIgnoredDuringExecution:
          - labelSelector:
              matchExpressions:
                - key: role
                  operator: In
                  values:
                    - web-server
            topologyKey: kubernetes.io/hostname
    autocreate: true
    command: ["python", "main.py"]
    container: api
    environment:
      environment: development
      name: user-${USER:peter}
      DBPASSWORD:
    envFiles:
      - .env1
      - .env2
    externalVolumes:
      - go-cache:/root/.cache/go-build/
      - pvc-name:subpath:/var/lib/mysql
    forward:
      - 8080:80
      - 5432:postgres:5432
    initContainer:
      image: okteto/bin:1.2.22
      resources:
        requests:
          cpu: 30m
          memory: 30Mi
        limits:
          cpu: 30m
          memory: 30Mi
    interface: 0.0.0.0
    image: python:3
    imagePullPolicy: Always
    lifecycle: true
    metadata:
      annotations:
        fluxcd.io/ignore: "true"
      labels:
        custom.label/dev: "true"
    mode: hybrid
    nodeSelector:
      disktype: ssd
    persistentVolume:
      enabled: true
      accessMode: ReadWriteOnce
      storageClass: standard
      volumeMode: Filesystem
      size: 30Gi
      annotations:
        custom.annotation/dev: "true"
      labels:
        custom.label/dev: "true"
    priorityClassName: okteto
    probes:
      liveness: true
      readiness: true
      startup: true
    resources:
      requests:
        cpu: "250m"
        memory: "64Mi"
        ephemeral-storage: "64Mi"
      limits:
        cpu: "500m"
        memory: "1Gi"
        ephemeral-storage: "1Gi"
    remote: 2222
    reverse:
      - 9000:9001
      - 8080:8080
    secrets:
      - $HOME/.token:/root/.token:400
    securityContext:
      runAsUser: 1000
      runAsGroup: 2000
      fsGroup: 3000
      runAsNonRoot: true
      allowPrivilegeEscalation: false
      capabilities:
        add:
          - SYS_PTRACE
    selector:
      app.kubernetes.io/name: vote
    serviceAccount: default
    services:
      - name: worker
        annotations:
          custom.annotation/dev: "true"
        labels:
          custom.label/dev: "true" 
        command: ["test"]
        container: test
        environment:
          VAR: value
        workdir: ./test
        replicas: 2
        sync:
          - .:/app
    sync:
      - .:/code
      - config:/etc/config
      - $HOME/.ssh:/root/.ssh
    timeout: 5m
    tolerations:
      - key: nvidia.com/gpu
        operator: Exists
    volumes:
      - /go/pkg/
      - /root/.cache/go-build/
    workdir: /test`,
		},
		{
			name: "with forward object",
			manifest: `
dev:
  api:
    forward:
      - localPort: 8080
        remotePort: 80
        name: app
      - localPort: 5432
        remotePort: 5432
        labels:
          app: db`,
		},
		{
			name: "with lifecycle object",
			manifest: `
dev:
  api:
    lifecycle:
      postStart:
        enabled: true
        command: "echo 'Container has started'"
      preStop:
        enabled: true
        command: "echo 'Container is stopping'"`,
		},
		{
			name: "with sync object",
			manifest: `
dev:
  api:
    sync:
      folders:
        - .:/code
      verbose: false
      compression: true
      rescanInterval: 100`,
		},
		{
			name: "with timeout object",
			manifest: `
dev:
  api:
    timeout:
      default: 3m
      resources: 5m`,
		},
		{
			name: "invalid command type",
			manifest: `
dev:
  api:
    command: 123
`,
			wantError: true,
		},
		//		{
		//			name: "persistentVolume.enabled must be true when using services",
		//			manifest: `
		// dev:
		//  api:
		//    services:
		//      - name: worker
		//        replicas: 2
		//        command: ["test"]
		//        sync:
		//          - .:/app
		//    persistentVolume:
		//      enabled: false`,
		//			wantError: true,
		//		},
		// {
		//	name: "persistentVolume.enabled must be true when using volumes",
		// },
		{
			name: "invalid sync format",
			manifest: `
dev:
  api:
    sync: "invalid"
`,
			wantError: true,
		},
		{
			name: "invalid forward format",
			manifest: `
dev:
  api:
    forward:
      - "invalid:format"
`,
			wantError: true,
		},
		{
			name: "valid forward with object notation",
			manifest: `
dev:
  api:
    forward:
      - localPort: 8080
        remotePort: 8080
        name: web
`,
		},
		{
			name: "valid timeout formats",
			manifest: `
dev:
  api:
    timeout: 5m
  web:
    timeout:
      default: 10m
      resources: 20m
`,
		},
		{
			name: "invalid timeout format",
			manifest: `
dev:
  api:
    timeout: 5minutes
`,
			wantError: true,
		},
		{
			name: "valid probes configuration",
			manifest: `
dev:
  api:
    probes:
      liveness: true
      readiness: false
      startup: true
`,
		},
		{
			name: "valid lifecycle configuration",
			manifest: `
dev:
  api:
    lifecycle:
      postStart:
        command: echo "Starting"
      preStop:
        command: echo "Stopping"
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateOktetoManifest(tt.manifest)
			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
