package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/cloudnativedevelopment/cnd/pkg/log"
	"github.com/cloudnativedevelopment/cnd/pkg/storage"
	"github.com/cloudnativedevelopment/cnd/pkg/syncthing"
	"github.com/spf13/cobra"
	yaml "gopkg.in/yaml.v2"
)

var (
	restClient *http.Client
)

type listOutput struct {
	Environments []listOutputEnvironment `yaml:"environments,omitempty"`
}

type listOutputEnvironment struct {
	Name       string   `yaml:"name,omitempty"`
	Source     string   `yaml:"source,omitempty"`
	Completion string   `yaml:"completion,omitempty"`
	Errors     []string `yaml:"errors,omitempty"`
}

type event struct {
	Type string `json:"type,omitempty"`
	Data struct {
		Completion float64 `json:"completion,omitempty"`
	} `json:"data,omitempty"`
}

type syncthingErrors struct {
	Errors []struct {
		Message string `json:"message,omitempty"`
	} `json:"errors,omitempty"`
}

//List implements the list logic
func List() *cobra.Command {
	var yamlOutput bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List your active cloud native development environments",
		RunE: func(cmd *cobra.Command, args []string) error {
			return list(yamlOutput)
		},
	}
	cmd.Flags().BoolVarP(&yamlOutput, "yaml", "y", false, "yaml output")
	return cmd
}

func list(yamlOutput bool) error {
	services := storage.All()

	output := listOutput{
		Environments: []listOutputEnvironment{},
	}

	client := syncthing.NewAPIClient()

	for name, svc := range services {
		if storage.RemoveIfStale(&svc, name) {
			log.Debugf("found stale entry for %s", name)
			continue
		}

		env := listOutputEnvironment{
			Name:   name,
			Source: svc.Folder,
		}

		sy := &syncthing.Syncthing{
			GUIAddress: svc.Syncthing,
			Client:     client,
		}

		if sy.GUIAddress == "" {
			continue
		}

		completion, err := getStatus(sy, svc)
		if err == nil {
			env.Completion = fmt.Sprintf("%2.f%%", completion)
		} else {
			log.Infof("Failed to get status of %s: %s", svc.Folder, err)
			env.Completion = "?"
		}

		apiErrors, err := getErrors(sy, svc)
		if err != nil {
			log.Infof("Failed to get errors of %s: %s", svc.Folder, err)
			continue
		}

		env.Errors = apiErrors
		output.Environments = append(output.Environments, env)
	}

	if yamlOutput {
		return printYAML(&output)
	}

	return printDirectly(&output)
}

func getStatus(sy *syncthing.Syncthing, s storage.Service) (float64, error) {
	body, err := sy.GetFromAPI("rest/events")
	if err != nil {
		return 0, fmt.Errorf("error getting syncthing state: %s", err)
	}

	var events []event
	if err := json.Unmarshal(body, &events); err != nil {
		return 0, fmt.Errorf("error unmarshalling syncthing state: %s", err)
	}

	for i := len(events) - 1; i >= 0; i-- {
		e := events[i]
		if e.Type == "FolderCompletion" {
			return e.Data.Completion, nil
		}
	}

	return 0, nil
}

func getErrors(sy *syncthing.Syncthing, s storage.Service) ([]string, error) {
	body, err := sy.GetFromAPI("rest/system/error")
	if err != nil {
		return nil, fmt.Errorf("error getting syncthing errors: %s", err)
	}

	var errors syncthingErrors
	if err := json.Unmarshal(body, &errors); err != nil {
		return nil, fmt.Errorf("error getting syncthing errors: %s", err)
	}

	parsedErrors := make([]string, len(errors.Errors))
	for i := range parsedErrors {
		sp := strings.Split(errors.Errors[i].Message, ":")
		if len(sp) == 0 {
			parsedErrors[i] = sp[0]
			log.Infof("error with unexpected format: %s", errors.Errors)
		} else {
			parsedErrors[i] = strings.TrimSpace(sp[1])
		}

	}

	return parsedErrors, nil
}

func printDirectly(o *listOutput) error {
	if len(o.Environments) == 0 {
		fmt.Println("There are no active cloud native development environments")
		return nil
	}

	fmt.Println("Active cloud native development environments:")
	for _, e := range o.Environments {
		var buff bytes.Buffer
		buff.WriteString(fmt.Sprintf("\t%s\n", e.Name))
		buff.WriteString(fmt.Sprintf("\t\t%-15s%s\n", "source:", e.Source))
		buff.WriteString(fmt.Sprintf("\t\t%-15s%s\n", "completion:", e.Completion))

		for _, er := range e.Errors {
			buff.WriteString(fmt.Sprintf("\t\t%-15s%s\n", "error:", er))
		}

		fmt.Println(buff.String())
	}

	return nil
}

func printYAML(o *listOutput) error {
	out, err := yaml.Marshal(o)
	if err != nil {
		return err
	}

	_, err = fmt.Println(string(out))
	return err
}
