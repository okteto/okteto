package monitor

import (
	"context"
	"fmt"
	"os/exec"
	"sync"
	"time"

	"github.com/okteto/tools/supervisor/pkg/setup"
	log "github.com/sirupsen/logrus"
)

// Monitor makes sure the list of processes is running
type Monitor struct {
	ps              []*Process
	err             chan error
	ctx             context.Context
	syncthingConfig SyncthingConfig
}

// SyncthingConfig represents the syncthing configuration
type SyncthingConfig struct {
	config string
	data   string
	secret string
}

// NewSyncthingConfig returns a new syncthing config
func NewSyncthingConfig(config, secret, data string) SyncthingConfig {
	return SyncthingConfig{
		config: config,
		data:   data,
		secret: secret,
	}
}

// NewMonitor returns an initialized monitor
func NewMonitor(ctx context.Context, syncthingConfig SyncthingConfig) *Monitor {
	return &Monitor{
		ctx:             ctx,
		err:             make(chan error, 1),
		syncthingConfig: syncthingConfig,
	}
}

// Add adds a process to monitor
func (m *Monitor) Add(p *Process) {
	if p == nil {
		return
	}

	if m.ps == nil {
		m.ps = []*Process{}
	}

	m.ps = append(m.ps, p)
}

// Run starts all the process of the monitor and checks the state of the process
func (m *Monitor) Run() error {
	if m.ps == nil {
		return fmt.Errorf("processes are not set")
	}

	t := time.NewTicker(1 * time.Second)
	for {
		select {
		case e := <-m.err:
			return e
		case <-m.ctx.Done():
			log.Info("context cancelled, exiting monitor")
			return nil
		default:
			wg := &sync.WaitGroup{}
			for _, p := range m.ps {
				wg.Add(1)
				go m.checkState(p, wg)
			}

			wg.Wait()
			<-t.C
		}
	}
}

func (m *Monitor) checkState(p *Process, wg *sync.WaitGroup) {
	defer wg.Done()
	switch p.state {
	case neverStarted:
		p.logger.Printf("starting %s for the first time", p.Name)
		if p.Path == SyncthingBin {
			if err := setup.Setup(m.syncthingConfig.secret, m.syncthingConfig.config); err != nil {
				p.logger.Errorf("failed to setup %s: %s", p.Name, err.Error())
				m.err <- fmt.Errorf("failed to setup %s: %w", p.Name, err)
				return
			}
		}
		p.start()
		p.logger.Printf("%s started 1", p.Name)
	case stopped:
		if !p.shouldStart() {
			log.Errorf("%s started %d times", p.Name, p.startCount)
			m.err <- fmt.Errorf("%s started %d times", p.Name, p.startCount)
			return
		}

		p.killAllByName()

		log.Infof("Restarting process %s", p.Name)
		if p.Path == SyncthingBin {
			if err := setup.Setup(m.syncthingConfig.secret, m.syncthingConfig.config); err != nil {
				p.logger.Printf("failed to setup %s: %s", p.Name, err.Error())
				m.err <- fmt.Errorf("failed to setup %s: %w", p.Name, err)
				return
			}
		}
		p.start()
		p.logger.Printf("%s started 2", p.Name)
	case fatal:
		if !p.shouldStart() {
			m.err <- fmt.Errorf("%s started %d times", p.Name, p.startCount)
			return
		}

		p.killAllByName()

		if p.Path == SyncthingBin {
			log.Info("Resetting syncthing database after error...")
			if err := setup.Setup(m.syncthingConfig.secret, m.syncthingConfig.config); err != nil {
				p.logger.Errorf("failed to setup %s: %s", p.Name, err.Error())
				m.err <- fmt.Errorf("failed to setup %s: %w", p.Name, err)
				return
			}

			log.Info("resetting syncthing database")
			cmd := exec.Command(SyncthingBin, "debug", "--config", m.syncthingConfig.config, "--data", m.syncthingConfig.data, "reset-database")
			output, err := cmd.CombinedOutput()
			if err != nil {
				log.WithError(err).Errorf("error resetting syncthing database: %s", output)
			} else {
				log.Infof("syncthing database reset: %s", output)
			}
		}

		log.Errorf("Restarting process %s after failure", p.Name)
		p.start()
		p.logger.Printf("%s started 3", p.Name)
	case started:
		//do nothing
	default:
		log.Errorf("unexpected state: %s", p.state)
	}
}

// Stop stops all the processes
func (m *Monitor) Stop() {
	wg := &sync.WaitGroup{}
	for _, p := range m.ps {
		wg.Add(1)
		go func(ps *Process) {
			defer wg.Done()
			ps.stop()
		}(p)
	}

	wg.Wait()
}
