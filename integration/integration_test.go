// +build integration

package integration

import (
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

func TestDownloadSyncthing(t *testing.T) {
	var tests = []struct {
		os string
	}{
		{os: "windows"}, {os: "darwin"}, {os: "linux"}, {os: "arm64"},
	}

	for _, tt := range tests {
		t.Run(tt.os, func(t *testing.T) {
			res, err := http.Head(cmd.SyncthingURL[tt.os])
			if err != nil {
				t.Errorf("Failed to download syncthing: %s", err)
			}

			if res.StatusCode != 200 {
				t.Errorf("Failed to download syncthing. Got status: %d", res.StatusCode)
			}
		})
	}
}
func TestAll(t *testing.T) {
	token := os.Getenv("OKTETO_TOKEN")
	if len(token) == 0 {
		log.Println("OKTETO_TOKEN is not defined, using logged in user")
	}

	user := os.Getenv("OKTETO_USER")
	if len(user) == 0 {
		t.Fatalf("OKTETO_USER is not defined")
	}

	_, ok := os.LookupEnv("OKTETO_CLIENTSIDE_TRANSLATION")
	if ok {
		log.Println("running in CLIENTSIDE mode")
	}

	oktetoPath, err := getOktetoPath()
	if err != nil {
		t.Fatal(err)
	}

	if _, err := exec.LookPath("kubectl"); err != nil {
		t.Fatalf("kubectl is not in the path: %s", err)
	}

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

	if err := createNamespace(namespace, oktetoPath); err != nil {
		t.Fatal(err)
	}

	if err := deploy(name, dPath); err != nil {
		t.Fatal(err)
	}

	if err := up(namespace, name, manifestPath, oktetoPath); err != nil {
		t.Fatal(err)
	}

	log.Println("getting synchronized content")
	c, err := getContent(name, namespace)
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
	c, err = getContent(name, namespace)
	if err != nil {
		t.Fatal(err)
	}

	if c != updatedContent {
		t.Fatalf("expected updated content to be %s, got %s", updatedContent, c)
	}

	if err := down(name, manifestPath, oktetoPath); err != nil {
		t.Fatal(err)
	}

	if err := deleteNamespace(oktetoPath, namespace); err != nil {
		t.Fatal(err)
	}
}

func waitForDeployment(name string, revision, timeout int) error {
	for i := 0; i < timeout; i++ {
		args := []string{"rollout", "status", "deployment", name, "--revision", fmt.Sprintf("%d", revision)}
		cmd := exec.Command("kubectl", args...)
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

func getContent(name, namespace string) (string, error) {
	endpoint := fmt.Sprintf("https://%s-%s.cloud.okteto.net/", name, namespace)
	for i := 0; i < 60; i++ {
		r, err := http.Get(endpoint)
		if err != nil {
			return "", err
		}

		defer r.Body.Close()
		if r.StatusCode != 200 {
			log.Printf("Got status %d, retrying", r.StatusCode)
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

func createNamespace(namespace, oktetoPath string) error {
	log.Printf("creating namespace %s", namespace)
	args := []string{"create", "namespace", namespace, "-l", "debug"}
	cmd := exec.Command(oktetoPath, args...)
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

func deleteNamespace(oktetoPath, namespace string) error {
	log.Printf("okteto delete namespace %s", namespace)
	deleteCMD := exec.Command(oktetoPath, "delete", "namespace", namespace)
	o, err := deleteCMD.CombinedOutput()
	if err != nil {
		return fmt.Errorf("okteto delete namespace failed: %s - %s", string(o), err)
	}

	return nil
}

func down(name, manifestPath, oktetoPath string) error {
	log.Printf("okteto down -f %s", manifestPath)
	downCMD := exec.Command(oktetoPath, "down", "-f", manifestPath)

	o, err := downCMD.CombinedOutput()
	log.Printf("okteto down output:\n%s", string(o))
	if err != nil {
		return fmt.Errorf("okteto down failed: %s", err)
	}

	log.Println("waiting for the deployment to be restored")
	if err := waitForDeployment(name, 3, 30); err != nil {
		return err
	}

	return nil
}

func up(namespace, name, manifestPath, oktetoPath string) error {
	log.Println("starting okteto up")
	upCMD := exec.Command(oktetoPath, "up", "-f", manifestPath)

	go func() {
		if err := upCMD.Start(); err != nil {
			log.Fatalf("okteto up failed to start: %s", err)
		}

		if err := upCMD.Wait(); err != nil {
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

func deploy(name, path string) error {
	log.Printf("deploying kubernetes manifest %s", path)
	cmd := exec.Command("kubectl", "apply", "-f", path)
	if o, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("kubectl apply failed: %s", string(o))
	}

	if err := waitForDeployment(name, 1, 30); err != nil {
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

func getOktetoPath() (string, error) {
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
	if o, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("okteto version failed: %s - %s", string(o), err)
	}

	return oktetoPath, nil
}
