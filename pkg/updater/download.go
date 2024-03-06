package updater

import (
	"fmt"
	"github.com/okteto/okteto/pkg/config"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
)

func DownloadBinary(url, targetPath string) {
	// Create the file
	out, err := os.Create(targetPath)
	if err != nil {
		fmt.Println("Failed to create file:", err)
		return
	}
	defer out.Close()

	// Get the data
	resp, err := http.Get(url)
	if err != nil {
		fmt.Println("Failed to download:", err)
		return
	}
	defer resp.Body.Close()

	// Write the body to file
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		fmt.Println("Failed to write file:", err)
		return
	}

	// Make the binary executable
	os.Chmod(targetPath, 0755)
}

func CheckAndUpdateVersion() {
	envVersion := os.Getenv("OKTETO_CLUSTER_VERSION")
	if envVersion != "" && envVersion != config.VersionString {
		targetPath := filepath.Join(os.Getenv("HOME"), ".okteto/bin", envVersion, "okteto")

		// Check if the binary exists
		if _, err := os.Stat(targetPath); os.IsNotExist(err) {
			// Download if not exists
			downloadURL := "https://get.okteto.com/" + envVersion + "/okteto"
			DownloadBinary(downloadURL, targetPath)
		}

		// Delegate the command execution to the downloaded binary
		os.Args[0] = targetPath
		cmd := exec.Command(targetPath, os.Args[1:]...)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err := cmd.Run()
		if err != nil {
			// Handle error
			fmt.Println("Error executing command:", err)
			os.Exit(1)
		}
		os.Exit(0)
	}
}
