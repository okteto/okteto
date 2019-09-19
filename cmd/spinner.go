package cmd

import (
	"fmt"
	"runtime"
	"time"

	sp "github.com/briandowns/spinner"
)

type spinner struct {
	sp *sp.Spinner
}

func newSpinner(suffix string) *spinner {
	s := sp.New(sp.CharSets[14], 100*time.Millisecond)
	s.Suffix = fmt.Sprintf(" %s", suffix)
	return &spinner{
		sp: s,
	}
}

func (p *spinner) start() {
	if runtime.GOOS == "windows" {
		fmt.Printf(" %s\n", p.sp.Suffix)
		return
	}

	p.sp.Start()
}

func (p *spinner) stop() {
	if runtime.GOOS == "windows" {
		return
	}

	p.sp.Stop()
}

func (p *spinner) update(text string) {
	p.sp.Suffix = fmt.Sprintf(" %s", text)
}
