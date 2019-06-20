package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/docker/docker/pkg/term"
	"github.com/okteto/okteto/pkg/analytics"
	"github.com/okteto/okteto/pkg/errors"
	k8Client "github.com/okteto/okteto/pkg/k8s/client"
	"github.com/okteto/okteto/pkg/k8s/deployments"
	"github.com/okteto/okteto/pkg/k8s/exec"
	"github.com/okteto/okteto/pkg/k8s/pods"
	"github.com/okteto/okteto/pkg/k8s/secrets"
	"github.com/okteto/okteto/pkg/k8s/services"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	syncK8s "github.com/okteto/okteto/pkg/syncthing/k8s"

	"github.com/okteto/okteto/pkg/k8s/forward"
	"github.com/okteto/okteto/pkg/syncthing"

	"github.com/spf13/cobra"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// ReconnectingMessage is the messaged show when we are trying to reconnect
const ReconnectingMessage = "Trying to reconnect to your cluster. File synchronization will automatically resume when the connection improves."

// UpContext is the common context of all operations performed during
// the up command
type UpContext struct {
	Context       context.Context
	Cancel        context.CancelFunc
	WG            *sync.WaitGroup
	Dev           *model.Dev
	Client        *kubernetes.Clientset
	RestConfig    *rest.Config
	DevPod        string
	SyncPod       string
	Node          string
	DevForwarder  *forward.PortForwardManager
	SyncForwarder *forward.PortForwardManager
	Disconnect    chan struct{}
	Running       chan error
	Exit          chan error
	Sy            *syncthing.Syncthing
	ErrChan       chan error
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

			if syncthingUpgradeAvailable() {
				fmt.Println("Installing dependencies...")
				if err := downloadSyncthing(); err != nil {
					return fmt.Errorf("couldn't download syncthing, please try again")
				}
			}

			dev, err := loadDev(devPath)
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

	cmd.Flags().StringVarP(&devPath, "file", "f", defaultManifest, "path to the manifest file")
	cmd.Flags().StringVarP(&namespace, "namespace", "n", "", "namespace where the up command is executed")
	return cmd
}

//RunUp starts the up sequence
func RunUp(dev *model.Dev) error {
	up := &UpContext{
		WG:   &sync.WaitGroup{},
		Dev:  dev,
		Exit: make(chan error, 1),
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
	retry := false
	inFd, _ := term.GetFdInfo(os.Stdin)
	state, err := term.SaveState(inFd)
	if err != nil {
		up.Exit <- err
		return
	}

	var namespace string
	up.Client, up.RestConfig, namespace, err = k8Client.GetLocal()
	if err != nil {
		up.Exit <- err
		return
	}
	if up.Dev.Namespace == "" {
		up.Dev.Namespace = namespace
	}

	for {
		up.Context, up.Cancel = context.WithCancel(context.Background())
		up.Disconnect = make(chan struct{}, 1)
		up.Running = make(chan error, 1)
		up.ErrChan = make(chan error, 1)

		d, err := deployments.Get(up.Dev.Name, up.Dev.Namespace, up.Client)
		create := false
		if err != nil {
			if !errors.IsNotFound(err) || retry {
				up.Exit <- err
				return
			}
			d = deployments.GevDevSandbox(up.Dev)
			create = true
		}
		devContainer := deployments.GetDevContainer(d, up.Dev.Name)
		if up.Dev.Image == "" {
			up.Dev.Image = devContainer.Image
		}

		if retry && !deployments.IsDevModeOn(d) {
			up.Exit <- fmt.Errorf("Okteto Environment has been deactivated")
			return
		}

		err = up.sync(d, devContainer)
		if err != nil {
			up.Exit <- err
			return
		}
		log.Success("Files synchronized")

		err = up.devMode(retry, d, create)
		if err != nil {
			up.Exit <- err
			return
		}
		retry = true

		printDisplayContext("Okteto Environment activated", up.Dev.Namespace, up.Dev.Name, up.Dev.Forward)

		go func() {
			up.Running <- up.runCommand()
			return
		}()

		prevError := up.WaitUntilExitOrInterrupt()
		log.Debug("Restoring terminal")
		if err := term.RestoreTerminal(inFd, state); err != nil {
			log.Debugf("failed to restore terminal: %s", err)
		} else {
			log.Debug("Terminal restored")
		}

		if prevError != nil {
			if prevError == errors.ErrLostConnection || (prevError == errors.ErrCommandFailed && !up.Sy.IsConnected()) {
				log.Yellow("\nConnection lost to your Okteto Environment, reconnecting...\n")
				up.shutdown()
				continue
			}
		}

		up.Exit <- prevError
		return
	}
}

// WaitUntilExitOrInterrupt blocks execution until a stop signal is sent or a disconnect event or an error
func (up *UpContext) WaitUntilExitOrInterrupt() error {
	for {
		select {
		case err := <-up.Running:
			fmt.Println()
			if err != nil {
				log.Infof("Command execution error: %s\n", err)
				return errors.ErrCommandFailed
			}
			return nil

		case err := <-up.ErrChan:
			log.Yellow(err.Error())

		case <-up.Disconnect:
			return errors.ErrLostConnection
		}
	}
}

func (up *UpContext) sync(d *appsv1.Deployment, c *apiv1.Container) error {
	var err error
	progress := newProgressBar("Synchronizing your files...")
	progress.start()
	defer progress.stop()

	up.Sy, err = syncthing.New(up.Dev)
	if err != nil {
		return err
	}

	if err := up.Sy.Run(up.Context, up.WG); err != nil {
		return err
	}

	if err := secrets.Create(up.Dev, up.Client); err != nil {
		return err
	}

	if err := syncK8s.Deploy(up.Dev, d, c, up.Client); err != nil {
		return err
	}

	p, err := pods.GetDevPod(up.Context, up.Dev, pods.OktetoSyncLabel, up.Client)
	if err != nil {
		return err
	}

	up.SyncPod = p.Name
	up.Node = p.Spec.NodeName

	up.SyncForwarder = forward.NewPortForwardManager(up.Context, up.RestConfig, up.Client, up.ErrChan)
	if err := up.SyncForwarder.Add(up.Sy.RemotePort, syncthing.ClusterPort); err != nil {
		return err
	}
	if err := up.SyncForwarder.Add(up.Sy.RemoteGUIPort, syncthing.GUIPort); err != nil {
		return err
	}

	up.SyncForwarder.Start(up.SyncPod, up.Dev.Namespace)

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

	return up.Sy.Restart(up.Context, up.WG)
}

func (up *UpContext) devMode(isRetry bool, d *appsv1.Deployment, create bool) error {
	progress := newProgressBar("Activating your Okteto Environment...")
	progress.start()
	defer progress.stop()

	deploys := map[string]*appsv1.Deployment{up.Dev.Name: d}

	for _, s := range up.Dev.Services {
		if _, ok := deploys[s.Name]; ok {
			continue
		}
		d, err := deployments.Get(s.Name, up.Dev.Namespace, up.Client)
		if err != nil {
			return err
		}
		deploys[s.Name] = d
	}

	if err := deployments.TraslateDevMode(deploys, up.Dev, up.Node); err != nil {
		return err
	}

	for _, d := range deploys {
		forceCreate := false
		if d.Name == up.Dev.Name {
			forceCreate = create
		}
		if err := deployments.Deploy(d, forceCreate, up.Client); err != nil {
			return err
		}
	}

	if create {
		if err := services.Create(up.Dev, up.Client); err != nil {
			return err
		}
	}

	p, err := pods.GetDevPod(up.Context, up.Dev, pods.OktetoDevLabel, up.Client)
	if err != nil {
		return err
	}

	up.DevPod = p.Name

	up.DevForwarder = forward.NewPortForwardManager(up.Context, up.RestConfig, up.Client, up.ErrChan)
	for _, f := range up.Dev.Forward {
		if err := up.DevForwarder.Add(f.Local, f.Remote); err != nil {
			return err
		}
	}
	up.DevForwarder.Start(up.DevPod, up.Dev.Namespace)

	return nil
}

func (up *UpContext) runCommand() error {
	exec.Exec(
		up.Context,
		up.Client,
		up.RestConfig,
		up.Dev.Namespace,
		up.DevPod,
		up.Dev.Container,
		true,
		os.Stdin,
		os.Stdout,
		os.Stderr,
		[]string{"sh", "-c", "trap '' TERM && kill -- -1 && sleep 0.1 & kill -s KILL -- -1 >/dev/null 2>&1"},
	)
	return exec.Exec(
		up.Context,
		up.Client,
		up.RestConfig,
		up.Dev.Namespace,
		up.DevPod,
		up.Dev.Container,
		true,
		os.Stdin,
		os.Stdout,
		os.Stderr,
		up.Dev.Command,
	)
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
		if up.DevForwarder != nil {
			up.DevForwarder.Stop()
		}
		if up.SyncForwarder != nil {
			up.SyncForwarder.Stop()
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
