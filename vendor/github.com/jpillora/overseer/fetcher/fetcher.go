package fetcher

import "io"

// Interface defines the required fetcher functions
type Interface interface {
	//Init should perform validation on fields. For
	//example, ensure the appropriate URLs or keys
	//are defined or ensure there is connectivity
	//to the appropriate web service.
	Init() error
	//Fetch should check if there is an updated
	//binary to fetch, and then stream it back the
	//form of an io.Reader. If io.Reader is nil,
	//then it is assumed there are no updates. Fetch
	//will be run repeatedly and forever. It is up the
	//implementation to throttle the fetch frequency.
	Fetch() (io.Reader, error)
}

// Func converts a fetch function into the fetcher interface
func Func(fn func() (io.Reader, error)) Interface {
	return &fetcher{fn}
}

type fetcher struct {
	fn func() (io.Reader, error)
}

func (f fetcher) Init() error {
	return nil //skip
}

func (f fetcher) Fetch() (io.Reader, error) {
	return f.fn()
}
