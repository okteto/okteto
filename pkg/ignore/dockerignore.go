package ignore

import (
	"os"

	"github.com/moby/patternmatcher"
	"github.com/moby/patternmatcher/ignorefile"
)

type DockerIgnorer struct {
	patterns []string
}

func (di *DockerIgnorer) Ignore(filePath string) (bool, error) {
	return patternmatcher.Matches(filePath, di.patterns)
}

func NewDockerIgnorerFromFileOrNoop(dockerignoreFile string) *DockerIgnorer {
	di := &DockerIgnorer{patterns: []string{}}
	f, err := os.Open(dockerignoreFile)
	if err != nil {
		return di
	}
	defer f.Close()

	p, err := ignorefile.ReadAll(f)
	if err == nil {
		di.patterns = p
	}
	return di
}
