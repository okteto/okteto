package cmd

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"sync"
	"time"

	"github.com/okteto/app/cli/pkg/analytics"
	"github.com/okteto/app/cli/pkg/config"
	"github.com/okteto/app/cli/pkg/errors"
	k8Client "github.com/okteto/app/cli/pkg/k8s/client"
	"github.com/okteto/app/cli/pkg/k8s/deployments"
	"github.com/okteto/app/cli/pkg/k8s/pods"
	"github.com/okteto/app/cli/pkg/k8s/secrets"
	"github.com/okteto/app/cli/pkg/k8s/services"
	"github.com/okteto/app/cli/pkg/k8s/volumes"
	"github.com/okteto/app/cli/pkg/log"
	"github.com/okteto/app/cli/pkg/model"

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
	WG         *sync.WaitGroup
	Dev        *model.Dev
	Client     *kubernetes.Clientset
	RestConfig *rest.Config
	Pod        string
	Container  string
	Forwarder  *forward.PortForwardManager
	Disconnect chan struct{}
	Running    chan error
	Exit       chan error
	Sy         *syncthing.Syncthing
	ErrChan    chan error
}

//Up starts a cloud dev environment
func Up() *cobra.Command {
	var devPath string
	var namespace string
	cmd := &cobra.Command{
		Use:   "up",
		Short: "Activates your Okteto Environment",
		RunE: func(cmd *cobra.Command, args []string) error {
			log.Debug("starting up command")
			u := upgradeAvailable()
			if len(u) > 0 {
				log.Yellow("Okteto %s is available. To upgrade:", u)
				log.Yellow("    %s", getUpgradeCommand())
				fmt.Println()
			}

			if !syncthing.IsInstalled() {
				fmt.Println("Installing dependencies...")
				if err := downloadSyncthing(); err != nil {
					return fmt.Errorf("couldn't download syncthing, please try again")
				}
			}

			if _, err := os.Stat(devPath); os.IsNotExist(err) {
				return fmt.Errorf("'%s' does not exist. Generate it by executing 'okteto create'", devPath)
			}

			dev, err := model.Get(devPath)
			if err != nil {
				return err
			}
			if namespace != "" {
				dev.Namespace = namespace
			}
			analytics.TrackUp(dev.Image, VersionString)
			return RunUp(dev)
		},
	}

	cmd.Flags().StringVarP(&devPath, "file", "f", config.ManifestFileName(), "path to the manifest file")
	cmd.Flags().StringVarP(&namespace, "namespace", "n", "", "namespace where the up command is executed")
	return cmd
}

//RunUp starts the up sequence
func RunUp(dev *model.Dev) error {
	up := &UpContext{
		WG:         &sync.WaitGroup{},
		Dev:        dev,
		Disconnect: make(chan struct{}, 1),
		Running:    make(chan error, 1),
		Exit:       make(chan error, 1),
		ErrChan:    make(chan error, 1),
	}

	defer up.shutdown()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	go up.Activate()
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
func (up *UpContext) Activate() {
	up.WG.Add(1)
	defer up.WG.Done()
	var prevError error
	attach := false

	for {
		up.Context, up.Cancel = context.WithCancel(context.Background())
		err := up.devMode(attach)
		if err != nil {
			up.Exit <- err
			return
		}
		attach = true

		fmt.Println(" ✓  Okteto Environment activated")

		progress := newProgressBar("Synchronizing your files...")
		progress.start()
		err = up.startSync()
		progress.stop()
		if err != nil {
			up.Exit <- err
			return
		}

		fmt.Println(" ✓  Files synchronized")

		progress = newProgressBar("Finalizing configuration...")
		progress.start()
		err = up.forceLocalSyncState()
		progress.stop()
		if err != nil {
			up.Exit <- err
			return
		}

		switch prevError {
		case errors.ErrLostConnection:
			log.Green("Reconnected to your cluster.")
		}

		printDisplayContext("Your Okteto Environment is ready", up.Dev.Namespace, up.Dev.Name, up.Dev.Forward)
		cmd, port := up.buildExecCommand()
		if err := cmd.Start(); err != nil {
			log.Infof("Failed to execute okteto exec: %s", err)
			up.Exit <- err
			return
		}

		log.Debugf("started new okteto exec")

		go func() {
			up.WG.Add(1)
			defer up.WG.Done()
			up.Running <- cmd.Wait()
			return
		}()

		execEndpoint := fmt.Sprintf("http://127.0.0.1:%d", port)
		prevError = up.WaitUntilExitOrInterrupt(execEndpoint)
		if prevError != nil && (prevError == errors.ErrLostConnection ||
			prevError == errors.ErrCommandFailed && !up.Sy.IsConnected()) {
			log.Yellow("\nConnection lost to your Okteto Environment, reconnecting...")
			fmt.Println()
			up.shutdown()
			continue
		}

		up.Exit <- nil
		return
	}
}

// WaitUntilExitOrInterrupt blocks execution until a stop signal is sent or a disconnect event or an error
func (up *UpContext) WaitUntilExitOrInterrupt(endpoint string) error {
	for {
		select {
		case <-up.Context.Done():
			log.Debug("context is done, sending interrupt to process")
			if _, err := http.Get(endpoint); err != nil {
				log.Infof("failed to communicate to exec: %s", err)
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
			log.Debug("disconnected, sending interrupt to process")
			if _, err := http.Get(endpoint); err != nil {
				log.Infof("failed to communicate to exec: %s", err)
			}
			return errors.ErrLostConnection
		}
	}
}

func (up *UpContext) devMode(isRetry bool) error {
	var err error
	var namespace string
	up.Client, up.RestConfig, namespace, err = k8Client.GetLocal()
	if err != nil {
		return err
	}
	if up.Dev.Namespace == "" {
		up.Dev.Namespace = namespace
	}

	up.Sy, err = syncthing.New(up.Dev)
	if err != nil {
		return err
	}

	d, err := deployments.Get(up.Dev.Name, up.Dev.Namespace, up.Client)
	create := false
	if err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
		if !askYesNo(fmt.Sprintf("Deployment '%s' doesn't exist. Do you want to create a new one? [y/n]: ", up.Dev.Name)) {
			return fmt.Errorf("deployment %s not found [current context: %s]", up.Dev.Name, up.Dev.Namespace)
		}
		d = deployments.GevDevSandbox(up.Dev)
		create = true
	}
	progress := newProgressBar("Activating your Okteto Environment...")
	progress.start()
	defer progress.stop()

	if isRetry && !deployments.IsDevModeOn(d) {
		return fmt.Errorf("Your Okteto Environment has been deactivated")
	}

	if err := secrets.Create(up.Dev, up.Client); err != nil {
		return err
	}

	if err := volumes.Create(volumes.GetVolumeName(up.Dev), up.Dev, up.Client); err != nil {
		return err
	}

	for i := range up.Dev.Volumes {
		if err := volumes.Create(volumes.GetVolumeDataName(up.Dev, i), up.Dev, up.Client); err != nil {
			return err
		}
	}

	c, err := deployments.DevModeOn(d, up.Dev, create, up.Client)
	if err != nil {
		return err
	}

	up.Container = c.Name

	if create {
		if err := services.Create(up.Dev, up.Client); err != nil {
			return err
		}
	}

	p, err := pods.GetDevPod(up.Context, up.Dev, up.Client)
	if err != nil {
		return err
	}

	up.Pod = p.Name

	return nil
}

func (up *UpContext) startSync() error {
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

	for _, f := range up.Dev.Forward {
		if err := up.Forwarder.Add(f.Local, f.Remote); err != nil {
			return err
		}
	}

	up.Forwarder.Start(up.Pod, up.Dev.Namespace)
	go up.Sy.Monitor(up.Context, up.WG, up.Disconnect)

	if err := up.Sy.WaitForPing(up.Context, up.WG); err != nil {
		return err
	}

	if err := up.Sy.WaitForCompletion(up.Context, up.WG, up.Dev); err != nil {
		return err
	}

	return nil
}

func (up *UpContext) forceLocalSyncState() error {
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

	return up.Sy.Restart(up.Context, up.WG)
}

// Shutdown runs the cancellation sequence. It will wait for all tasks to finish for up to 500 milliseconds
func (up *UpContext) shutdown() {
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

func printDisplayContext(message, namespace, name string, ports []model.Forward) {
	log.Success(message)
	log.Println(fmt.Sprintf("    %s %s", log.BlueString("Namespace:"), namespace))
	log.Println(fmt.Sprintf("    %s      %s", log.BlueString("Name:"), name))
	if len(ports) > 0 {
		log.Println(fmt.Sprintf("    %s   %d -> %d", log.BlueString("Forward:"), ports[0].Local, ports[0].Remote))
		for i := 1; i < len(ports); i++ {
			log.Println(fmt.Sprintf("               %d -> %d", ports[i].Local, ports[i].Remote))
		}
	}
	fmt.Println()
}

func (up *UpContext) buildExecCommand() (*exec.Cmd, int) {
	port, err := model.GetAvailablePort()
	if err != nil {
		log.Infof("couldn't access the network: %s", err)
		port = 15000
	}

	args := []string{
		"exec",
		"--pod",
		up.Pod,
		"--container",
		up.Container,
		"--port",
		fmt.Sprintf("%d", port),
		"-n",
		up.Dev.Namespace,
		"--",
	}
	args = append(args, up.Dev.Command...)

	cmd := exec.Command(config.GetBinaryFullPath(), args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd, port
}
