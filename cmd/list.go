package cmd

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"path"

	"github.com/okteto/cnd/pkg/storage"
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
		Short: "lists your active cloud native development environments",
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
		completion := status(svc)
		fmt.Printf("%s\t\t%s\t\t%.2f%%\n", name, svc.Folder, completion)
	}
	return nil
}

func status(s storage.Service) float64 {
	client := &http.Client{}
	urlPath := path.Join(s.Syncthing, "rest", "events")
	req, err := http.NewRequest("GET", fmt.Sprintf("http://%s", urlPath), nil)
	if err != nil {
		fmt.Printf("error getting syncthing client: %s\n", err)
		return 100
	}
	// add query parameters
	q := req.URL.Query()
	q.Add("limit", "30")
	req.URL.RawQuery = q.Encode()
	// add auth header
	req.Header.Add("X-API-Key", "okteto")
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("error getting syncthing state: %s\n", err)
		return 100
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("error reading body: %s\n", err.Error())
		return 100
	}
	if resp.StatusCode != 200 {
		fmt.Printf("error %d getting synchthing status: %s", resp.StatusCode, string(body))
		return 100
	}
	var events []Event
	err = json.Unmarshal(body, &events)
	if err != nil {
		fmt.Printf("error unmarshalling events: %s\n", err.Error())
	}
	for i := len(events) - 1; i >= 0; i-- {
		e := events[i]
		if e.Type == "FolderCompletion" {
			return e.Data.Completion
		}
	}
	return 100
}
