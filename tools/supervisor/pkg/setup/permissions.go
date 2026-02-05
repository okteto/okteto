package setup

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
)

func addPermissions(configPath string) error {
	// Define the files to set permissions
	files := []string{
		filepath.Join(configPath, "cert.pem"),
		filepath.Join(configPath, "config.xml"),
		filepath.Join(configPath, "key.pem"),
	}

	// Set the permissions to 644
	for _, file := range files {
		log.Default().Printf("setting permissions to 644 for %s", file)
		err := os.Chmod(file, 0644)
		if err != nil {
			return fmt.Errorf("failed to change permissions for %s: %w", file, err)
		}
		log.Default().Printf("permissions set to 644 for %s", file)
	}
	return nil
}
