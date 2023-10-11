package build

type outputFormat string

const (
	// TTYFormat is the default format for the output
	TTYFormat outputFormat = "tty"

	// PlainFormat is the plain format for the output
	PlainFormat outputFormat = "plain"
)
