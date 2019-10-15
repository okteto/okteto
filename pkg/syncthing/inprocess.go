package syncthing

import (
	"crypto/tls"
	"fmt"

	"github.com/okteto/okteto/pkg/log"
	"github.com/syncthing/syncthing/lib/api"
	"github.com/syncthing/syncthing/lib/connections"
	"github.com/syncthing/syncthing/lib/events"
	"github.com/syncthing/syncthing/lib/locations"
	"github.com/syncthing/syncthing/lib/model"
	"github.com/syncthing/syncthing/lib/sha256"
	"github.com/syncthing/syncthing/lib/syncthing"
)

func (s *Syncthing) inprocessStart() error {
	log.Info("start syncthing inprocess")

	if err := locations.SetBaseDir(locations.ConfigBaseDir, s.Home); err != nil {
		return fmt.Errorf("failed to set the syncthing directory: %w", err)
	}

	cert, err := tls.LoadX509KeyPair(
		locations.Get(locations.CertFile),
		locations.Get(locations.KeyFile),
	)

	if err != nil {
		return fmt.Errorf("failed to load certificate: %w", err)
	}

	cfgPath := locations.Get(locations.ConfigFile)
	cfg, err := syncthing.LoadConfigAtStartup(cfgPath, cert, events.NoopLogger, false, true)
	if err != nil {
		return fmt.Errorf("failed to initialize config: %w", err)
	}

	dbFile := locations.Get(locations.Database)
	ldb, err := syncthing.OpenGoleveldb(dbFile, cfg.Options().DatabaseTuning)
	if err != nil {
		return fmt.Errorf("error opening database: %w", err)
	}

	opt := syncthing.Options{
		NoUpgrade: true,
		Verbose:   true,
	}

	overrideSyncthingLogging()
	s.app = syncthing.New(cfg, ldb, events.NoopLogger, cert, opt)
	s.app.Start()
	return nil
}

func overrideSyncthingLogging() {
	l := log.GetLog()
	api.SetLogger(l)
	connections.SetLogger(l)
	model.SetLogger(l)
	syncthing.SetLogger(l)
	sha256.SetLogger(l)
}

func (s *Syncthing) inprocessStop() error {
	if e := s.app.Stop(syncthing.ExitSuccess); e != syncthing.ExitSuccess {
		return fmt.Errorf("unexpected status: %v", e)
	}

	if e := s.app.Wait(); e != syncthing.ExitSuccess {
		return fmt.Errorf("unexpected status after wait: %v", e)
	}

	return nil
}

func (s *Syncthing) inprocessRestart() error {
	log.Infof("restarting syncthing in process")
	if err := s.inprocessStop(); err != nil {
		return err
	}

	if err := s.inprocessStart(); err != nil {
		return err
	}

	log.Infof("restarted syncthing in process")
	return nil
}
