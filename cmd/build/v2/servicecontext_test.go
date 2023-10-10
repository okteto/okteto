package v2

type fakeServiceContext struct {
	isClean bool
	hash    string
}

func (sc *fakeServiceContext) IsCleanBuildContext() bool {
	return sc.isClean
}

func (sc *fakeServiceContext) GetBuildHash() string {
	return sc.hash
}
