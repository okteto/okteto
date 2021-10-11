package deploy

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	contextCMD "github.com/okteto/okteto/cmd/context"
	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
)

const tempKubeConfig = "/tmp/.okteto/kubeconfig.json"

// Options options for deploy command
type Options struct {
	ManifestPath string
	Name         string
	Variables    []string
}

type kubeConfigHandler interface {
	Read() (*rest.Config, error)
	Modify(ctx context.Context, port int, sessionToken string) error
}

type proxyInterface interface {
	Start()
	Shutdown(ctx context.Context) error
	GetPort() int
	GetToken() string
}

type deployCommand struct {
	getManifest func(cwd, name, filename string) (*utils.Manifest, error)

	proxy      proxyInterface
	kubeconfig kubeConfigHandler
	executor   utils.ManifestExecutor
}

//Deploy deploys the okteto manifest
func Deploy(ctx context.Context) *cobra.Command {
	options := &Options{}

	cmd := &cobra.Command{
		Use:    "deploy",
		Short:  "Executes the list of commands specified in the Okteto manifest to deploy the application",
		Args:   utils.NoArgsAccepted("https://okteto.com/docs/reference/cli/#version"),
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := contextCMD.Init(ctx); err != nil {
				return err
			}

			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to get the current working directory: %w", err)
			}

			if options.Name == "" {
				options.Name = getName(cwd)
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

			// Generate a token for the requests done to the proxy
			sessionToken := uuid.NewString()

			kubeconfig := newKubeConfig()
			clusterConfig, err := kubeconfig.Read()
			if err != nil {
				log.Errorf("could not read kubeconfig file: %s", err)
				return err
			}

			handler, err := getProxyHandler(options.Name, sessionToken, clusterConfig)
			if err != nil {
				log.Errorf("could not configure local proxy: %s", err)
				return err
			}

			s := &http.Server{
				Addr:         fmt.Sprintf(":%d", port),
				Handler:      handler,
				ReadTimeout:  5 * time.Second,
				WriteTimeout: 10 * time.Second,
				IdleTimeout:  120 * time.Second,
				TLSConfig: &tls.Config{
					Certificates: []tls.Certificate{cert},
				},
			}

			c := &deployCommand{
				getManifest: utils.GetManifest,

				kubeconfig: kubeconfig,
				executor:   utils.NewExecutor(),
				proxy: newProxy(proxyConfig{
					port:  port,
					token: sessionToken,
				}, s),
			}
			return c.runDeploy(ctx, cwd, options)
		},
	}

	cmd.Flags().StringVarP(&options.Name, "name", "a", "", "application name")
	cmd.Flags().StringVarP(&options.ManifestPath, "filename", "f", "", "path to the manifest file")
	cmd.Flags().StringArrayVarP(&options.Variables, "var", "v", []string{}, "set a variable (can be set more than once)")

	return cmd
}

func (dc *deployCommand) runDeploy(ctx context.Context, cwd string, opts *Options) error {
	if err := dc.kubeconfig.Modify(ctx, dc.proxy.GetPort(), dc.proxy.GetToken()); err != nil {
		log.Errorf("could not create temporal kubeconfig %s", err)
		return err
	}

	// Read manifest file with the commands to be executed
	manifest, err := dc.getManifest(cwd, opts.Name, opts.ManifestPath)
	if err != nil {
		log.Errorf("could not find manifest file to be executed: %s", err)
		return err
	}

	log.Debugf("start server on %d", dc.proxy.GetPort())
	dc.proxy.Start()

	defer func() {
		log.Debugf("stopping local server...")
		if err := dc.proxy.Shutdown(ctx); err != nil {
			log.Errorf("could not stop local server: %s", err)
		}
	}()

	// Set variables and KUBECONFIG environment variable as environment for the commands to be executed
	env := append(opts.Variables, fmt.Sprintf("KUBECONFIG=%s", tempKubeConfig))

	var commandErr error
	for _, command := range manifest.Deploy {
		if err := dc.executor.Execute(command, env); err != nil {
			log.Errorf("error executing command '%s': %s", command, err.Error())
			commandErr = err
			break
		}
	}

	return commandErr
}

// This will probably will be moved to a common place when we implement the full dev spec
func getName(cwd string) string {
	repo, err := model.GetRepositoryURL(cwd)
	if err != nil {
		log.Info("inferring name from folder")
		return filepath.Base(cwd)
	}

	log.Info("inferring name from git repository URL")
	return model.TranslateURLToName(repo)
}

func getProxyHandler(name, token string, clusterConfig *rest.Config) (http.Handler, error) {
	trans, err := rest.TransportFor(clusterConfig)
	if err != nil {
		log.Errorf("could not get http transport from config: %s", err)
		return nil, err
	}

	handler := http.NewServeMux()

	proxy := httputil.NewSingleHostReverseProxy(&url.URL{
		Host:   strings.TrimPrefix(clusterConfig.Host, "https://"),
		Scheme: "https",
	})
	proxy.Transport = trans

	log.Debugf("forwarding host: %s", clusterConfig.Host)

	handler.HandleFunc("/", func(rw http.ResponseWriter, r *http.Request) {
		requestToken := r.Header.Get("Authorization")
		expectedToken := fmt.Sprintf("Bearer %s", token)
		// Validate token with the generated for the local kubeconfig file
		if requestToken != expectedToken {
			rw.WriteHeader(401)
			return
		}

		// Set the right bearer token based on the original kubeconfig
		r.Header.Set("Authorization", fmt.Sprintf("Bearer %s", clusterConfig.BearerToken))

		// Modify all resources updated or created to include the label.
		if r.Method == "PUT" || r.Method == "POST" {
			b, err := io.ReadAll(r.Body)
			if err != nil {
				log.Errorf("could not read the request body: %s", err)
				rw.WriteHeader(500)
				return
			}

			var body map[string]json.RawMessage
			if err := json.Unmarshal(b, &body); err != nil {
				log.Errorf("could not unmarshal request: %s", err)
				rw.WriteHeader(500)
				return
			}

			m, ok := body["metadata"]
			if !ok {
				log.Error("request body doesn't have metadata field")
				rw.WriteHeader(500)
				return
			}

			var metadata metav1.ObjectMeta
			if err := json.Unmarshal(m, &metadata); err != nil {
				log.Errorf("could not process resource's metadata: %s", err)
				rw.WriteHeader(500)
				return
			}

			if metadata.Labels == nil {
				metadata.Labels = map[string]string{}
			}
			metadata.Labels[model.DeployedByLabel] = name

			metadataAsByte, err := json.Marshal(metadata)
			if err != nil {
				log.Errorf("could not process resource's metadata: %s", err)
				rw.WriteHeader(500)
				return
			}

			body["metadata"] = metadataAsByte

			b, err = json.Marshal(body)
			if err != nil {
				log.Errorf("could not marshal modified body: %s", err)
				rw.WriteHeader(500)
				return
			}

			// Needed to set the new Content-Length
			r.ContentLength = int64(len(b))
			r.Body = io.NopCloser(bytes.NewBuffer(b))
		}

		// Redirect request to the k8s server (based on the transport HTTP generated from the config)
		proxy.ServeHTTP(rw, r)
	})

	return handler, nil

}
