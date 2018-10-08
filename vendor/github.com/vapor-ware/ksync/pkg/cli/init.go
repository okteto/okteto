package cli

import (
	"os"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

// InitLogging initializes the logging engine with the configured values.
func InitLogging() {
	log.SetOutput(os.Stdout)

	lvl, err := log.ParseLevel(viper.GetString("log-level"))
	// TODO: is log the right way to output failures, should it be print?
	if err != nil {
		log.Fatalf("Invalid log level: %s", viper.GetString("log-level"))
	}
	log.SetLevel(lvl)
}
