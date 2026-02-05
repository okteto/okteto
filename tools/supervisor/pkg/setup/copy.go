package setup

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
)

func copyFiles(src, dst string) error {
	log.Default().Printf("copying %s to %s", src, dst)

	fs, err := os.Stat(dst)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			if err := os.MkdirAll(dst, 0755); err != nil {
				return fmt.Errorf("failed to create directory: %w", err)
			}
		} else {
			return fmt.Errorf("failed to get file info: %w", err)
		}
	}
	if !fs.IsDir() {
		return fmt.Errorf("source is not a directory: %s", dst)
	}

	files := []string{
		"cert.pem",
		"config.xml",
		"key.pem",
	}
	for _, file := range files {
		src := filepath.Join(src, file)
		dst := filepath.Join(dst, file)
		if _, err := os.Stat(dst); err == nil {
			log.Default().Printf("file already exists: %s", dst)
			continue
		}
		log.Default().Printf("copying %s to %s", src, dst)
		err := runCommand("cp", src, dst)
		if err != nil {
			return fmt.Errorf("failed to copy files: %w", err)
		}
	}
	return nil
}

func runCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	log.Default().Printf("running command: %s", cmd.String())
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to run command: %w", err)
	}
	log.Default().Printf("command executed correctly: %s", cmd.String())
	return nil
}
