package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"cli/cnd/pkg/config"
	"cli/cnd/pkg/log"
	"cli/cnd/pkg/storage"

	"github.com/manifoldco/promptui"
)

var (
	errNoCNDEnvironment = fmt.Errorf("There aren't any cloud native development environments active in your current folder")
)

type devenv struct {
	Namespace  string
	Deployment string
	Container  string
	Pod        string
	Manifest   string
}

func getFullPath(p string) string {
	a, _ := filepath.Abs(p)
	return a
}

func getDevEnvironments(mustBeRunning, checkForStale bool) ([]devenv, error) {
	services := storage.All()
	devEnvironments := []devenv{}
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

			parts := strings.SplitN(name, "/", 3)
			if len(parts) < 3 {
				return nil, fmt.Errorf("Your state file is malformed. Please remove '%s' and try again", config.GetCNDHome())
			}

			d := devenv{
				Namespace:  parts[0],
				Deployment: parts[1],
				Container:  parts[2],
				Pod:        svc.Pod,
				Manifest:   svc.Manifest,
			}

			devEnvironments = append(devEnvironments, d)
		}
	}

	return devEnvironments, nil
}

func getDevEnvironmentByManifest(devEnvironments []devenv, devPath string) []devenv {
	fullDevPath := getFullPath(devPath)
	result := []devenv{}

	for _, d := range devEnvironments {
		if d.Manifest == "" || d.Manifest == fullDevPath {
			result = append(result, d)
		}
	}

	return result
}

func askUserForDevEnvironment(devEnvironments []devenv, namespace string) (*devenv, error) {
	sort.Slice(devEnvironments, func(i, j int) bool {
		return devEnvironments[i].Namespace == namespace ||
			devEnvironments[i].Deployment > devEnvironments[j].Deployment ||
			devEnvironments[i].Container > devEnvironments[j].Container ||
			devEnvironments[i].Manifest > devEnvironments[j].Manifest
	})

	i, err := runPrompt(devEnvironments)

	if err != nil {
		log.Debugf("invalid deactivate option: %s", err)
		return nil, fmt.Errorf("Invalid option")
	}

	return &devEnvironments[i], nil

}

func runPrompt(devEnvironments []devenv) (int, error) {
	templates := &promptui.SelectTemplates{
		Label:    fmt.Sprintf("%s {{ . }}", log.BlueString("?")),
		Selected: fmt.Sprintf(" ✓  {{ .Namespace | oktetoblue }} {{ .Deployment }}/{{ .Container }}"),
		Active:   fmt.Sprintf("%s {{ .Namespace | oktetoblue }} {{ .Deployment }}/{{ .Container }}", promptui.IconSelect),
		Inactive: "  {{ .Namespace | oktetoblue }} {{ .Deployment }}/{{ .Container }}",
		FuncMap:  promptui.FuncMap,
	}

	templates.FuncMap["oktetoblue"] = log.BlueString

	prompt := promptui.Select{
		Label:     "There are multiple environments active in your current folder, please pick one",
		Items:     devEnvironments,
		Templates: templates,
	}

	i, _, err := prompt.Run()
	return i, err
}
