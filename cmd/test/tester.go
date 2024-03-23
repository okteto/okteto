package test

import (
	"context"
	"fmt"

	"github.com/okteto/okteto/pkg/model"
)

type Tester struct {
	Manifest *model.Manifest
}

func (te *Tester) Run(ctx context.Context) (err error) {
	// TODO: Run the tests
	for testName, test := range te.Manifest.Test {
		fmt.Printf("name=%s command=%s image=%s", testName, test.Command, test.Image)
	}
	return
}
