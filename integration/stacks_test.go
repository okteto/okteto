//go:build integration
// +build integration

// Copyright 2022 The Okteto Authors
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
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"
	"time"
)

const (
	stackGitRepo     = "git@github.com:okteto/stacks-getting-started.git"
	stackGitFolder   = "stacks-getting-started"
	stackManifest    = "docker-compose.yml"
	composeGitRepo   = "git@github.com:okteto/flask-producer-consumer.git"
	composeGitFolder = "flask-producer-consumer"
)

func TestStacks(t *testing.T) {

	ctx := context.Background()
	oktetoPath, err := getOktetoPath(ctx)
	if err != nil {
		t.Fatal(err)
	}

	tName := fmt.Sprintf("TestStacks-%s", runtime.GOOS)
	name := strings.ToLower(fmt.Sprintf("%s-%d", tName, time.Now().Unix()))
	namespace := fmt.Sprintf("%s-%s", name, user)
	t.Run(tName, func(t *testing.T) {
		log.Printf("running %s \n", tName)
		startNamespace := getCurrentNamespace()
		defer changeToNamespace(ctx, oktetoPath, startNamespace)
		if err := createNamespace(ctx, oktetoPath, namespace); err != nil {
			t.Fatal(err)
		}

		log.Printf("created namespace %s \n", namespace)

		if err := cloneGitRepo(ctx, stackGitRepo); err != nil {
			t.Fatal(err)
		}

		log.Printf("cloned repo %s \n", stackGitRepo)

		defer deleteGitRepo(ctx, stackGitFolder)

		if err := deployStack(ctx, oktetoPath, stackManifest, stackGitFolder); err != nil {
			t.Fatal(err)
		}

		log.Printf("deployed stack using %s\n", stackManifest)

		endpoint := fmt.Sprintf("https://vote-%s.%s", namespace, appsSubdomain)
		content, err := getContent(endpoint, 150, nil)
		if err != nil {
			t.Fatalf("failed to get stack content: %s", err)
		}

		if !strings.Contains(content, "Cats vs Dogs!") {
			t.Fatalf("wrong stack content: %s", content)
		}
		if err := destroyStack(ctx, oktetoPath, stackManifest, stackGitFolder); err != nil {
			t.Fatal(err)
		}

		log.Println("destroyed stack")

		time.Sleep(5 * time.Second)
		_, err = getDeployment(ctx, namespace, "vote")
		if err == nil {
			t.Fatalf("'vote' deployment not deleted after 'okteto stack destroy'")
		}

		if !strings.Contains(err.Error(), "not found") {
			t.Fatalf("error getting deployment 'vote': %s", err.Error())
		}

		if err := deleteNamespace(ctx, oktetoPath, namespace); err != nil {
			log.Printf("failed to delete namespace %s: %s\n", namespace, err)
		}
	})
}

func deployStack(ctx context.Context, oktetoPath, stackPath, dir string) error {
	log.Printf("okteto stack deploy %s", stackPath)
	cmd := exec.Command(oktetoPath, "stack", "deploy", "-f", stackPath, "--build", "--wait")
	cmd.Env = os.Environ()
	cmd.Dir = dir

	stderr, _ := cmd.StderrPipe()
	cmd.Start()

	scanner := bufio.NewScanner(stderr)
	scanner.Split(bufio.ScanWords)
	for scanner.Scan() {
		m := scanner.Text()
		fmt.Println(m)
	}
	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("okteto stack deploy failed: %s", err)
	}
	log.Printf("okteto stack deploy %s success", stackPath)
	return nil
}

func destroyStack(ctx context.Context, oktetoPath, stackManifest, dir string) error {
	log.Printf("okteto stack destroy")
	cmd := exec.Command(oktetoPath, "stack", "destroy", "-f", stackManifest)
	cmd.Env = os.Environ()
	cmd.Dir = dir
	o, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("okteto stack destroy failed: %s - %s", string(o), err)
	}
	log.Printf("okteto stack destroy success")
	return nil
}

func destroyStackWithVolumes(ctx context.Context, oktetoPath, stackManifest, dir string) error {
	log.Printf("okteto stack destroy with volumes")
	cmd := exec.Command(oktetoPath, "stack", "destroy", "-v", "-f", stackManifest)
	cmd.Env = os.Environ()
	cmd.Dir = dir
	o, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("okteto stack destroy failed: %s - %s", string(o), err)
	}
	log.Printf("okteto stack destroy success")
	return nil
}

func TestCompose(t *testing.T) {

	ctx := context.Background()
	oktetoPath, err := getOktetoPath(ctx)
	if err != nil {
		t.Fatal(err)
	}

	tName := fmt.Sprintf("TestStacks-%s", runtime.GOOS)
	name := strings.ToLower(fmt.Sprintf("%s-%d", tName, time.Now().Unix()))
	namespace := fmt.Sprintf("%s-%s", name, user)

	startNamespace := getCurrentNamespace()
	defer changeToNamespace(ctx, oktetoPath, startNamespace)
	if err := createNamespace(ctx, oktetoPath, namespace); err != nil {
		t.Fatal(err)
	}

	log.Printf("created namespace %s \n", namespace)

	if err := cloneGitRepo(ctx, composeGitRepo); err != nil {
		t.Fatal(err)
	}

	if err := deployStack(ctx, oktetoPath, "docker-compose.yml", composeGitFolder); err != nil {
		t.Fatal(err)
	}
	defer deleteGitRepo(ctx, composeGitFolder)

	log.Printf("deployed stack using %s \n", "docker-compose.yml")

	jobEndpoint := fmt.Sprintf("https://nginx-%s.%s/db/initialized", namespace, appsSubdomain)
	content, err := getContent(jobEndpoint, 200, nil)
	if err != nil {
		t.Fatalf("failed to get stack content: %s", err)
	}

	endpoint := fmt.Sprintf("https://nginx-%s.%s/db", namespace, appsSubdomain)
	content, err = getContent(endpoint, 200, nil)
	if err != nil {
		t.Fatalf("failed to get stack content: %s", err)
	}
	var objmap map[string]json.RawMessage
	err = json.Unmarshal([]byte(content), &objmap)

	if strings.ToUpper(string(objmap["lower"])) != string(objmap["uppercase"]) {
		t.Fatal("Not the right response")
	}

	log.Println("Starting to redeploy stack")
	if err := deployStack(ctx, oktetoPath, "docker-compose.yml", composeGitFolder); err != nil {
		t.Fatal(err)
	}
	log.Println("Stack has been redeployed successfully")

	if err := destroyStack(ctx, oktetoPath, "docker-compose.yml", composeGitFolder); err != nil {
		t.Fatal(err)
	}

	log.Println("destroyed stack")

	time.Sleep(5 * time.Second)
	_, err = getDeployment(ctx, namespace, "api")
	if err == nil {
		t.Fatalf("'api' deployment not deleted after 'okteto stack destroy'")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Fatalf("error getting deployment 'api': %s", err.Error())
	}

	_, err = getDeployment(ctx, namespace, "producer")
	if err == nil {
		t.Fatalf("'producer' deployment not deleted after 'okteto stack destroy'")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Fatalf("error getting deployment 'producer': %s", err.Error())
	}

	_, err = getDeployment(ctx, namespace, "consumer")
	if err == nil {
		t.Fatalf("'consumer' deployment not deleted after 'okteto stack destroy'")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Fatalf("error getting deployment 'consumer': %s", err.Error())
	}

	_, err = getDeployment(ctx, namespace, "web-svc")
	if err == nil {
		t.Fatalf("'web-svc' deployment not deleted after 'okteto stack destroy'")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Fatalf("error getting deployment 'web-svc': %s", err.Error())
	}

	_, err = getStatefulset(ctx, namespace, "mongodb")
	if err == nil {
		t.Fatalf("'mongodb' statefulset not deleted after 'okteto stack destroy'")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Fatalf("error getting statefulset 'mongodb': %s", err.Error())
	}

	_, err = getStatefulset(ctx, namespace, "rabbitmq")
	if err == nil {
		t.Fatalf("'rabbitmq' statefulset not deleted after 'okteto stack destroy'")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Fatalf("error getting statefulset 'rabbitmq': %s", err.Error())
	}

	_, err = getJob(ctx, namespace, "initialize-queue")
	if err == nil {
		t.Fatalf("'initialize-queue' job not deleted after 'okteto stack destroy'")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Fatalf("error getting job 'initialize-queue': %s", err.Error())
	}

	_, err = getVolume(ctx, namespace, "volume")
	if err != nil {
		t.Fatalf("'volume' volume deleted after 'okteto stack destroy'")
	}

	if err := destroyStackWithVolumes(ctx, oktetoPath, "docker-compose.yml", composeGitFolder); err != nil {
		t.Fatal(err)
	}

	log.Println("destroyed stack and volumes")

	_, err = getVolume(ctx, namespace, "volume")
	if err == nil {
		t.Fatalf("'volume' volume not deleted after 'okteto stack destroy -v'")
	}

	if err := deleteNamespace(ctx, oktetoPath, namespace); err != nil {
		log.Printf("failed to delete namespace %s: %s\n", namespace, err)
	}
}
