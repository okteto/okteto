package v2

import (
	"github.com/okteto/okteto/pkg/filesystem"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/repository"
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

func newSvcContextCleanlinessChecker(buildInfo *model.BuildInfo) *SvcContextCleanlinessChecker {
	wdCtrl := filesystem.NewOsWorkingDirectoryCtrl()
	wd, err := wdCtrl.Get()
	if err != nil {
		oktetoLog.Infof("could not get working dir: %w", err)
	}
	gitRepo := repository.NewRepository(wd)

	isClean, err := gitRepo.IsBuildContextClean(buildInfo.Context)
	if err != nil {
		oktetoLog.Infof("could not check if build context for service '%s' is clean: %w", buildInfo.Name, err)
	}

	return &SvcContextCleanlinessChecker{
		isClean: isClean,
	}
}
