package cmd

import (
	"fmt"
	"strings"
	"time"

	"cli/cnd/pkg/errors"
	k8Client "cli/cnd/pkg/k8/client"
	"cli/cnd/pkg/k8/deployments"
	"cli/cnd/pkg/log"
	"cli/cnd/pkg/model"
	"cli/cnd/pkg/storage"
	"cli/cnd/pkg/syncthing"

	"github.com/briandowns/spinner"

	"github.com/spf13/cobra"
)

//Down stops a cloud native environment
func Down() *cobra.Command {
	var devPath string
	var removeVolumes bool
	var force bool
	cmd := &cobra.Command{
		Use:   "down",
		Short: "Deactivate your cloud native development environment",
		RunE: func(cmd *cobra.Command, args []string) error {
			log.Debug("starting down command")

			devEnvironments, err := getDevEnvironments(false, true)
			if err != nil {
				return fmt.Errorf("failed to deactivate your environment: %s", err)
			}

			if len(devEnvironments) == 0 {
				log.Debugf("No CND environment running")
				return nil
			}

			var devEnvironment *devenv
			if len(devEnvironments) == 1 {
				devEnvironment = &devEnvironments[0]
			} else {
				devEnvironments = getDevEnvironmentByManifest(devEnvironments, devPath)
				switch len(devEnvironments) {
				case 0:
					return fmt.Errorf("None of the active cloud native environments matches %s", devPath)
				case 1:
					devEnvironment = &devEnvironments[0]
				default:
					n, err := k8Client.GetCurrentNamespace()
					if err != nil {
						return fmt.Errorf("Couldn't get your kubectl context: %s", err)
					}

					devEnvironment, err = askUserForDevEnvironment(devEnvironments, n)
					if err != nil {
						return err
					}
				}
			}

			s := spinner.New(spinner.CharSets[14], 100*time.Millisecond)
			s.Suffix = " Deactivating your cloud native development environment..."
			s.Start()
			err = executeDown(devEnvironment, removeVolumes, force)
			s.Stop()
			if err == nil {
				fmt.Printf("%s %s\n", log.SuccessSymbol, log.GreenString("Environment deactivated"))
				return nil
			}

			return err

		},
	}

	addDevPathFlag(cmd, &devPath)
	cmd.Flags().BoolVarP(&removeVolumes, "volumes", "v", false, "remove persistent volumes")
	cmd.Flags().BoolVarP(&force, "force", "", false, "force down execution if there are errors")
	return cmd
}

func executeDown(devEnvironment *devenv, removeVolumes, force bool) error {
	_, client, _, k8sContext, err := k8Client.Get(devEnvironment.Namespace)
	if err != nil {
		return err
	}

	var dev *model.Dev
	d, err := deployments.Get(devEnvironment.Namespace, devEnvironment.Deployment, client)
	if err == nil {
		dev, err = deployments.GetDevFromAnnotation(d.GetObjectMeta(), devEnvironment.Container)
		if err != nil {
			if err != errors.ErrNotDevDeployment && !force {
				return err
			}
			dev = &model.Dev{Name: devEnvironment.Deployment, Container: devEnvironment.Container}
		}
		if deployments.IsAutoDestroyOnDown(d.GetObjectMeta()) {
			if err := deployments.Destroy(d, client); err != nil {
				return fmt.Errorf("couldn't destroy deployment %s: %s", devEnvironment.Deployment, err)
			}
		} else {
			if err := deployments.DevModeOff(d, dev, client, removeVolumes); err != nil && !force {
				return err
			}
		}
	} else {
		fullname := deployments.GetFullName(devEnvironment.Namespace, devEnvironment.Deployment)
		if force {
			log.Errorf("error getting deployment %s [current context: %s]", fullname, k8sContext)
			dev = &model.Dev{Name: devEnvironment.Deployment, Container: devEnvironment.Container}
		} else {
			if strings.Contains(err.Error(), "not found") {
				return fmt.Errorf("deployment %s not found [current context: %s]", fullname, k8sContext)
			}
			return err
		}
	}

	sy, err := syncthing.NewSyncthing(devEnvironment.Namespace, devEnvironment.Deployment, nil, false)
	if err != nil {
		return err
	}

	if err := storage.Delete(devEnvironment.Namespace, dev); err != nil {
		return err
	}

	err = sy.Stop()
	if err != nil {
		return err
	}

	err = sy.RemoveFolder()
	if err != nil {
		return err
	}

	return nil
}
