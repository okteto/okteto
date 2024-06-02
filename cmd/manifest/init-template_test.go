package manifest

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/h2non/gock"
	"github.com/stretchr/testify/require"
)

func TestRunCreateTemplate(t *testing.T) {
	dir := t.TempDir()

	// Create and defer closing the template file
	tmplPath := filepath.Join(dir, "template.tmpl")
	tmplFile, err := os.Create(tmplPath)
	require.NoError(t, err)
	defer tmplFile.Close()

	// Define the template content
	tmplCnt := `name: {{ .name }}
namespace: {{ .namespace }}
context: {{ .context }}
dev:
  {{ .deployname }}:
    workdir: /usr/src/app/
    sync:
    - .:/usr/src/app
    image: {{ .image }}`

	// Write the template content to the file
	_, err = tmplFile.Write([]byte(tmplCnt))
	require.NoError(t, err)

	// Set up the context and options for running the command
	ctx := context.Background()

	mc := &Command{}
	opts := &InitOpts{
		DevPath:   "",
		Language:  "golang",
		Context:   "sample-ctx",
		Namespace: "sample-ns",
		Workdir:   dir,
		Template:  tmplPath,
		TemplateArgs: map[string]string{
			"name":       "test",
			"image":      "okteto/bin",
			"deployname": "test-deploy",
		},
	}

	// Run the command and check for errors
	man, err := mc.RunCreateTemplate(ctx, opts)
	require.NoError(t, err)

	// Validate the contents of the manifest
	dev := man.Dev["test-deploy"]
	require.Equal(t, "sample-ns", man.Namespace)
	require.Equal(t, "sample-ctx", man.Context)
	require.Equal(t, "okteto/bin", dev.Image.Name)
}

func TestRunCreateTemplateWithUrl(t *testing.T) {
	dir := t.TempDir()

	// Define the template content
	tmplCnt := `name: {{ .name }}
namespace: {{ .namespace }}
context: {{ .context }}
dev:
  {{ .deployname }}:
    workdir: /usr/src/app/
    sync:
    - .:/usr/src/app
    image: {{ .image }}`

	// Setup mock resonse from http server
	gock.New("https://raw.githubusercontent.com/okteto/example-template").
		Get("/").
		Reply(200).
		BodyString(tmplCnt)
	// Set up the context and options for running the command
	ctx := context.Background()
	mc := &Command{}
	opts := &InitOpts{
		DevPath:   "",
		Language:  "golang",
		Context:   "sample-ctx",
		Namespace: "sample-ns",
		Workdir:   dir,
		Template:  "https://raw.githubusercontent.com/okteto/example-template",
		TemplateArgs: map[string]string{
			"name":       "test",
			"image":      "okteto/bin",
			"deployname": "test-deploy",
		},
	}

	// Run the command and check for errors
	man, err := mc.RunCreateTemplate(ctx, opts)
	require.NoError(t, err)

	// Validate the contents of the manifest
	dev := man.Dev["test-deploy"]
	require.Equal(t, "sample-ns", man.Namespace)
	require.Equal(t, "sample-ctx", man.Context)
	require.Equal(t, "okteto/bin", dev.Image.Name)
}

func TestRunCreateTemplateWithUrlFailure(t *testing.T) {
	dir := t.TempDir()

	// Define the template content
	tmplCnt := `name: {{ .name }}
namespace: {{ .namespace }}
context: {{ .context }}
dev:
  {{ .deployname }}:
    workdir: /usr/src/app/
    sync:
    - .:/usr/src/app
    image: {{ .image }}`

	// Setup mock resonse from http server
	gock.New("https://raw.githubusercontent.com/okteto/example-template").
		Get("/").
		Reply(404).
		BodyString(tmplCnt)
	// Set up the context and options for running the command
	ctx := context.Background()
	mc := &Command{}
	opts := &InitOpts{
		DevPath:   "",
		Language:  "golang",
		Context:   "sample-ctx",
		Namespace: "sample-ns",
		Workdir:   dir,
		Template:  "https://raw.githubusercontent.com/okteto/example-template",
		TemplateArgs: map[string]string{
			"name":       "test",
			"image":      "okteto/bin",
			"deployname": "test-deploy",
		},
	}

	// Run the command and check for errors
	_, err := mc.RunCreateTemplate(ctx, opts)
	require.Error(t, err)
}
