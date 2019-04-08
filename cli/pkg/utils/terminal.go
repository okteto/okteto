package utils

import (
	"fmt"

	"cli/cnd/pkg/log"
)

func AskYesNo(q string) bool {
	var answer string
	for {
		fmt.Printf(q)
		fmt.Scanln(&answer)
		if answer == "y" || answer == "n" {
			break
		}

		fmt.Println(log.RedString("input must be y or n"))
	}

	if answer == "n" {
		return false
	}

	return true
}
