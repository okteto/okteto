package ksync

import (
	"fmt"
	"os/exec"

	"github.com/okteto/cnd/model"
)

//Create creates a sync folder
func Create(d *model.Dev) error {
	cmd := exec.Command(
		"ksync",
		"create",
		"--reload=false",
		fmt.Sprintf("--selector=cnd=%s", d.Name),
		d.Mount.Source,
		d.Mount.Target)
	return cmd.Run()
}
