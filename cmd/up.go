package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/pkg/term"
	"github.com/okteto/okteto/pkg/analytics"
	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/errors"
	k8Client "github.com/okteto/okteto/pkg/k8s/client"
	"github.com/okteto/okteto/pkg/k8s/deployments"
	"github.com/okteto/okteto/pkg/k8s/exec"
	"github.com/okteto/okteto/pkg/k8s/namespaces"
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
	success       bool
}

//Up starts a cloud dev environment
func Up() *cobra.Command {
	var devPath string
	var namespace string
	var remote int32
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

			checkWatchesConfiguration()

			dev, err := loadDev(devPath)
			if err != nil {
				return err
			}
			if namespace != "" {
				dev.Namespace = namespace
			}

			if remote > 0 {
				dev.LoadRemote(int(remote))
			}

			err = RunUp(dev)
			return err
		},
	}

	cmd.Flags().StringVarP(&devPath, "file", "f", defaultManifest, "path to the manifest file")
	cmd.Flags().StringVarP(&namespace, "namespace", "n", "", "namespace where the up command is executed")
	cmd.Flags().Int32VarP(&remote, "remote", "r", 0, "configures remote execution on the specified port")
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
			up.updateStateFile(failed)
			return err
		}
	}
	return nil
}

// Activate activates the dev environment
func (up *UpContext) Activate() {
	retry := false
	var state *term.State
	inFd, isTerm := term.GetFdInfo(os.Stdin)
	if isTerm {
		var err error
		state, err = term.SaveState(inFd)
		if err != nil {
			up.Exit <- err
			return
		}
	}

	var namespace string
	var err error
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

		d, err := deployments.Get(up.Dev, up.Dev.Namespace, up.Client)
		create := false
		if err != nil {
			if !errors.IsNotFound(err) || retry {
				up.Exit <- err
				return
			}

			if len(up.Dev.Labels) == 0 {
				_, deploy := os.LookupEnv("OKTETO_AUTODEPLOY")
				if !deploy {
					deploy = askYesNo(fmt.Sprintf("Deployment %s doesn't exist in namespace %s. Do you want to create a new one? [y/n]: ", up.Dev.Name, up.Dev.Namespace))
				}

				if !deploy {
					up.Exit <- errors.UserError{
						E:    fmt.Errorf("Deployment %s doesn't exist in namespace %s", up.Dev.Name, up.Dev.Namespace),
						Hint: "Deploy your application first or use `okteto namespace` to select a different namespace and try again"}
					return
				}
			} else {
				if err == errors.ErrNotFound {
					err = errors.UserError{
						E:    fmt.Errorf("Didn't find a deployment in namespace %s that matches the labels in your Okteto manifest", up.Dev.Namespace),
						Hint: "Update your labels or use `okteto namespace` to select a different namespace and try again"}
				}
				up.Exit <- err
				return
			}

			d = up.Dev.GevSandbox()
			create = true
		}

		if namespaces.IsOktetoNamespace(up.Dev.Namespace, up.Client) {
			if err := namespaces.CheckAvailableResources(up.Dev, create, d, up.Client); err != nil {
				up.Exit <- err
				return
			}
		}

		devContainer := deployments.GetDevContainer(&d.Spec.Template.Spec, up.Dev.Container)
		if devContainer == nil {
			up.Exit <- fmt.Errorf("Container '%s' does not exist in deployment '%s'", up.Dev.Container, up.Dev.Name)
			return
		}
		up.Dev.Container = devContainer.Name
		if up.Dev.Image == "" {
			up.Dev.Image = devContainer.Image
		}

		if retry && !deployments.IsDevModeOn(d) {
			up.Exit <- fmt.Errorf("Okteto Environment has been deactivated")
			return
		}

		up.updateStateFile(starting)
		err = up.sync(d, devContainer)
		if err != nil {
			up.Exit <- err
			return
		}

		err = up.devMode(retry, d, create)
		if err != nil {
			up.Exit <- err
			return
		}

		up.success = true
		retry = true

		printDisplayContext("Okteto Environment activated", up.Dev)

		go func() {
			up.Running <- up.runCommand()
		}()

		prevError := up.WaitUntilExitOrInterrupt()
		if isTerm {
			log.Debug("Restoring terminal")
			if err := term.RestoreTerminal(inFd, state); err != nil {
				log.Debugf("failed to restore terminal: %s", err)
			} else {
				log.Debug("Terminal restored")
			}
		}

		if prevError != nil {
			if prevError == errors.ErrLostConnection || (prevError == errors.ErrCommandFailed && !pods.Exists(up.DevPod, up.Dev.Namespace, up.Client)) {
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
	err := up.startRemoteSyncthing(d, c)
	if err != nil {
		return err
	}
	log.Success("Persistent volume provisioned")

	if err := up.startLocalSyncthing(); err != nil {
		return err
	}

	if err := up.synchronizeFiles(); err != nil {
		return err
	}

	log.Success("Files synchronized")
	return nil
}

func (up *UpContext) startRemoteSyncthing(d *appsv1.Deployment, c *apiv1.Container) error {
	progress := newProgressSpinner("Provisioning your persistent volume...")
	progress.start()
	up.updateStateFile(provisioning)

	defer progress.stop()

	log.Info("create deployment secrets")
	if err := secrets.Create(up.Dev, up.Client); err != nil {
		return err
	}

	log.Info("deploying syncK8s")
	if err := syncK8s.Deploy(up.Dev, d, c, up.Client); err != nil {
		return err
	}

	log.Info("getting the sync pod")
	p, err := pods.GetByLabel(up.Context, up.Dev, pods.OktetoSyncLabel, up.Client, true)
	if err != nil {
		return err
	}

	up.SyncPod = p.Name
	up.Node = p.Spec.NodeName

	up.Sy, err = syncthing.New(up.Dev)
	if err != nil {
		return err
	}

	up.SyncForwarder = forward.NewPortForwardManager(up.Context, up.RestConfig, up.Client, up.ErrChan)
	if err := up.SyncForwarder.Add(up.Sy.RemotePort, syncthing.ClusterPort); err != nil {
		return err
	}
	if err := up.SyncForwarder.Add(up.Sy.RemoteGUIPort, syncthing.GUIPort); err != nil {
		return err
	}

	up.SyncForwarder.Start(up.SyncPod, up.Dev.Namespace)
	return nil
}

func (up *UpContext) startLocalSyncthing() error {
	progress := newProgressSpinner("Starting the file synchronization service...")
	progress.start()
	up.updateStateFile(startingSync)
	defer progress.stop()

	if err := up.Sy.Run(up.Context, up.WG); err != nil {
		return err
	}

	if err := up.Sy.WaitForPing(up.Context, up.WG, true); err != nil {
		return err
	}
	up.Sy.SendStignoreFile(up.Context, up.WG, up.Dev)
	if err := up.Sy.WaitForScanning(up.Context, up.WG, up.Dev, true); err != nil {
		return err
	}
	return nil
}

func (up *UpContext) synchronizeFiles() error {
	progress := newProgressSpinner("Synchronizing your files...")
	up.updateStateFile(synchronizing)
	progress.start()
	defer progress.stop()

	reporter := make(chan float64)
	defer close(reporter)

	go func() {
		// only show the progress if sync takes more than 5 seconds
		<-time.NewTicker(5 * time.Second).C

		previous := 0.1
		for c := range reporter {
			if c > previous {
				text := fmt.Sprintf("Synchronizing your files (%s)...", fmt.Sprintf("%6v", fmt.Sprintf("%.2f%%", c)))
				progress.update(text)
				previous = c
			}
		}
	}()

	if err := up.Sy.WaitForCompletion(up.Context, up.WG, up.Dev, reporter); err != nil {
		if err == errors.ErrSyncFrozen {
			return errors.UserError{
				E: err,
				Hint: fmt.Sprintf(`Help us improve okteto by filing an issue in https://github.com/okteto/okteto/issues/new.
    Please include your syncthing log (%s) if possible.`, up.Sy.LogPath),
			}
		}

		return err
	}

	up.Sy.Type = "sendreceive"
	if err := up.Sy.UpdateConfig(); err != nil {
		return err
	}

	go up.Sy.Monitor(up.Context, up.WG, up.Disconnect)
	return up.Sy.Restart(up.Context, up.WG)
}

func (up *UpContext) devMode(isRetry bool, d *appsv1.Deployment, create bool) error {
	progress := newProgressSpinner("Activating your Okteto Environment...")
	up.updateStateFile(activating)
	progress.start()
	defer progress.stop()

	tr, err := deployments.GetTranslations(up.Dev, d, up.Node, up.Client)
	if err != nil {
		return err
	}

	if err := deployments.TraslateDevMode(tr, up.Dev.Namespace, up.Client); err != nil {
		return err
	}

	for name := range tr {
		if name == d.Name {
			if err := deployments.Deploy(tr[name].Deployment, create, up.Client); err != nil {
				return err
			}
		} else {
			if err := deployments.Deploy(tr[name].Deployment, false, up.Client); err != nil {
				return err
			}
		}
	}
	if create && len(up.Dev.Services) == 0 {
		if err := services.Create(up.Dev, up.Client); err != nil {
			return err
		}
	}

	p, err := pods.GetByLabel(up.Context, up.Dev, pods.OktetoInteractiveDevLabel, up.Client, true)
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
	in := strings.NewReader("\n")
	if err := exec.Exec(
		up.Context,
		up.Client,
		up.RestConfig,
		up.Dev.Namespace,
		up.DevPod,
		up.Dev.Container,
		false,
		in,
		os.Stdout,
		os.Stderr,
		[]string{"sh", "-c", "((cp /var/okteto/bin/* /usr/local/bin); (trap '' TERM && kill -- -1 && sleep 0.1 & kill -s KILL -- -1 )) >/dev/null 2>&1"},
	); err != nil {
		log.Infof("first session to the remote container: %s", err)
	}

	log.Infof("starting remote command")
	up.updateStateFile(ready)

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
	log.Debugf("up shutdown")
	analytics.TrackUp(up.Dev.Image, config.VersionString, up.success)

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

func printDisplayContext(message string, dev *model.Dev) {
	log.Success(message)
	log.Println(fmt.Sprintf("    %s %s", log.BlueString("Namespace:"), dev.Namespace))
	log.Println(fmt.Sprintf("    %s      %s", log.BlueString("Name:"), dev.Name))
	if len(dev.Forward) > 0 {
		log.Println(fmt.Sprintf("    %s   %d -> %d", log.BlueString("Forward:"), dev.Forward[0].Local, dev.Forward[0].Remote))
		for i := 1; i < len(dev.Forward); i++ {
			log.Println(fmt.Sprintf("               %d -> %d", dev.Forward[i].Local, dev.Forward[i].Remote))
		}
	}
	fmt.Println()
}
