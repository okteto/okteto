package deployments

import (
	"encoding/json"

	"github.com/okteto/okteto/pkg/model"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func getLabel(o metav1.Object, key string) string {
	labels := o.GetLabels()
	if labels != nil {
		return labels[key]
	}
	return ""
}

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

func getDevListFromAnnotation(o metav1.Object) ([]*model.Dev, error) {
	devList := []*model.Dev{}
	devListAnnotation := getAnnotation(o, oktetoDevAnnotation)
	if devListAnnotation == "" {
		return devList, nil
	}
	if err := json.Unmarshal([]byte(devListAnnotation), &devList); err != nil {
		return nil, err
	}
	return devList, nil
}

func setTranslationAsAnnotation(o metav1.Object, tr *model.Translation) error {
	translationBytes, err := json.Marshal(tr)
	if err != nil {
		return err
	}
	setAnnotation(o, OktetoTranslationAnnotation, string(translationBytes))
	return nil
}
