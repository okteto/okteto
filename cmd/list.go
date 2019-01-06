package cmd

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"path"

	"github.com/okteto/cnd/pkg/storage"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

//Event struct
type Event struct {
	Type string `json:"type,omitempty"`
	Data Data   `json:"data,omitempty"`
}

//Data event data
type Data struct {
	Completion float64 `json:"completion,omitempty"`
}

//List implements the list logic
func List() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List your active cloud native development environments",
		RunE: func(cmd *cobra.Command, args []string) error {
			return list()
		},
	}
	return cmd
}

func list() error {

	services := storage.All()

	if len(services) == 0 {
		fmt.Println("There are no active cloud native development environments")
		return nil
	}
	fmt.Println("Active cloud native development environments:")
	for name, svc := range services {
		completion, err := getStatus(svc)
		if err == nil {
			fmt.Printf("%s\t\t%s\t\t%.2f%%\n", name, svc.Folder, completion)
		} else {
			log.Infof("Failed to get status of %s: %s", svc.Folder, err)
			fmt.Printf("%s\t\t%s\t\t%.2f%%\n", name, svc.Folder, -1.0)
		}
	}

	return nil
}

func getStatus(s storage.Service) (float64, error) {
	client := &http.Client{}
	urlPath := path.Join(s.Syncthing, "rest", "events")
	req, err := http.NewRequest("GET", fmt.Sprintf("http://%s", urlPath), nil)
	if err != nil {
		return 100, err
	}

	// add query parameters
	q := req.URL.Query()
	q.Add("limit", "30")
	req.URL.RawQuery = q.Encode()
	// add auth header
	req.Header.Add("X-API-Key", "cnd")
	resp, err := client.Do(req)
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
