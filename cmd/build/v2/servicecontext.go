package v2

type serviceContextInterface interface {
	IsCleanBuildContext() bool
	GetBuildHash() string
}

type serviceContextChecker struct {
	isClean bool
	hash    string
}

func (sc serviceContextChecker) IsCleanBuildContext() bool {
	return sc.isClean
}

func (sc serviceContextChecker) GetBuildHash() string {
	return sc.hash
}
