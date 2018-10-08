package model

import (
	"testing"
)

func TestReadDev(t *testing.T) {
	dev := Dev{
		Name: "test",
		Mount: mount{
			Source: ".",
			Target: "/go/src/github.com/okteto/cnd",
		},
	}

	dev.fixPath("/go/src/github.com/okteto/cnd/cnd.yml")
	if dev.Mount.Source != "/go/src/github.com/okteto/cnd" {
		t.Errorf(dev.Mount.Source)
	}

}
