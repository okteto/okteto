package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sync"
	"time"

	"github.com/okteto/app/cli/pkg/config"
	"github.com/okteto/app/cli/pkg/errors"
	k8Client "github.com/okteto/app/cli/pkg/k8s/client"
	"github.com/okteto/app/cli/pkg/k8s/pods"
	"github.com/okteto/app/cli/pkg/log"
	"github.com/okteto/app/cli/pkg/model"
	"github.com/okteto/app/cli/pkg/okteto"

	"github.com/okteto/app/cli/pkg/k8s/forward"
	"github.com/okteto/app/cli/pkg/syncthing"

	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// ReconnectingMessage is the messaged show when we are trying to reconnect
const ReconnectingMessage = "Trying to reconnect to your cluster. File synchronization will automatically resume when the connection improves."

// UpContext is the common context of all operations performed during
// the up command
type UpContext struct {
	Context    context.Context
	Cancel     context.CancelFunc
	DevPath    string
	WG         *sync.WaitGroup
	Dev        *model.Dev
	Client     *kubernetes.Clientset
	RestConfig *rest.Config
	Pod        string
	Forwarder  *forward.PortForwardManager
	Disconnect chan struct{}
	Running    chan error
	Exit       chan error
	Sy         *syncthing.Syncthing
	ErrChan    chan error
	progress   *progress
}

//Up starts a cloud dev environment
func Up() *cobra.Command {
	var devPath string
	cmd := &cobra.Command{
		Use:   "up",
		Short: "Activates your Okteto Environment",
		RunE: func(cmd *cobra.Command, args []string) error {
			log.Debug("starting up command")

			if !syncthing.IsInstalled() {
				fmt.Println("Installing dependencies...")
				if err := downloadSyncthing(); err != nil {
					return fmt.Errorf("couldn't download syncthing, please try again")
				}
			}

			devPath = getFullPath(devPath)

			if _, err := os.Stat(devPath); os.IsNotExist(err) {
				if err := createManifest(devPath); err != nil {
					return fmt.Errorf("couldn't create your manifest: %s", err)
				}
			}

			dev, err := model.Get(devPath)
			if err != nil {
				return err
			}
			return RunUp(dev, devPath)
		},
	}

	cmd.Flags().StringVarP(&devPath, "file", "f", config.ManifestFileName(), "path to the manifest file")
	return cmd
}

//RunUp starts the up sequence
func RunUp(dev *model.Dev, devPath string) error {
	up := &UpContext{
		WG:         &sync.WaitGroup{},
		Dev:        dev,
		DevPath:    filepath.Base(devPath),
		Disconnect: make(chan struct{}, 1),
		Running:    make(chan error, 1),
		Exit:       make(chan error, 1),
		ErrChan:    make(chan error, 1),
	}

	up.progress = newProgressBar("Activating your Okteto Environment...")
	up.progress.start()

	defer up.Shutdown()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	go up.Activate(devPath)
	select {
	case <-stop:
		log.Debugf("CTRL+C received, starting shutdown sequence")
		fmt.Println()
	case err := <-up.Exit:
		if err == nil {
			log.Debugf("finished channel received, starting shutdown sequence")
		} else {
			return err
		}
	}
	return nil
}

// Activate activates the dev environment
func (up *UpContext) Activate(devPath string) {
	up.WG.Add(1)
	defer up.WG.Done()
	var prevError error
	for {
		up.Context, up.Cancel = context.WithCancel(context.Background())
		err := up.Execute(prevError != nil)
		up.progress.stop()
		if err != nil {
			up.Exit <- err
			return
		}

		switch prevError {
		case errors.ErrLostConnection:
			log.Green("Reconnected to your cluster.")
		}

		log.Success("Your Okteto Environment is ready")
		fmt.Println()

		args := []string{"exec", "--pod", up.Pod, "--"}
		args = append(args, up.Dev.Command...)
		cmd := exec.Command(config.GetBinaryFullPath(), args...)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Start(); err != nil {
			log.Infof("Failed to execute okteto exec: %s", err)
			up.Exit <- err
			return
		}

		go func() {
			up.WG.Add(1)
			defer up.WG.Done()
			up.Running <- cmd.Wait()
			return
		}()

		prevError = up.WaitUntilExitOrInterrupt(cmd)
		if prevError != nil && prevError == errors.ErrLostConnection {
			log.Yellow("Connection lost to your Okteto Environment, reconnecting...")
			fmt.Println()
		}
		if prevError != nil && prevError == errors.ErrCommandFailed && !up.Sy.IsConnected() {
			log.Yellow("Connection lost to your Okteto Environment, reconnecting...")
			up.Shutdown()
			continue

		}

		up.Exit <- nil
		return
	}
}

// WaitUntilExitOrInterrupt blocks execution until a stop signal is sent or a disconnect event or an error
func (up *UpContext) WaitUntilExitOrInterrupt(cmd *exec.Cmd) error {
	for {
		select {
		case <-up.Context.Done():
			if err := cmd.Process.Signal(os.Interrupt); err != nil {
				log.Infof("Failed to kill process: %s", err)
			}
			return nil

		case err := <-up.Running:
			if err != nil {
				log.Infof("Command execution error: %s\n", err)
				return errors.ErrCommandFailed
			}
			return nil

		case err := <-up.ErrChan:
			log.Yellow(err.Error())
		case <-up.Disconnect:
			if err := cmd.Process.Signal(os.Interrupt); err != nil {
				log.Infof("Failed to kill process: %s", err)
			}
			return errors.ErrLostConnection
		}
	}
}

// Execute runs all the logic for the up command
func (up *UpContext) Execute(isRetry bool) error {
	var err error
	var namespace string
	up.Client, up.RestConfig, namespace, err = k8Client.Get()
	if err != nil {
		return err
	}

	up.Sy, err = syncthing.New(up.Dev, up.DevPath, namespace)
	if err != nil {
		return err
	}

	if err := okteto.DevModeOn(up.Dev, up.DevPath); err != nil {
		return err
	}

	up.Pod, err = pods.GetDevPod(up.Context, up.Dev, namespace, up.Client)
	if err != nil {
		return err
	}

	if err := up.Sy.Run(up.Context, up.WG); err != nil {
		return err
	}

	up.Forwarder = forward.NewPortForwardManager(up.Context, up.RestConfig, up.Client, up.ErrChan)
	if err := up.Forwarder.Add(up.Sy.RemotePort, syncthing.ClusterPort); err != nil {
		return err
	}
	if err := up.Forwarder.Add(up.Sy.RemoteGUIPort, syncthing.GUIPort); err != nil {
		return err
	}

	up.Forwarder.Start(up.Pod, namespace)
	go up.Sy.Monitor(up.Context, up.WG, up.Disconnect)

	if err := up.Sy.WaitForPing(up.Context, up.WG); err != nil {
		return err
	}
	if err := up.Sy.WaitForCompletion(up.Context, up.WG, up.Dev); err != nil {
		return err
	}
	if err := up.Sy.OverrideChanges(up.Context, up.WG, up.Dev); err != nil {
		return err
	}
	if err := up.Sy.WaitForCompletion(up.Context, up.WG, up.Dev); err != nil {
		return err
	}

	up.Sy.Type = "sendreceive"
	if err := up.Sy.UpdateConfig(); err != nil {
		return err
	}
	if err := up.Sy.Restart(up.Context, up.WG); err != nil {
		return err
	}

	return nil
}

// Shutdown runs the cancellation sequence. It will wait for all tasks to finish for up to 500 milliseconds
func (up *UpContext) Shutdown() {
	log.Debugf("cancelling context")
	if up.Cancel != nil {
		up.Cancel()
	}

	log.Debugf("waiting for tasks for be done")
	done := make(chan struct{})
	go func() {
		if up.WG != nil {
			up.WG.Wait()
		}
		close(done)
	}()

	go func() {
		if up.Forwarder != nil {
			up.Forwarder.Stop()
		}

		return
	}()

	select {
	case <-done:
		log.Debugf("completed shutdown sequence")
		return
	case <-time.After(1 * time.Second):
		log.Debugf("tasks didn't finish, terminating")
		return
	}
}
