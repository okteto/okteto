package main

import (
	"cli/cnd/pkg/config"
	"cli/cnd/pkg/log"
	"os"

	"github.com/sirupsen/logrus"

	"cli/cmd"
	cnd "cli/cnd/cmd"
)

// VersionString the version of the cli
var VersionString string

func init() {
	config.VersionString = VersionString
	config.SetConfig(&config.Config{
		AnalyticsEndpoint:   "https://us-central1-okteto-prod.cloudfunctions.net/oktetocli-analytics",
		CNDFolderName:       ".okteto",
		CNDManifestFileName: "okteto.yml",
	})
}

func main() {
	log.Init(logrus.WarnLevel, cnd.GetActionID())
	cnd.Register(cmd.Diagnostics)
	os.Exit(cnd.Execute())
}
