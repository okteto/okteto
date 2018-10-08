package ksync

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"

	"github.com/vapor-ware/ksync/pkg/cli"
	"github.com/vapor-ware/ksync/pkg/debug"
	"github.com/vapor-ware/ksync/pkg/syncthing"
)

var (
	duplicateListenerError = "Something is running on 8384 (run 'lsof -i :8384' to find out). Please stop that process before continuing."
)

// SignalLoss is a channel for communicating when contact with a cluster has been lost
var SignalLoss = make(chan bool)

// Syncthing represents the local syncthing process.
type Syncthing struct {
	cmd *exec.Cmd
}

// NewSyncthing constructs a new Syncthing.
func NewSyncthing() *Syncthing {
	return &Syncthing{}
}

func (s *Syncthing) String() string {
	return debug.YamlString(s)
}

// Fields returns a set of structured fields for logging.
func (s *Syncthing) Fields() log.Fields {
	return debug.StructFields(s)
}

// Propogate stdout/stderr into the ksync logs for debugging.
func (s *Syncthing) outputHandler() error {
	logger := log.WithFields(log.Fields{
		"name": "syncthing",
	})

	stdout, err := s.cmd.StdoutPipe()
	if err != nil {
		return err
	}

	outScanner := bufio.NewScanner(stdout)

	stderr, err := s.cmd.StderrPipe()
	if err != nil {
		return err
	}

	errScanner := bufio.NewScanner(stderr)

	go func() {
		for outScanner.Scan() {
			text := outScanner.Text()
			if strings.Contains(text, "address already in use") {
				log.Fatalf(duplicateListenerError)
			}

			logger.Debug(text)
		}
	}()

	go func() {
		for errScanner.Scan() {
			logger.Warn(errScanner.Text())
		}
	}()

	return nil
}

func (s *Syncthing) binPath() string {
	// TODO: Clean this up to be a little more elegant
	path := filepath.Join(cli.ConfigPath(), "bin", "syncthing")

	switch runtime.GOOS {
	case "windows":
		return fmt.Sprintf("%s.%s", path, "exe")
	default:
		return path
	}
}

// HasBinary checks whether the syncthing binary exists in the correct location
// or not.
func (s *Syncthing) HasBinary() bool {
	if _, err := os.Stat(s.binPath()); err != nil {
		return false
	}

	return true
}

// Fetch the latest syncthing binary to Syncthing.binPath().
func (s *Syncthing) Fetch() error {
	return syncthing.Fetch(s.binPath())
} // nolint: gosec

// To make sure no odd devices or folders are being synced after edge cases
// (such as the process being kill'd), the config and db are blown away before
// each run.
func (s *Syncthing) resetState() error {
	base := filepath.Join(cli.ConfigPath(), "syncthing")
	if err := os.RemoveAll(base); err != nil {
		return err
	}

	return syncthing.ResetConfig(filepath.Join(base, "config.xml"))
}

// Normally, syscall.Kill would be good enough. Unfortunately, that's not
// supported in windows. While this isn't tested on windows it at least gets
// past the compiler.
func (s *Syncthing) cleanupDaemon(pidPath string) error {
	// Deal with Windows conditions by bailing
	if runtime.GOOS == "windows" {
		return nil
	}

	if _, err := os.Stat(pidPath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}

		return err
	}

	content, err := ioutil.ReadFile(pidPath) // nolint: gosec
	if err != nil {
		return err
	}

	log.Debug("cleaning background daemon")

	pid, pidErr := strconv.Atoi(string(content))
	if pidErr != nil {
		return pidErr
	}

	proc := os.Process{Pid: pid}

	if err := proc.Signal(os.Interrupt); err != nil {
		if strings.Contains(err.Error(), "process already finished") {
			return nil
		}

		return err
	}

	defer proc.Wait() // nolint: errcheck

	return nil
}

// Run starts up a local syncthing process to serve files from.
func (s *Syncthing) Run() error {
	pidPath := filepath.Join(cli.ConfigPath(), "syncthing.pid")
	if !s.HasBinary() {
		return fmt.Errorf("missing pre-requisites, run init to fix")
	}

	if err := s.resetState(); err != nil {
		return err
	}

	if err := s.cleanupDaemon(pidPath); err != nil {
		return err
	}

	path := filepath.Join(cli.ConfigPath(), "bin", "syncthing")

	address := fmt.Sprintf("localhost:%d", viper.GetInt("syncthing-port"))

	cmdArgs := []string{
		"-gui-address", address,
		"-gui-apikey", viper.GetString("apikey"),
		"-home", filepath.Join(cli.ConfigPath(), "syncthing"),
		"-no-browser",
	}

	s.cmd = exec.Command(path, cmdArgs...) //nolint: gas, gosec

	// These need to change by platform.
	s.cmd.SysProcAttr = syncthingProcAttr

	if err := s.outputHandler(); err != nil {
		return err
	}

	if err := s.cmd.Start(); err != nil {
		return err
	}

	// Because child process signal handling is completely broken, just save the
	// pid and try to kill it every start.
	if err := ioutil.WriteFile(
		pidPath,
		[]byte(strconv.Itoa(s.cmd.Process.Pid)),
		0600); err != nil {
		return err
	}

	// This is horrific, but spin off a process to check for signals about LOS
	// (Loss Of Signal) from the cluster. Cleanup and bail if we get one.
	go func() {
		for { // nolint: megacheck
			select {
			case <-SignalLoss:
				log.WithFields(s.Fields()).Info("signal loss dectected. shutting down")
				s.Stop() // nolint: errcheck, gosec
				os.Exit(1)
				return
			}
		}
	}()

	log.WithFields(log.Fields{
		"cmd":  s.cmd.Path,
		"args": s.cmd.Args,
	}).Debug("starting syncthing")

	return nil
}

// Stop halts the background process and cleans up.
func (s *Syncthing) Stop() error {
	defer s.cmd.Process.Wait() // nolint: errcheck

	return s.cmd.Process.Signal(os.Interrupt)
}
