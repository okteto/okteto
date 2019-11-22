package deployments

import (
	"testing"

	okLabels "github.com/okteto/okteto/pkg/k8s/labels"
	"github.com/okteto/okteto/pkg/model"
)

func Test_set_translation_as_annotation_and_back(t *testing.T) {
	manifest := []byte(`name: web
container: dev
image: web:latest
command: ["./run_web.sh"]
workdir: /app
annotations:
  key1: value1
  key2: value2`)
	dev, err := model.Read(manifest)
	if err != nil {
		t.Fatal(err)
	}
	d := dev.GevSandbox()
	dev.DevPath = "okteto.yml"
	translations, err := GetTranslations(dev, d, nil)
	if err != nil {
		t.Fatal(err)
	}
	tr1 := translations[d.Name]
	if err := setTranslationAsAnnotation(d.GetObjectMeta(), tr1); err != nil {
		t.Fatal(err)
	}
	translationString := d.GetObjectMeta().GetAnnotations()[okLabels.TranslationAnnotation]
	if translationString == "" {
		t.Fatal("Marshalled translation was not found in the deployment's annotations")
	}
	tr2, err := getTranslationFromAnnotation(d.GetObjectMeta().GetAnnotations())
	if err != nil {
		t.Fatal(err)
	}
	if tr1.Name != tr2.Name {
		t.Fatal("Mismatching Name value between original and unmarshalled translation")
	}
	if tr1.Version != tr2.Version {
		t.Fatal("Mismatching Version value between original and unmarshalled translation")
	}
	if tr1.Interactive != tr2.Interactive {
		t.Fatal("Mismatching Interactive flag between original and unmarshalled translation")
	}
	if tr1.Replicas != tr2.Replicas {
		t.Fatal("Mismatching Replicas count between original and unmarshalled translation")
	}
}
