package v2

type fakeServiceContext struct {
	isClean bool
}

func (sc *fakeServiceContext) IsCleanBuildContext() bool {
	return sc.isClean
}
