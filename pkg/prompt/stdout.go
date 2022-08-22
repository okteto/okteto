package prompt

import (
	"os"

	"github.com/chzyer/readline"
)

type stdout struct{}

// Write implements an io.WriterCloser over os.Stderr, but it skips the terminal
// bell character.
func (*stdout) Write(b []byte) (int, error) {
	if len(b) == 1 && b[0] == readline.CharBell {
		return 0, nil
	}
	return os.Stderr.Write(b)
}

// Close implements an io.WriterCloser over os.Stderr.
func (*stdout) Close() error {
	return os.Stderr.Close()
}
