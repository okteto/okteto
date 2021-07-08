// Copyright 2021 The Okteto Authors
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

package labels

import (
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Get(o metav1.Object, key string) string {
	labels := o.GetLabels()
	if labels != nil {
		return labels[key]
	}
	return ""
}

func Set(o metav1.Object, key, value string) {
	labels := o.GetLabels()
	if labels == nil {
		labels = map[string]string{}
	}
	labels[key] = value
	o.SetLabels(labels)
}

// TransformLabelsToSelector transforms a map of labels into a string k8s selector
func TransformLabelsToSelector(labels map[string]string) string {
	labelList := make([]string, 0)
	for key, value := range labels {
		labelList = append(labelList, fmt.Sprintf("%s=%s", key, value))
	}
	return strings.Join(labelList, ",")
}
