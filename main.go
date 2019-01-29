package main

import (
	"github.com/Sirupsen/logrus"
	"github.com/cloudnativedevelopment/cnd/cmd"
	"github.com/cloudnativedevelopment/cnd/pkg/log"
)

func main() {
	log.Init(logrus.WarnLevel)
	cmd.Execute()
}
