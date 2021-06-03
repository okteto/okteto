package annotations

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Get(o metav1.Object, key string) string {
	annotations := o.GetAnnotations()
	if annotations != nil {
		return annotations[key]
	}
	return ""
}

func Set(o metav1.Object, key, value string) {
	annotations := o.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}
	annotations[key] = value
	o.SetAnnotations(annotations)
}
