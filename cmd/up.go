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
	"github.com/okteto/okteto/pkg/k8s/volumes"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"

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
	Context    context.Context
	Cancel     context.CancelFunc
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
	success    bool
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

	ns, err := namespaces.Get(up.Dev.Namespace, up.Client)
	if err != nil {
		up.Exit <- err
		return
	}

	if !namespaces.IsOktetoAllowed(ns) {
		up.Exit <- fmt.Errorf("`okteto up` is not allowed in this namespace")
		return
	}

	for {
		up.Context, up.Cancel = context.WithCancel(context.Background())
		up.Disconnect = make(chan struct{}, 1)
		up.Running = make(chan error, 1)
		up.ErrChan = make(chan error, 1)

		if err := volumes.Create(up.Context, up.Dev, up.Client); err != nil {
			up.Exit <- err
			return
		}

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

		if namespaces.IsOktetoNamespace(ns) {
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
			log.Information("Okteto Environment has been deactivated")
			up.Exit <- nil
			return
		}

		up.updateStateFile(starting)

		log.Info("create deployment secrets")
		if err := secrets.Create(up.Dev, up.Client, up.Sy.GUIPasswordHash); err != nil {
			up.Exit <- err
			return
		}

		err = up.devMode(retry, ns, d, create)
		if err != nil {
			up.Exit <- err
			return
		}
		log.Success("Okteto Environment activated")

		err = up.sync()
		if err != nil {
			up.Exit <- err
			return
		}

		up.success = true
		retry = true

		printDisplayContext("Files synchronized", up.Dev)

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
			if prevError == errors.ErrLostConnection || (prevError == errors.ErrCommandFailed && !pods.Exists(up.Pod, up.Dev.Namespace, up.Client)) {
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

func (up *UpContext) devMode(isRetry bool, ns *apiv1.Namespace, d *appsv1.Deployment, create bool) error {
	spinner := newSpinner("Activating your Okteto Environment...")
	up.updateStateFile(activating)
	spinner.start()
	defer spinner.stop()

	tr, err := deployments.GetTranslations(up.Dev, d, up.Client)
	if err != nil {
		return err
	}

	if err := deployments.TraslateDevMode(tr, ns, up.Client); err != nil {
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

	up.Pod = p.Name
	sy, err := syncthing.New(up.Dev)
	if err != nil {
		return err
	}
	up.Sy = sy

	up.Forwarder = forward.NewPortForwardManager(up.Context, up.RestConfig, up.Client, up.ErrChan)
	for _, f := range up.Dev.Forward {
		if err := up.Forwarder.Add(f.Local, f.Remote); err != nil {
			return err
		}
	}
	if err := up.Forwarder.Add(up.Sy.RemotePort, syncthing.ClusterPort); err != nil {
		return err
	}
	if err := up.Forwarder.Add(up.Sy.RemoteGUIPort, syncthing.GUIPort); err != nil {
		return err
	}
	up.Forwarder.Start(up.Pod, up.Dev.Namespace)

	return nil
}

func (up *UpContext) sync() error {
	if err := up.startLocalSyncthing(); err != nil {
		return err
	}

	if err := up.synchronizeFiles(); err != nil {
		return err
	}

	return nil
}

func (up *UpContext) startLocalSyncthing() error {
	spinner := newSpinner("Starting the file synchronization service...")
	spinner.start()
	up.updateStateFile(startingSync)
	defer spinner.stop()

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
	postfix := "Synchronizing your files..."
	spinner := newSpinner(postfix)
	pbScaling := 0.30

	up.updateStateFile(synchronizing)
	spinner.start()
	defer spinner.stop()
	reporter := make(chan float64)
	go func() {
		<-time.NewTicker(2 * time.Second).C
		var previous float64

		for c := range reporter {
			if c > previous {
				// todo: how to calculate how many characters can the line fit?
				pb := renderProgressBar(postfix, c, pbScaling)
				spinner.update(pb)
				previous = c
			}
		}
	}()

	err := up.Sy.WaitForCompletion(up.Context, up.WG, up.Dev, reporter)
	if err != nil {
		if err == errors.ErrSyncFrozen {
			return errors.UserError{
				E: err,
				Hint: fmt.Sprintf(`Help us improve okteto by filing an issue in https://github.com/okteto/okteto/issues/new.
    Please include your syncthing log (%s) if possible.`, up.Sy.LogPath),
			}
		}

		return err
	}

	// render to 100
	spinner.update(renderProgressBar(postfix, 100, pbScaling))

	up.Sy.Type = "sendreceive"
	if err := up.Sy.UpdateConfig(); err != nil {
		return err
	}

	go up.Sy.Monitor(up.Context, up.WG, up.Disconnect)
	return up.Sy.Restart(up.Context, up.WG)
}

func (up *UpContext) runCommand() error {
	in := strings.NewReader("\n")
	if err := exec.Exec(
		up.Context,
		up.Client,
		up.RestConfig,
		up.Dev.Namespace,
		up.Pod,
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
		up.Pod,
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
		if up.Forwarder != nil {
			up.Forwarder.Stop()
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
