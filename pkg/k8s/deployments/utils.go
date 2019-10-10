package deployments

import (
	"encoding/json"

	okLabels "github.com/okteto/okteto/pkg/k8s/labels"
	"github.com/okteto/okteto/pkg/model"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func setLabel(o metav1.Object, key, value string) {
	labels := o.GetLabels()
	if labels == nil {
		labels = map[string]string{}
	}
	labels[key] = value
	o.SetLabels(labels)
}

func getAnnotation(o metav1.Object, key string) string {
	annotations := o.GetAnnotations()
	if annotations != nil {
		return annotations[key]
	}
	return ""
}

func setAnnotation(o metav1.Object, key, value string) {
	annotations := o.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}
	annotations[key] = value
	o.SetAnnotations(annotations)
}

func setTranslationAsAnnotation(o metav1.Object, tr *model.Translation) error {
	translationBytes, err := json.Marshal(tr)
	if err != nil {
		return err
	}
	setAnnotation(o, okLabels.TranslationAnnotation, string(translationBytes))
	return nil
}
