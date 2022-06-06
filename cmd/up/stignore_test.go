package up

import (
	"crypto/md5"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/model"
	"github.com/stretchr/testify/assert"
)

func Test_addStignoreSecrets(t *testing.T) {

	localPath := t.TempDir()

	tests := []struct {
		name                               string
		dev                                *model.Dev
		stignoreContent                    string
		expectedTransformedStignoreContent string
		expectedError                      bool
		expectedAnnotation                 model.Annotations
	}{
		{
			name: "test",
			dev: &model.Dev{
				Name:      "test-name",
				Namespace: "test-namespace",
				Sync: model.Sync{
					Folders: []model.SyncFolder{
						{
							LocalPath:  localPath,
							RemotePath: "",
						},
					},
				},
				Metadata: &model.Metadata{
					Annotations: model.Annotations{},
				},
			},
			stignoreContent: `.ignore
#include file
(?d) folder
!exclude`,
			expectedTransformedStignoreContent: `(?d).ignore
`,
			expectedAnnotation: model.Annotations{
				model.OktetoStignoreAnnotation: fmt.Sprintf("%x", md5.Sum([]byte(`
.ignore`))),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			stignorePath := filepath.Join(localPath, ".stignore")
			if err := os.WriteFile(stignorePath, []byte(tt.stignoreContent), 0644); err != nil {
				t.Fatal(err)
			}

			err := addStignoreSecrets(tt.dev)
			if err == nil && tt.expectedError {
				t.Fatal("expected Error, but no error")
			}
			if err != nil && !tt.expectedError {
				t.Fatal(err)
			}
			assert.Equal(t, tt.expectedAnnotation, tt.dev.Metadata.Annotations)

			transformedStignorePath := filepath.Join(config.GetAppHome(tt.dev.Namespace, tt.dev.Name), ".stignore-1")
			file, err := os.ReadFile(transformedStignorePath)
			if err != nil {
				t.Fatal(err)
			}
			if string(file) != tt.expectedTransformedStignoreContent {
				t.Fatalf("expectedTransformedStignoreContent: %s, but got %s", tt.expectedTransformedStignoreContent, string(file))
			}

		})
	}
}
