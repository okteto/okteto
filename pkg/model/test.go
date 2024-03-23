package model

import "github.com/okteto/okteto/pkg/env"

type Test struct {
	Image   string   `json:"image,omitempty" yaml:"image,omitempty"`
	Command []string `json:"command,omitempty" yaml:"command,omitempty"`
}

func (test *Test) expandEnvVars() error {
	var err error
	if len(test.Image) > 0 {
		test.Image, err = env.ExpandEnv(test.Image)
		if err != nil {
			return err
		}
	}
	if len(test.Command) > 0 {
		for i, v := range test.Command {
			test.Command[i], err = env.ExpandEnv(v)
			if err != nil {
				return err
			}
		}
	}

	return nil
}
