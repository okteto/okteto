package deploy

import (
	"context"
	"crypto/tls"
	"fmt"
	"os"
	"path/filepath"

	"github.com/google/uuid"
	contextCMD "github.com/okteto/okteto/cmd/context"
	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/spf13/cobra"
	"k8s.io/client-go/rest"
)

var tempKubeConfig = "/var/tmp/okteto.kubeconfig"

type kubeConfigHandler interface {
	Read() (*rest.Config, error)
	Modify(ctx context.Context, port int, sessionToken string) error
}

type proxyInterface interface {
	Start(ctx context.Context, name string, clusterConfig *rest.Config) error
	Shutdown(ctx context.Context) error
	GetPort() int
	GetToken() string
}

type manifestExecutor interface {
	Execute(command string, env []string) error
}

type deployCommand struct {
	getSecrets  func(ctx context.Context) ([]string, error)
	getManifest func(cwd, name, filename string) (*Manifest, error)

	proxy      proxyInterface
	kubeconfig kubeConfigHandler
	executor   manifestExecutor
}

//Deploy deploys the okteto manifest
func Deploy(ctx context.Context) *cobra.Command {
	var filename string
	var variables []string
	var name string

	cmd := &cobra.Command{
		Use:   "deploy",
		Short: "Executes the list of commands specified in the Okteto manifest",
		Args:  utils.NoArgsAccepted("https://okteto.com/docs/reference/cli/#version"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := contextCMD.Init(ctx); err != nil {
				return err
			}

			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to get the current working directory: %w", err)
			}

			if name == "" {
				name = getName(cwd)
			}

			// Look for a free local port to start the proxy
			port, err := model.GetAvailablePort("localhost")
			if err != nil {
				log.Errorf("could not find a free port to start proxy server: %s", err)
				return err
			}
			log.Debugf("found available port %d", port)

			// TODO for now, using self-signed certificates
			cert, err := tls.X509KeyPair(cert, key)
			if err != nil {
				log.Errorf("could not read certificate: %s", err)
				return err
			}

			sessionToken := uuid.NewString()

			c := &deployCommand{
				getSecrets:  getOktetoSecretsAsEnvironmenVariables,
				getManifest: getManifest,

				kubeconfig: newKubeConfig(),
				executor:   newExecutor(),
				proxy: newProxy(proxyConfig{
					port:         port,
					certificates: []tls.Certificate{cert},
					token:        sessionToken,
				}),
			}
			return c.runDeploy(ctx, cwd, name, filename, variables)
		},
	}

	cmd.Flags().StringVarP(&name, "name", "a", "", "application name")
	cmd.Flags().StringVarP(&filename, "filename", "f", "", "path to the manifest file")
	cmd.Flags().StringArrayVarP(&variables, "var", "v", []string{}, "set a pipeline variable (can be set more than once)")

	return cmd
}

func (dc *deployCommand) runDeploy(ctx context.Context, cwd, name, filename string, variables []string) error {
	clusterConfig, err := dc.kubeconfig.Read()
	if err != nil {
		log.Errorf("could not build kube config: %s", err)
		return err
	}

	if err := dc.kubeconfig.Modify(ctx, dc.proxy.GetPort(), dc.proxy.GetToken()); err != nil {
		log.Errorf("could not create temporal kubeconfig %s", err)
		return err
	}

	// Read manifest file and execute commands
	m, err := dc.getManifest(cwd, name, filename)
	if err != nil {
		log.Errorf("could not find manifest file to be executed: %s", err)
		return err
	}

	// Get secrets from API
	sList, err := dc.getSecrets(ctx)
	if err != nil {
		log.Errorf("could not load Okteto Secrets: %s", err.Error())
		sList = []string{}
	}

	if err := dc.proxy.Start(ctx, name, clusterConfig); err != nil {
		log.Errorf("could not start proxy server %s", err)
		return err
	}

	// Include secrets as part of variables list
	env := append(variables, sList...)
	// Set KUBECONFIG environment variable to make sure it executes kubectl commands against the proxy
	env = append(env, fmt.Sprintf("KUBECONFIG=%s", tempKubeConfig))

	var commandErr error
	for _, command := range m.Deploy {
		if err := dc.executor.Execute(command, env); err != nil {
			log.Errorf("error executing command '%s': %s", command, err.Error())
			commandErr = err
			break
		}
	}

	if err := dc.proxy.Shutdown(ctx); err != nil {
		log.Errorf("could not stop local server: %s", err)
		return err
	}

	return commandErr
}

/*func runDeploy(ctx context.Context, cwd, name, filename string, variables []string) error {
	// Look for a free local port to start the proxy
	port, err := model.GetAvailablePort("localhost")
	if err != nil {
		log.Fatalf("could not find a free port to start proxy server: %s", err)
		return err
	}
	log.Debugf("found available port %d", port)

	// TODO for now, using self-signed certificates
	c, err := tls.X509KeyPair(cert, key)
	if err != nil {
		log.Fatalf("could not read certificate: %s", err)
		return err
	}

	sessionToken := uuid.NewString()
	clusterConfig, err := readKubeConfig()
	if err != nil {
		log.Errorf("could not build kube config: %s", err)
		return err
	}

	p := newProxy(proxyConfig{
		port:         port,
		certificates: []tls.Certificate{c},
		token:        sessionToken,
	})

	if err := p.startProxy(ctx, name, clusterConfig); err != nil {
		log.Fatalf("could not start proxy server %s", err)
		return err
	}

	modifyKubeconfig(ctx, port, sessionToken, clusterConfig)

	// Get secrets from API
	sList, err := getOktetoSecretsAsEnvironmenVariables(ctx)
	if err != nil {
		log.Errorf("could not load Okteto Secrets: %s", err.Error())
		sList = []string{}
	}

	// Get variables
	env := append(variables, sList...)
	// Set KUBECONFIG environment variable to make sure it executes kubectl commands against the proxy
	env = append(env, fmt.Sprintf("KUBECONFIG=%s", tempKubeConfig))

	//Read pipeline file and execute commands
	m, err := getManifest(cwd, name, filename)

	for _, command := range m.Deploy {
		if err := executeCommand(command, env); err != nil {
			log.Errorf("error executing command '%s': %s", command, err.Error())
			break
		}
	}

	if err := p.shutdown(ctx); err != nil {
		log.Fatalf("could not stop local server: %s", err)
		return err
	}
	return nil
}*/

func getName(cwd string) string {
	repo, err := model.GetRepositoryURL(cwd)
	if err != nil {
		log.Info("inferring name from folder")
		return filepath.Base(cwd)
	}

	log.Info("inferring name from git repository URL")
	return model.TranslateURLToName(repo)
}
