// +build integration

// Copyright 2020 The Okteto Authors
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

package integration

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"text/template"
	"time"

	"github.com/Masterminds/semver"
	ps "github.com/mitchellh/go-ps"
	okCmd "github.com/okteto/okteto/cmd"
	"github.com/okteto/okteto/pkg/config"
	k8Client "github.com/okteto/okteto/pkg/k8s/client"
	"github.com/okteto/okteto/pkg/syncthing"
	"go.undefinedlabs.com/scopeagent"
	"go.undefinedlabs.com/scopeagent/agent"
	"go.undefinedlabs.com/scopeagent/instrumentation/nethttp"
	"go.undefinedlabs.com/scopeagent/instrumentation/process"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	yaml "gopkg.in/yaml.v2"
)

var (
	deploymentTemplate = template.Must(template.New("deployment").Parse(deploymentFormat))
	manifestTemplate   = template.Must(template.New("manifest").Parse(manifestFormat))
	user               = ""
	kubectlBinary      = "kubectl"
)

type deployment struct {
	Name string
}

const (
	deploymentFormat = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ .Name }}
spec:
  replicas: 1
  selector:
    matchLabels:
      app: {{ .Name }}
  template:
    metadata:
      labels:
        app: {{ .Name }}
    spec:
      terminationGracePeriodSeconds: 1
      containers:
      - name: test
        image: python:alpine
        ports:
        - containerPort: 8080
        workingDir: /usr/src/app
        command:
            - "python"
            - "-m"
            - "http.server"
            - "8080"
---
apiVersion: v1
kind: Service
metadata:
  name: {{ .Name }}
  annotations:
    dev.okteto.com/auto-ingress: "true"
spec:
  type: ClusterIP  
  ports:
  - name: {{ .Name }}
    port: 8080
  selector:
    app: {{ .Name }}
`
	manifestFormat = `
name: {{ .Name }}
image: python:alpine
command:
  - "python"
  - "-m"
  - "http.server"
  - "8080"
forward:
  - 8080:8080
workdir: /usr/src/app
`
)

var mode string

func TestMain(m *testing.M) {
	if u, ok := os.LookupEnv("OKTETO_USER"); !ok {
		log.Println("OKTETO_USER is not defined")
		os.Exit(1)
	} else {
		user = u
	}

	mode = "server"
	if _, ok := os.LookupEnv("OKTETO_CLIENTSIDE_TRANSLATION"); ok {
		log.Println("running in CLIENTSIDE mode")
		mode = "client"
	}

	if _, ok := os.LookupEnv("SCOPE_APIKEY"); ok {
		log.Println("SCOPE is enabled")
		nethttp.PatchHttpDefaultClient()
	}

	if runtime.GOOS == "windows" {
		kubectlBinary = "kubectl.exe"
	}

	os.Exit(scopeagent.Run(m, agent.WithMetadata(map[string]interface{}{
		"mode": mode,
	})))
}

func TestGetVersion(t *testing.T) {
	v, err := okCmd.GetLatestVersionFromGithub()
	if err != nil {
		t.Fatal(err)
	}

	_, err = semver.NewVersion(v)
	if err != nil {
		t.Fatal(err)
	}
}

func TestDownloadSyncthing(t *testing.T) {
	var tests = []struct {
		os string
	}{
		{os: "windows"}, {os: "darwin"}, {os: "linux"}, {os: "arm64"},
	}

	test := scopeagent.GetTest(t)
	for _, tt := range tests {
		test.Run(tt.os, func(t *testing.T) {
			u, err := syncthing.GetDownloadURL(tt.os, "amd64")
			req, err := http.NewRequest("GET", u, nil)
			if err != nil {
				t.Fatal(err.Error())
			}

			req = req.WithContext(test.Context())
			res, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("Failed to download syncthing: %s", err)
			}

			if res.StatusCode != 200 {
				t.Fatalf("Failed to download syncthing. Got status: %d", res.StatusCode)
			}
		})
	}
}

func TestAll(t *testing.T) {
	ctx := scopeagent.GetContextFromTest(t)
	tName := fmt.Sprintf("TestAll-%s-%s", runtime.GOOS, mode)

	t.Run(tName, func(t *testing.T) {
		oktetoPath, err := getOktetoPath(ctx)
		if err != nil {
			t.Fatal(err)
		}

		if _, err := exec.LookPath(kubectlBinary); err != nil {
			t.Fatalf("kubectl is not in the path: %s", err)
		}

		k8Client.Reset()

		name := strings.ToLower(fmt.Sprintf("%s-%d", tName, time.Now().Unix()))
		namespace := fmt.Sprintf("%s-%s", name, user)

		dir, err := ioutil.TempDir("", tName)
		if err != nil {
			t.Fatal(err)
		}
		log.Printf("created tempdir: %s", dir)

		dPath := filepath.Join(dir, "deployment.yaml")
		if err := writeDeployment(name, dPath); err != nil {
			t.Fatal(err)
		}

		contentPath := filepath.Join(dir, "index.html")
		if err := ioutil.WriteFile(contentPath, []byte(name), 0644); err != nil {
			t.Fatal(err)
		}

		log.Printf("original content: %s", name)

		manifestPath := filepath.Join(dir, "okteto.yml")
		if err := writeManifest(manifestPath, name); err != nil {
			t.Fatal(err)
		}

		if err := createNamespace(ctx, oktetoPath, namespace); err != nil {
			t.Fatal(err)
		}

		if err := deploy(ctx, name, dPath); err != nil {
			t.Fatal(err)
		}

		deployment, err := getDeployment(namespace, name)
		if err != nil {
			t.Fatal(err)
		}

		var wg sync.WaitGroup
		p, err := up(ctx, &wg, namespace, name, manifestPath, oktetoPath)
		if err != nil {
			t.Fatal(err)
		}

		log.Println("getting synchronized content")
		endpoint := "http://localhost:8080/index.html"

		c, err := getContent(endpoint, 120)
		if err != nil {
			t.Fatalf("failed to get content: %s", err)
		}

		log.Println("got synchronized content")

		if c != name {
			t.Fatalf("expected synchronized content to be %s, got %s", name, c)
		}

		// Update content in token file
		updatedContent := fmt.Sprintf("%s-%d", name, time.Now().Unix())
		if err := ioutil.WriteFile(contentPath, []byte(updatedContent), 0644); err != nil {
			t.Fatalf("failed to update %s: %s", contentPath, err)
		}

		time.Sleep(1 * time.Second)

		log.Printf("getting updated content from %s\n", endpoint)

		gotUpdated := false
		for i := 0; i < 10; i++ {
			c, err = getContent(endpoint, 120)
			if err != nil {
				t.Fatalf("failed to get updated content: %s", err)
			}

			if c != updatedContent {
				log.Printf("expected updated content to be %s, got %s\n", updatedContent, c)
				time.Sleep(3 * time.Second)
				continue
			}

			log.Println("got updated content")
			gotUpdated = true
			break
		}

		if !gotUpdated {
			t.Fatal("never got the updated content")
		}

		if err := down(ctx, name, manifestPath, oktetoPath); err != nil {
			t.Fatal(err)
		}

		if err := checkIfUpFinished(ctx, p.Pid); err != nil {
			t.Error(err)
		}

		if err := compareDeployment(deployment); err != nil {
			t.Error(err)
		}

		if err := deleteNamespace(ctx, oktetoPath, namespace); err != nil {
			log.Printf("failed to delete namespace %s: %s\n", namespace, err)
		}
	})
}

func waitForDeployment(ctx context.Context, name string, revision, timeout int) error {
	for i := 0; i < timeout; i++ {
		args := []string{"rollout", "status", "deployment", name, "--revision", fmt.Sprintf("%d", revision)}

		cmd := exec.Command(kubectlBinary, args...)
		cmd.Env = os.Environ()
		span, _ := process.InjectToCmdWithSpan(ctx, cmd)
		defer span.Finish()

		o, _ := cmd.CombinedOutput()
		log.Printf("%s %s", kubectlBinary, strings.Join(args, " "))
		output := string(o)
		log.Println(output)

		if strings.Contains(output, "is different from the running revision") {
			time.Sleep(1 * time.Second)
			continue
		}

		if strings.Contains(output, "successfully rolled out") {
			log.Println(output)
			return nil
		}

		time.Sleep(1 * time.Second)
	}

	return fmt.Errorf("%s didn't rollout after 30 seconds", name)
}

func getContent(endpoint string, totRetries int) (string, error) {
	retries := 0
	t := time.NewTicker(1 * time.Second)
	for i := 0; i < totRetries; i++ {
		r, err := http.Get(endpoint)
		if err != nil {
			retries++
			if retries > 3 {
				return "", fmt.Errorf("failed to get %s: %w", endpoint, err)
			}

			log.Printf("called %s, got %s, retrying", endpoint, err)
			<-t.C
			continue
		}

		defer r.Body.Close()
		if r.StatusCode != 200 {
			log.Printf("called %s, got status %d, retrying", endpoint, r.StatusCode)
			<-t.C
			continue
		}

		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			return "", err
		}

		return string(body), nil
	}

	return "", fmt.Errorf("service wasn't available")
}

func writeManifest(path, name string) error {
	oFile, err := os.Create(path)
	if err != nil {
		return err
	}

	defer oFile.Close()

	if err := manifestTemplate.Execute(oFile, deployment{Name: name}); err != nil {
		return err
	}

	return nil
}

func createNamespace(ctx context.Context, oktetoPath, namespace string) error {
	log.Printf("creating namespace %s", namespace)
	args := []string{"create", "namespace", namespace, "-l", "debug"}
	cmd := exec.Command(oktetoPath, args...)
	cmd.Env = os.Environ()
	span, _ := process.InjectToCmdWithSpan(ctx, cmd)
	defer span.Finish()
	o, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s: %s", oktetoPath, strings.Join(args, " "), string(o))
	}

	log.Printf("create namespace output: \n%s\n", string(o))

	_, _, n, err := k8Client.GetLocal()
	if err != nil {
		return err
	}

	if namespace != n {
		return fmt.Errorf("current namespace is %s, expected %s", n, namespace)
	}

	return nil
}

func deleteNamespace(ctx context.Context, oktetoPath, namespace string) error {
	log.Printf("okteto delete namespace %s", namespace)
	deleteCMD := exec.Command(oktetoPath, "delete", "namespace", namespace)
	deleteCMD.Env = os.Environ()
	span, _ := process.InjectToCmdWithSpan(ctx, deleteCMD)
	defer span.Finish()
	o, err := deleteCMD.CombinedOutput()
	if err != nil {
		return fmt.Errorf("okteto delete namespace failed: %s - %s", string(o), err)
	}

	return nil
}

func down(ctx context.Context, name, manifestPath, oktetoPath string) error {
	log.Printf("okteto down -f %s -v", manifestPath)
	downCMD := exec.Command(oktetoPath, "down", "-f", manifestPath, "-v")

	downCMD.Env = os.Environ()
	span, _ := process.InjectToCmdWithSpan(ctx, downCMD)
	defer span.Finish()

	o, err := downCMD.CombinedOutput()

	log.Printf("okteto down output:\n%s", string(o))
	if err != nil {
		m, _ := ioutil.ReadFile(manifestPath)
		log.Printf("manifest: \n%s\n", string(m))
		return fmt.Errorf("okteto down failed: %s", err)
	}

	log.Println("waiting for the deployment to be restored")
	if err := waitForDeployment(ctx, name, 3, 60); err != nil {
		return err
	}

	return nil
}

func up(ctx context.Context, wg *sync.WaitGroup, namespace, name, manifestPath, oktetoPath string) (*os.Process, error) {
	log.Println("starting okteto up")
	var out bytes.Buffer
	cmd := exec.Command(oktetoPath, "up", "-f", manifestPath)
	cmd.Env = os.Environ()
	cmd.Stdout = &out
	span, _ := process.InjectToCmdWithSpan(ctx, cmd)
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("okteto up failed to start: %s", err)
	}

	wg.Add(1)

	go func() {
		defer wg.Done()
		defer span.Finish()
		if err := cmd.Wait(); err != nil {
			if err != nil {
				log.Printf("okteto up exited: %s.\nOutput:\n%s", err, out.String())
			}
		}
	}()

	return cmd.Process, waitForReady(namespace, name)
}

func waitForUpExit(wg *sync.WaitGroup) error {
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-time.After(30 * time.Second):
		return fmt.Errorf("up didn't exit after 30 seconds")
	}
}

func waitForReady(namespace, name string) error {
	log.Println("waiting for okteto up to be ready")

	state := path.Join(config.GetOktetoHome(), namespace, name, "okteto.state")

	t := time.NewTicker(1 * time.Second)
	for i := 0; i < 180; i++ {
		c, err := ioutil.ReadFile(state)
		if err != nil {
			log.Printf("failed to read state file %s: %s", state, err)
			if !os.IsNotExist(err) {
				return err
			}

			<-t.C
			continue
		}

		if i%5 == 0 {
			log.Printf("okteto up is: %s", c)
		}

		if string(c) == "ready" {
			log.Printf("okteto up is: %s", c)
			return nil
		} else if string(c) == "failed" {
			return fmt.Errorf("development container failed")
		}

		<-t.C
	}

	return fmt.Errorf("development container was never ready")
}

func deploy(ctx context.Context, name, path string) error {
	log.Printf("deploying kubernetes manifest %s", path)
	cmd := exec.Command(kubectlBinary, "apply", "-f", path)
	cmd.Env = os.Environ()
	span, _ := process.InjectToCmdWithSpan(ctx, cmd)
	defer span.Finish()

	if o, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("kubectl apply failed: %s", string(o))
	}

	if err := waitForDeployment(ctx, name, 1, 60); err != nil {
		return err
	}

	return nil
}

func writeDeployment(name, path string) error {
	dFile, err := os.Create(path)
	if err != nil {
		return err
	}

	if err := deploymentTemplate.Execute(dFile, deployment{Name: name}); err != nil {
		return err
	}
	defer dFile.Close()
	return nil
}

func getOktetoPath(ctx context.Context) (string, error) {
	oktetoPath, ok := os.LookupEnv("OKTETO_PATH")
	if !ok {
		oktetoPath = "/usr/local/bin/okteto"
	}

	log.Printf("using %s", oktetoPath)

	var err error
	oktetoPath, err = filepath.Abs(oktetoPath)
	if err != nil {
		return "", err
	}

	cmd := exec.Command(oktetoPath, "version")
	cmd.Env = os.Environ()
	span, _ := process.InjectToCmdWithSpan(ctx, cmd)
	defer span.Finish()

	o, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("okteto version failed: %s - %s", string(o), err)
	}

	log.Println(string(o))
	return oktetoPath, nil
}

func getDeployment(ns, name string) (*appsv1.Deployment, error) {
	client, _, _, err := k8Client.GetLocal()
	if err != nil {
		return nil, err
	}

	return client.AppsV1().Deployments(ns).Get(name, metav1.GetOptions{})
}

func compareDeployment(deployment *appsv1.Deployment) error {
	after, err := getDeployment(deployment.GetNamespace(), deployment.GetName())
	if err != nil {
		return err
	}

	b, err := yaml.Marshal(deployment.Spec)
	if err != nil {
		return err
	}

	a, err := yaml.Marshal(after.Spec)
	if err != nil {
		return err
	}

	if string(a) != string(b) {
		return fmt.Errorf("got:\n%s\nexpected:\n%s", string(a), string(b))
	}

	return nil
}

func checkIfUpFinished(ctx context.Context, pid int) error {
	var err error
	t := time.NewTicker(1 * time.Second)
	for i := 0; i < 30; i++ {
		var found ps.Process
		found, err = ps.FindProcess(pid)
		if err == nil && found == nil {
			return nil
		}

		if err != nil {
			err = fmt.Errorf("error when finding process: %s", err)
		} else if found != nil {
			err = fmt.Errorf("okteto up didn't exit after down")
		}

		<-t.C
	}

	return err
}
