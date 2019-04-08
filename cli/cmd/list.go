package cmd

import (
	"bytes"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"

	"cli/cnd/pkg/log"
	"cli/cnd/pkg/storage"
	"cli/cnd/pkg/syncthing"

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

	log.Debugf("found %d services", len(services))

	wg := sync.WaitGroup{}
	for name, svc := range services {
		wg.Add(1)
		go func(s storage.Service, n string) {
			defer wg.Done()
			if storage.RemoveIfStale(&s, n) {
				log.Debugf("found stale entry for %s", n)
				return
			}

			env := getServiceStatus(n, &s, client)
			if env != nil {
				output.Environments = append(output.Environments, *env)
			}

			return
		}(svc, name)
	}

	wg.Wait()

	sort.Slice(output.Environments, func(i, j int) bool {
		return output.Environments[i].Name < output.Environments[j].Name
	})

	if yamlOutput {
		return printYAML(&output)
	}

	return printDirectly(&output)
}

func getServiceStatus(name string, svc *storage.Service, client *http.Client) *listOutputEnvironment {

	env := listOutputEnvironment{
		Name:   name,
		Source: svc.Folder,
	}

	sy := &syncthing.Syncthing{
		GUIAddress: svc.Syncthing,
		Client:     client,
	}

	if sy.GUIAddress == "" || sy.GUIAddress == "127.0.0.1:0" {
		log.Debugf("empty syncthing %+v", svc.Folder)
		return nil
	}

	parts := strings.Split(name, "/")
	if len(parts) != 3 {
		log.Debugf("found malformed entry: %s", name)
		return nil
	}

	log.Debugf("getting completion percentage of %s", name)
	completion, err := sy.GetFolderSyncCompletion(parts[1], parts[2])
	if err == nil {
		env.Completion = fmt.Sprintf("%2.f%%", completion)
	} else {
		env.Completion = "?"
		log.Infof("Failed to get status of %s: %s", svc.Folder, err)
	}

	log.Debugf("getting sync errors of %s", name)
	apiErrors, err := sy.GetErrors()
	if err != nil {
		log.Infof("Failed to get errors of %s: %s", svc.Folder, err)
	} else {
		env.Errors = apiErrors
	}

	return &env
}

func printDirectly(o *listOutput) error {
	if len(o.Environments) == 0 {
		fmt.Println("There are no active cloud native development environments")
		return nil
	}

	fmt.Println("Active cloud native development environments:")
	for _, e := range o.Environments {
		var buff bytes.Buffer
		buff.WriteString(fmt.Sprintf("  %s\n", log.BlueString(e.Name)))
		buff.WriteString(fmt.Sprintf("    %s     %s\n", log.BlueString("source:"), e.Source))
		buff.WriteString(fmt.Sprintf("    %s %s\n", log.BlueString("completion:"), e.Completion))

		for _, er := range e.Errors {
			buff.WriteString(fmt.Sprintf("    %s      %s\n", "error:", er))
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
