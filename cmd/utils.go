package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/okteto/okteto/pkg/log"
)

func getFullPath(p string) string {
	a, _ := filepath.Abs(p)
	return a
}

func askYesNo(q string) bool {
	var answer string
	for {
		fmt.Printf(q)
		fmt.Scanln(&answer)
		if answer == "y" || answer == "n" {
			break
		}

		log.Fail("input must be 'y' or 'n'")
	}

	if answer == "n" {
		return false
	}

	return true
}
