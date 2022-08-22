package prompt

import (
	"fmt"

	oktetoLog "github.com/okteto/okteto/pkg/log"
)

// AskYesNo prompts for yes/no confirmation and returns boolean with answer
func AskYesNo(q string) (bool, error) {
	var answer string
	for {
		oktetoLog.Question(q)
		if _, err := fmt.Scanln(&answer); err != nil {
			return false, err
		}
		if answer == "y" || answer == "n" {
			break
		}
		oktetoLog.Fail("input must be 'y' for yes or 'n' for no")
	}
	return answer == "y", nil
}

// AsksQuestion asks a question to the user and returns string with answer
func AsksQuestion(q string) (string, error) {
	var answer string

	oktetoLog.Question(q)
	if _, err := fmt.Scanln(&answer); err != nil {
		return "", err
	}

	return answer, nil
}
