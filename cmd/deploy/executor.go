package deploy

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"

	"github.com/okteto/okteto/pkg/log"
)

type executor struct{}

func newExecutor() *executor {
	return &executor{}
}

func (e *executor) Execute(command string, env []string) error {
	log.Information("Running '%s'...", command)

	cmd := exec.Command("bash", "-c", command)
	cmd.Env = append(os.Environ(), env...)

	r, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}

	cmd.Stderr = cmd.Stdout
	done := make(chan struct{})
	scanner := bufio.NewScanner(r)

	go func() {
		for scanner.Scan() {
			line := scanner.Text()
			fmt.Println(line)
		}
		done <- struct{}{}
	}()

	if err := cmd.Start(); err != nil {
		return err
	}

	if err := cmd.Wait(); err != nil {
		return err
	}

	return nil
}
