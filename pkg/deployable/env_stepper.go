package deployable

import (
	"fmt"
	"sync"

	"github.com/compose-spec/godotenv"
	"github.com/spf13/afero"
)

// envStepper is an environment reader utility that reads the environment from
// a temporary .env file and keeps a reference to it. It is meant to be called
// to read environment variables in a command list loop
type envStepper struct {
	sync.RWMutex

	filename string
	env      map[string]string

	fs afero.Fs
}

func (es *envStepper) WithFS(fs afero.Fs) {
	es.fs = fs
}
func (es *envStepper) Step() ([]string, error) {
	data, err := afero.ReadFile(es.fs, es.filename)
	if err != nil {
		return nil, err
	}

	current, err := godotenv.Unmarshal(string(data))
	if err != nil {
		return nil, err
	}

	list := mapToEnvList(current)

	es.Lock()
	es.env = current
	es.Unlock()

	return list, nil
}

func (es *envStepper) Map() (env map[string]string) {
	es.RLock()
	env = es.env
	es.RUnlock()
	return
}

func NewEnvStepper(filename string) *envStepper {
	return &envStepper{
		RWMutex:  sync.RWMutex{},
		filename: filename,
		env:      make(map[string]string),
		fs:       afero.NewOsFs(),
	}
}

func mapToEnvList(env map[string]string) []string {
	list := make([]string, 0, len(env))
	for k, v := range env {
		list = append(list, fmt.Sprintf("%s=%s", k, v))
	}
	return list
}
