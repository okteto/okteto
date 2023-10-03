package v2

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
