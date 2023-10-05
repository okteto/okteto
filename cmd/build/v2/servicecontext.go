package v2

import (
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
)

type serviceContextInterface interface {
	IsCleanBuildContext() bool
}

type SvcContextCleanlinessChecker struct {
	isClean bool
}

func (sc *SvcContextCleanlinessChecker) IsCleanBuildContext() bool {
	return sc.isClean
}

func newSvcContextCleanlinessChecker(buildInfo *model.BuildInfo, isCleanServiceBuild func(string) (bool, error)) *SvcContextCleanlinessChecker {
	isClean, err := isCleanServiceBuild(buildInfo.Context)
	if err != nil {
		oktetoLog.Infof("could not check if build context for service '%s' is clean: %w", buildInfo.Name, err)
	}

	return &SvcContextCleanlinessChecker{
		isClean: isClean,
	}
}
