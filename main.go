package main

import (
	"os"

	log "github.com/sirupsen/logrus"

	"github.com/cloudnativedevelopment/cnd/cmd"
)

func main() {
	// Output to stdout instead of the default stderr
	// Can be any io.Writer, see below for File example
	log.SetOutput(os.Stdout)

	// Only log the warning severity or above.
	log.SetLevel(log.WarnLevel)

	cmd.Execute()
}
