package okteto

import (
	"io/ioutil"
	"os"
)

// SetToken is used in test to setup token params, returns tempDir and error
func SetToken(t *Token) (string, error) {
	currentToken = nil
	dir, err := ioutil.TempDir("", "")
	if err != nil {
		return "", err
	}

	os.Setenv("OKTETO_FOLDER", dir)

	if t != nil {
		if err := save(t); err != nil {
			return "", err
		}
	}

	return dir, nil

}
