package test

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
	"time"

	k8Client "github.com/okteto/okteto/pkg/k8s/client"
)

func TestAll(t *testing.T) {
	token := os.Getenv("OKTETO_TOKEN")
	if len(token) == 0 {
		log.Println("OKTETO_TOKEN is not defined, using logged in user")
	}

	user := os.Getenv("OKTETO_USER")
	if len(user) == 0 {
		t.Fatalf("OKTETO_USER is not defined")
	}

	oktetoPath := os.Getenv("OKTETO_PATH")
	if len(oktetoPath) == 0 {
		oktetoPath = "/usr/local/bin/okteto"
	}

	name := strings.ToLower(fmt.Sprintf("%s-%d", t.Name(), time.Now().Unix()))
	namespace := fmt.Sprintf("%s-%s", name, user)
	// Create namespace
	args := []string{"create", "namespace", namespace, "-l", "debug"}
	cmd := exec.Command(oktetoPath, args...)
	cmd.Env = os.Environ()
	if o, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%s %s: %s", oktetoPath, args, string(o))
	}

	_, _, n, err := k8Client.GetLocal()
	if err != nil {
		t.Fatal(err)
	}

	if namespace != n {
		t.Fatalf("current namespace is %s, expected %s", namespace, name)
	}

	log.Printf("created namespace: %s", namespace)

	// Create kubernetes manifest
	dir, err := ioutil.TempDir("", t.Name())
	if err != nil {
		t.Fatal(err)
	}

	log.Printf("created tempdir: %s", dir)

	dPath := filepath.Join(dir, name)
	dFile, err := os.Create(dPath)
	if err != nil {
		t.Fatal(err)
	}

	if err := deploymentTemplate.Execute(dFile, deployment{Name: name}); err != nil {
		t.Fatal(err)
	}

	// Deploy kubernetes manifest
	cmd = exec.Command("/usr/local/bin/kubectl", "apply", "-f", dPath)
	if o, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("kubectl apply failed: %s", string(o))
	}

	if err := waitForDeployment(name, 1, 30); err != nil {
		t.Fatal(err)
	}

	log.Println("started deployment")

	// Create token file
	tPath := filepath.Join(dir, "index.html")
	ioutil.WriteFile(tPath, []byte(name), 0644)

	// Write okteto manifest
	oPath := filepath.Join(dir, "okteto.yml")
	if err := writeManifest(oPath, name); err != nil {
		t.Fatal(err)
	}

	// Start okteto
	upCMD := exec.Command(oktetoPath, "up", "-f", oPath)
	log.Println("starting okteto up")

	//stdout, err := upCMD.StdoutPipe()
	//if err != nil {
	//	t.Fatal(err)
	//}

	go func() {
		if err := upCMD.Start(); err != nil {
			t.Fatalf("okteto up failed to start: %s", err)
		}

		if err := upCMD.Wait(); err != nil {
			log.Printf("okteto up exited: %s", err)
		}
	}()

	log.Println("waiting for the statefulset")
	if err := waitForPod(fmt.Sprintf("okteto-%s-0", name), 100); err != nil {
		t.Fatal(err)
	}

	log.Println("waiting for the deployment")
	if err := waitForDeployment(name, 2, 30); err != nil {
		t.Fatal(err)
	}

	// Check content
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
	ioutil.WriteFile(tPath, []byte(updatedContent), 0644)
	time.Sleep(3 * time.Second)

	log.Println("getting updated content")
	c, err = getContent(name, namespace)
	if err != nil {
		t.Fatal(err)
	}

	if c != updatedContent {
		t.Fatalf("expected updated content to be %s, got %s", updatedContent, c)
	}
}

func waitForDeployment(name string, revision, timeout int) error {
	for i := 0; i < timeout; i++ {
		args := []string{"rollout", "status", "deployment", name, "--revision", fmt.Sprintf("%d", revision)}
		cmd := exec.Command("/usr/local/bin/kubectl", args...)
		o, _ := cmd.CombinedOutput()
		log.Printf("/usr/local/bin/kubectl %s", strings.Join(args, " "))
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
	for i := 0; i < 10; i++ {
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

func waitForPod(name string, timeout int) error {
	for i := 0; i < timeout; i++ {
		args := []string{"get", "pod", "-ojsonpath='{.status.phase}'", name}
		cmd := exec.Command("/usr/local/bin/kubectl", args...)
		o, err := cmd.CombinedOutput()
		if err != nil {
			log.Printf("`kubectl %s` failed: %s", strings.Join(args, " "), err)
			continue
		}

		output := string(o)
		log.Printf("`kubectl %s` output: %s", strings.Join(args, " "), output)
		if strings.Contains(output, "Running") {
			return nil
		}

		time.Sleep(5 * time.Second)
	}

	return fmt.Errorf("pod/%s wasn't ready after %d seconds", name, timeout)

}
