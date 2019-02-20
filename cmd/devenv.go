package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cloudnativedevelopment/cnd/pkg/config"
	"github.com/cloudnativedevelopment/cnd/pkg/log"
	"github.com/cloudnativedevelopment/cnd/pkg/storage"
)

func getFullPath(p string) string {
	a, _ := filepath.Abs(p)
	return a
}

func findDevEnvironment(mustBeRunning, checkForStale bool) (string, string, string, string, error) {
	services := storage.All()
	candidates := []storage.Service{}
	deploymentFullName := ""
	podName := ""
	folder, _ := os.Getwd()

	for name, svc := range services {
		if strings.HasPrefix(folder, svc.Folder) {
			if mustBeRunning {
				if svc.Syncthing == "" {
					continue
				}
			}

			if checkForStale {
				if storage.RemoveIfStale(&svc, name) {
					log.Debugf("found stale entry for %s", name)
					continue
				}
			}

			candidates = append(candidates, svc)
			if deploymentFullName == "" {
				deploymentFullName = name
			}

			if podName == "" {
				podName = svc.Pod
			}
		}
	}

	if len(candidates) == 0 {
		return "", "", "", "", errNoCNDEnvironment
	}

	if len(candidates) > 1 {
		return "", "", "", "", errMultipleCNDEnvironment
	}

	parts := strings.SplitN(deploymentFullName, "/", 3)
	if len(parts) < 3 {
		return "", "", "", "", fmt.Errorf("unable to parse the cnd local state. Remove '%s' and try again", config.GetCNDHome())
	}
	namespace := parts[0]
	deploymentName := parts[1]
	devContainer := parts[2]

	return namespace, deploymentName, devContainer, podName, nil
}

func getDevEnvironment(devPath string, mustBeRunning bool) (string, string, string, string, error) {
	services := storage.All()
	folder, _ := os.Getwd()
	fullDevPath := getFullPath(devPath)

	for name, svc := range services {
		if strings.HasPrefix(folder, svc.Folder) {
			if mustBeRunning {
				if svc.Syncthing == "" {
					continue
				}
			}

			if svc.Manifest == "" || svc.Manifest == fullDevPath {
				parts := strings.SplitN(name, "/", 3)
				if len(parts) < 3 {
					return "", "", "", "", fmt.Errorf("unable to parse the cnd local state. Remove '%s' and try again", config.GetCNDHome())
				}

				namespace := parts[0]
				deploymentName := parts[1]
				devContainer := parts[2]

				return namespace, deploymentName, devContainer, svc.Pod, nil
			}
		}
	}

	log.Infof("couldn't find any service that matched %s and %s", folder, fullDevPath)
	return "", "", "", "", errNoCNDEnvironment
}
