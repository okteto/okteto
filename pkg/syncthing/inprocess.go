package syncthing

import (
	"fmt"

	"github.com/okteto/okteto/pkg/log"
	"github.com/syncthing/syncthing/lib/events"
	"github.com/syncthing/syncthing/lib/locations"
	"github.com/syncthing/syncthing/lib/syncthing"
)

func (s *Syncthing) initSyncthingApp() error {
	log.Info("start syncthing in process")

	if err := locations.SetBaseDir(locations.ConfigBaseDir, s.Home); err != nil {
		return fmt.Errorf("failed to set the syncthing directory: %w", err)
	}

	cert, err := syncthing.LoadOrGenerateCertificate(
		locations.Get(locations.CertFile),
		locations.Get(locations.KeyFile),
	)

	if err != nil {
		return fmt.Errorf("failed to load/generate certificate: %w", err)
	}

	evLogger := events.NewLogger()
	go evLogger.Serve()
	defer evLogger.Stop()

	cfg, err := syncthing.LoadConfigAtStartup(locations.Get(locations.ConfigFile), cert, evLogger, false, true)
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
		Verbose:   false,
	}
	s.app = syncthing.New(cfg, ldb, evLogger, cert, opt)
	return nil
}

func (s *Syncthing) inprocessStop() error {
	if e := s.app.Stop(syncthing.ExitSuccess); e != syncthing.ExitSuccess {
		return fmt.Errorf("unexpected status: %v", e)
	}

	if e := s.app.Wait(); e != syncthing.ExitSuccess {
		return fmt.Errorf("unexpected status after wait: %v", e)
	}

	s.app.Start()
	return nil
}

func (s *Syncthing) inprocessRestart() error {
	log.Infof("restarting syncthing in process")
	if err := s.inprocessStop(); err != nil {
		return err
	}

	if err := s.initSyncthingApp(); err != nil {
		return err
	}

	log.Infof("restarted syncthing in process")
	return nil
}
