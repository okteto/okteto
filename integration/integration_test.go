// +build integration

package integration

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"text/template"
	"time"

	"github.com/okteto/okteto/cmd"
	k8Client "github.com/okteto/okteto/pkg/k8s/client"
	"go.undefinedlabs.com/scopeagent"
	"go.undefinedlabs.com/scopeagent/instrumentation/nethttp"
	"go.undefinedlabs.com/scopeagent/instrumentation/process"
)

var (
	deploymentTemplate = template.Must(template.New("deployment").Parse(deploymentFormat))
	manifestTemplate   = template.Must(template.New("manifest").Parse(manifestFormat))
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
  annotations:
    dev.okteto.com/auto-ingress: "true"
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
workdir: /usr/src/app
`
)

var (
	url  = "cloud.okteto.net"
	user = ""
)

func TestMain(m *testing.M) {
	if u, ok := os.LookupEnv("OKTETO_URL"); ok {
		log.Printf("OKTETO_URL is defined, using %s\n", u)
		url = u
	}

	_, ok := os.LookupEnv("OKTETO_TOKEN")
	if !ok {
		log.Println("OKTETO_TOKEN is not defined, using logged in user")
	}

	if u, ok := os.LookupEnv("OKTETO_USER"); !ok {
		log.Println("OKTETO_USER is not defined")
		os.Exit(1)
	} else {
		user = u
	}

	if _, ok := os.LookupEnv("OKTETO_CLIENTSIDE_TRANSLATION"); ok {
		log.Println("running in CLIENTSIDE mode")
	}

	if _, ok := os.LookupEnv("SCOPE_APIKEY"); ok {
		log.Println("SCOPE is enabled")
		nethttp.PatchHttpDefaultClient()
	}

	os.Exit(scopeagent.Run(m))
}

func TestDownloadSyncthing(t *testing.T) {
	var tests = []struct {
		os string
	}{
		{os: "windows"}, {os: "darwin"}, {os: "linux"}, {os: "arm64"},
	}

	for _, tt := range tests {
		t.Run(tt.os, func(t *testing.T) {
			test := scopeagent.StartTest(t)
			defer test.End()

			req, err := http.NewRequest("HEAD", cmd.SyncthingURL[tt.os], nil)
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

func TestHealth(t *testing.T) {
	test := scopeagent.StartTest(t)
	defer test.End()

	ctx := test.Context()

	err := checkHealth(ctx, url)
	if err != nil {
		t.Fatalf("healthcheck failed: %s", err)
	}
}
func TestAll(t *testing.T) {
	test := scopeagent.StartTest(t)
	defer test.End()
	ctx := test.Context()

	oktetoPath, err := getOktetoPath(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := exec.LookPath("kubectl"); err != nil {
		t.Fatalf("kubectl is not in the path: %s", err)
	}

	err = checkHealth(ctx, url)
	if err != nil {
		t.Fatalf("healthcheck failed: %s", err)
	}

	log.Printf("%s is healthy \n", url)

	name := strings.ToLower(fmt.Sprintf("%s-%d", t.Name(), time.Now().Unix()))
	namespace := fmt.Sprintf("%s-%s", name, user)

	dir, err := ioutil.TempDir("", t.Name())
	if err != nil {
		t.Fatal(err)
	}
	log.Printf("created tempdir: %s", dir)

	dPath := filepath.Join(dir, "deployment.yaml")
	if err := writeDeployment(name, dPath); err != nil {
		t.Fatal(err)
	}

	contentPath := filepath.Join(dir, "index.html")
	ioutil.WriteFile(contentPath, []byte(name), 0644)

	manifestPath := filepath.Join(dir, "okteto.yml")
	if err := writeManifest(manifestPath, name); err != nil {
		t.Fatal(err)
	}

	if err := createNamespace(ctx, namespace, oktetoPath); err != nil {
		t.Fatal(err)
	}

	if err := deploy(ctx, name, dPath); err != nil {
		t.Fatal(err)
	}

	if err := up(ctx, namespace, name, manifestPath, oktetoPath); err != nil {
		t.Fatal(err)
	}

	log.Println("getting synchronized content")
	c, err := getContent(name, namespace, url)
	if err != nil {
		t.Fatal(err)
	}

	if c != name {
		t.Fatalf("expected content to be %s, got %s", name, c)
	}

	// Update content in token file
	updatedContent := fmt.Sprintf("%d", time.Now().Unix())
	ioutil.WriteFile(contentPath, []byte(updatedContent), 0644)
	time.Sleep(3 * time.Second)

	log.Println("getting updated content")
	c, err = getContent(name, namespace, url)
	if err != nil {
		t.Fatal(err)
	}

	if c != updatedContent {
		t.Fatalf("expected updated content to be %s, got %s", updatedContent, c)
	}

	if err := down(ctx, name, manifestPath, oktetoPath); err != nil {
		t.Fatal(err)
	}

	if err := deleteNamespace(ctx, oktetoPath, namespace); err != nil {
		t.Fatal(err)
	}
}

func waitForDeployment(ctx context.Context, name string, revision, timeout int) error {
	for i := 0; i < timeout; i++ {
		args := []string{"rollout", "status", "deployment", name, "--revision", fmt.Sprintf("%d", revision)}

		cmd := exec.Command("kubectl", args...)
		cmd.Env = os.Environ()

		span, _ := process.InjectToCmdWithSpan(ctx, cmd)
		defer span.Finish()

		o, _ := cmd.CombinedOutput()
		log.Printf("kubectl %s", strings.Join(args, " "))
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

func getContent(name, namespace, url string) (string, error) {
	endpoint := fmt.Sprintf("https://%s-%s.%s/", name, namespace, url)
	for i := 0; i < 60; i++ {
		r, err := http.Get(endpoint)
		if err != nil {
			return "", err
		}

		defer r.Body.Close()
		if r.StatusCode != 200 {
			log.Printf("Called %s, got status %d, retrying", endpoint, r.StatusCode)
			time.Sleep(1 * time.Second)
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

func checkHealth(ctx context.Context, url string) error {
	endpoint := fmt.Sprintf("https://okteto.%s/healthz", url)
	if url == "cloud.okteto.net" {
		endpoint = "https://cloud.okteto.com/healthz"
	}

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(req.WithContext(ctx))
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("bad status code from healthz %s: %d", endpoint, resp.StatusCode)
	}

	return nil
}

func createNamespace(ctx context.Context, namespace, oktetoPath string) error {
	log.Printf("creating namespace %s", namespace)
	args := []string{"create", "namespace", namespace, "-l", "debug"}
	cmd := exec.Command(oktetoPath, args...)
	cmd.Env = os.Environ()
	span, _ := process.InjectToCmdWithSpan(ctx, cmd)
	defer span.Finish()
	if o, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%s %s: %s", oktetoPath, strings.Join(args, " "), string(o))
	}

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
	log.Printf("okteto down -f %s", manifestPath)
	downCMD := exec.Command(oktetoPath, "down", "-f", manifestPath)

	downCMD.Env = os.Environ()
	span, _ := process.InjectToCmdWithSpan(ctx, downCMD)
	defer span.Finish()

	o, err := downCMD.CombinedOutput()

	log.Printf("okteto down output:\n%s", string(o))
	if err != nil {
		return fmt.Errorf("okteto down failed: %s", err)
	}

	log.Println("waiting for the deployment to be restored")
	if err := waitForDeployment(ctx, name, 3, 30); err != nil {
		return err
	}

	return nil
}

func up(ctx context.Context, namespace, name, manifestPath, oktetoPath string) error {
	log.Println("starting okteto up")
	cmd := exec.Command(oktetoPath, "up", "-f", manifestPath)
	cmd.Env = os.Environ()
	span, _ := process.InjectToCmdWithSpan(ctx, cmd)

	go func() {
		defer span.Finish()
		if err := cmd.Start(); err != nil {
			log.Fatalf("okteto up failed to start: %s", err)
		}

		if err := cmd.Wait(); err != nil {
			log.Printf("okteto up exited: %s", err)
		}

	}()

	return waitForReady(namespace, name)
}

func waitForReady(namespace, name string) error {
	state := fmt.Sprintf("%s/.okteto/%s/%s/okteto.state", os.Getenv("HOME"), namespace, name)
	t := time.NewTicker(1 * time.Second)
	for i := 0; i < 60; i++ {
		c, err := ioutil.ReadFile(state)
		if err != nil {
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
			return nil
		} else if string(c) == "failed" {
			return fmt.Errorf("dev environment failed")
		}

		<-t.C
	}

	return fmt.Errorf("dev environment was never ready")
}

func deploy(ctx context.Context, name, path string) error {
	log.Printf("deploying kubernetes manifest %s", path)
	cmd := exec.Command("kubectl", "apply", "-f", path)
	cmd.Env = os.Environ()
	span, _ := process.InjectToCmdWithSpan(ctx, cmd)
	defer span.Finish()

	if o, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("kubectl apply failed: %s", string(o))
	}

	if err := waitForDeployment(ctx, name, 1, 30); err != nil {
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
	oktetoPath := os.Getenv("OKTETO_PATH")
	if len(oktetoPath) == 0 {
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
