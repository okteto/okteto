package integration

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"text/template"
	"time"

	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/registry"
)

const (
	buildGitRepo   = "git@github.com:okteto/react-getting-started.git"
	buildGitFolder = "react-getting-started"
	buildManifest  = "okteto.yml"
)

func TestBuildCommand(t *testing.T) {

	ctx := context.Background()
	oktetoPath, err := getOktetoPath(ctx)
	if err != nil {
		t.Fatal(err)
	}

	testName := fmt.Sprintf("TestBuildCommand-%s", runtime.GOOS)
	testRef := strings.ToLower(fmt.Sprintf("%s-%d", testName, time.Now().Unix()))
	namespace := fmt.Sprintf("%s-%s", testRef, user)

	t.Run(testName, func(t *testing.T) {
		log.Printf("running test %s", testName)

		// setup namespace for test
		startNamespace := getCurrentNamespace()
		defer changeToNamespace(ctx, oktetoPath, startNamespace)

		if err := createNamespace(ctx, oktetoPath, namespace); err != nil {
			t.Fatal(err)
		}
		log.Printf("created namespace %s \n", namespace)
		defer deleteNamespace(ctx, oktetoPath, namespace)

		// clone repo
		if err := cloneGitRepo(ctx, buildGitRepo); err != nil {
			t.Fatal(err)
		}
		log.Printf("cloned repo %s \n", buildGitRepo)
		defer deleteGitRepo(ctx, buildGitFolder)

		cwd, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}

		dir, err := os.MkdirTemp(cwd, testName)
		if err != nil {
			t.Fatal(err)
		}
		log.Printf("created tempdir: %s", dir)
		defer os.RemoveAll(dir)

		manifestPath := filepath.Join(dir, "okteto.yml")
		if err := writeManifestV2(manifestPath, testRef); err != nil {
			t.Fatal(err)
		}

		if err := oktetoBuild(ctx, oktetoPath, manifestPath); err != nil {
			t.Fatal(err)
		}

		log.Println("checking at registry if the image has been built...")
		expectedImageTag := fmt.Sprintf("%s/%s/%s:dev", okteto.Context().Registry, namespace, buildGitFolder)
		if _, err := registry.GetImageTagWithDigest(expectedImageTag); err != nil {
			log.Println("error getting the image digest")
			t.Fatal(err)
		}

	})
}

func oktetoBuild(ctx context.Context, oktetoPath, oktetoManifestPath string) error {
	log.Printf("okteto build %s", oktetoManifestPath)

	// enable feature for using the manifest at build
	os.Setenv("OKTETO_ENABLE_MANIFEST_V2", "true")

	cmd := exec.Command(oktetoPath, "build", "-f", oktetoManifestPath, "-l", "debug")
	cmd.Env = os.Environ()
	cmd.Dir = buildGitFolder
	o, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("okteto build failed: %s - %s", string(o), err)
	}
	log.Printf("okteto build %s success", oktetoManifestPath)
	return nil
}

var (
	v2ManifestTemplate = template.Must(template.New("manifest_v2").Parse(v2ManifestFormat))
)

const (
	v2ManifestFormat = `build:
  react-getting-started:
    context: .
`
)

type templateVars struct {
	Name string
}

func writeManifestV2(path, name string) error {
	oFile, err := os.Create(path)
	if err != nil {
		return err
	}
	defer oFile.Close()

	if err := v2ManifestTemplate.Execute(oFile, templateVars{Name: name}); err != nil {
		return err
	}

	return nil
}
