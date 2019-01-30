package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/cloudnativedevelopment/cnd/pkg/log"
	"github.com/cloudnativedevelopment/cnd/pkg/storage"

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

//Event struct
type Event struct {
	Type string `json:"type,omitempty"`
	Data Data   `json:"data,omitempty"`
}

//SyncthingErrors struct
type SyncthingErrors struct {
	Errors []SyncthingError `json:"errors,omitempty"`
}

//SyncthingError struct
type SyncthingError struct {
	Message string `json:"message,omitempty"`
}

//Data event data
type Data struct {
	Completion float64 `json:"completion,omitempty"`
}

type addAPIKeyTransport struct {
	T http.RoundTripper
}

func (akt *addAPIKeyTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Add("X-API-Key", "cnd")
	return akt.T.RoundTrip(req)
}

func init() {
	transport := &addAPIKeyTransport{http.DefaultTransport}
	restClient = &http.Client{
		Timeout:   60 * time.Second,
		Transport: transport,
	}
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

	for name, svc := range services {
		env := listOutputEnvironment{
			Name:   name,
			Source: svc.Folder,
		}

		completion, err := getStatus(svc)
		if err == nil {
			env.Completion = fmt.Sprintf("%2.f%%", completion)
		} else {
			log.Infof("Failed to get status of %s: %s", svc.Folder, err)
			env.Completion = "?"
		}

		apiErrors, err := getErrors(svc)
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

func getStatus(s storage.Service) (float64, error) {
	urlPath := path.Join(s.Syncthing, "rest", "events")
	req, err := http.NewRequest("GET", fmt.Sprintf("http://%s", urlPath), nil)
	if err != nil {
		return 100, err
	}

	// add query parameters
	q := req.URL.Query()
	q.Add("limit", "30")
	req.URL.RawQuery = q.Encode()

	resp, err := restClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("error getting syncthing state: %s", err)
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}
	if resp.StatusCode != 200 {
		return 0, fmt.Errorf("error %d getting synchthing status: %s", resp.StatusCode, string(body))
	}
	var events []Event
	err = json.Unmarshal(body, &events)
	if err != nil {
		return 0, fmt.Errorf("error unmarshalling events: %s", err.Error())
	}

	for i := len(events) - 1; i >= 0; i-- {
		e := events[i]
		if e.Type == "FolderCompletion" {
			return e.Data.Completion, nil
		}
	}

	return 100, nil
}

func getErrors(s storage.Service) ([]string, error) {
	urlPath := path.Join(s.Syncthing, "rest", "system", "error")
	log.Debugf("getting errors via %s", urlPath)
	req, err := http.NewRequest("GET", fmt.Sprintf("http://%s", urlPath), nil)
	if err != nil {
		return nil, err
	}

	// add query parameters
	q := req.URL.Query()
	q.Add("limit", "5")
	req.URL.RawQuery = q.Encode()

	resp, err := restClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error calling the syncthing api: %s", err)
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("error %d getting folder sync errors: %s", resp.StatusCode, string(body))
	}

	var errors SyncthingErrors
	err = json.Unmarshal(body, &errors)
	if err != nil {
		return nil, fmt.Errorf("error unmarshalling folder sync errors: %s", err.Error())
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
