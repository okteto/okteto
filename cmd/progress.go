package cmd

import (
	"fmt"
	"runtime"
	"time"

	"github.com/briandowns/spinner"
)

type progress struct {
	sp *spinner.Spinner
}

func newProgressSpinner(suffix string) *progress {
	s := spinner.New(spinner.CharSets[14], 100*time.Millisecond)
	s.Suffix = fmt.Sprintf(" %s", suffix)
	return &progress{
		sp: s,
	}
}

func (p *progress) start() {
	if runtime.GOOS == "windows" {
		fmt.Printf(" %s\n", p.sp.Suffix)
		return
	}

	p.sp.Start()
}

func (p *progress) stop() {
	if runtime.GOOS == "windows" {
		return
	}

	p.sp.Stop()
}

func (p *progress) update(text string) {
	p.sp.Suffix = fmt.Sprintf(" %s", text)
}
