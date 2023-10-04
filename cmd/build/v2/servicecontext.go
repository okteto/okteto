package v2

import (
	"fmt"
	"strings"

	"github.com/okteto/okteto/pkg/model"
)

type serviceContextInterface interface {
	isCleanContext() bool
	getServiceHash() string
}

type serviceConfig struct {
	isClean bool
	hash    string
}

func (sc *serviceConfig) isCleanContext() bool {
	return sc.isClean
}

func (sc *serviceConfig) getServiceHash() string {
	return sc.hash
}

func getTextToHash(buildInfo *model.BuildInfo, sha string) string {
	args := []string{}
	for _, arg := range buildInfo.Args {
		args = append(args, arg.String())
	}
	argsText := strings.Join(args, ";")

	secrets := []string{}
	for key, value := range buildInfo.Secrets {
		secrets = append(secrets, fmt.Sprintf("%s=%s", key, value))
	}
	secretsText := strings.Join(secrets, ";")

	// We use a builder to avoid allocations when building the string
	var b strings.Builder
	fmt.Fprintf(&b, "build_context:%s;", sha)
	fmt.Fprintf(&b, "target:%s;", buildInfo.Target)
	fmt.Fprintf(&b, "build_args:%s;", argsText)
	fmt.Fprintf(&b, "secrets:%s;", secretsText)
	fmt.Fprintf(&b, "context:%s;", buildInfo.Context)
	fmt.Fprintf(&b, "dockerfile:%s;", buildInfo.Dockerfile)
	fmt.Fprintf(&b, "image:%s;", buildInfo.Image)
	return b.String()
}
