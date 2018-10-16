package model

import (
	"os"
	"path"

	apiv1 "k8s.io/api/core/v1"
	k8Yaml "k8s.io/apimachinery/pkg/util/yaml"
)

type service struct {
	File string `yaml:"file"`
}

//Service returns a k8 service for a cloud native environment
func (dev *Dev) Service(translate bool) (*apiv1.Service, error) {
	cwd, _ := os.Getwd()
	file, err := os.Open(path.Join(cwd, dev.Swap.Service.File))
	if err != nil {
		return nil, err
	}
	dec := k8Yaml.NewYAMLOrJSONDecoder(file, 1000)
	var s apiv1.Service
	dec.Decode(&s)

	if !translate {
		return &s, nil
	}

	labels := s.GetObjectMeta().GetLabels()
	if labels == nil {
		labels = map[string]string{"cnd": dev.Name}
	} else {
		labels["cnd"] = dev.Name
	}
	s.GetObjectMeta().SetLabels(labels)
	s.Spec.Selector["cnd"] = dev.Name
	return &s, nil
}
