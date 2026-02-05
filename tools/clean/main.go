package main

import (
	"os"
	"path/filepath"

	"github.com/shirou/gopsutil/v3/process"
	log "github.com/sirupsen/logrus"
)

// CommitString is the commit used to build the server
var CommitString string

var exceptByParent = map[string]struct{}{
	"screen":       {},
	"tmux: server": {},
	"tmux":         {},
}

var exceptByName = map[string]struct{}{
	"okteto-remote":     {},
	"syncthing":         {},
	"okteto-supervisor": {},
	"supervisor":        {},
}

func shouldKill(p *process.Process) bool {
	pid := int32(p.Pid)
	if pid == 1 {
		// PID 1 is always protected. With shareProcessNamespace: true, this is /pause
		// (Kubernetes infrastructure), not the container root. Container root processes
		// (supervisor) are protected via exceptByName instead.
		log.Info("not killing root process of the container")
		return false
	}

	if pid == int32(os.Getppid()) {
		log.Info("not killing parent process")
		return false
	}

	if pid == int32(os.Getpid()) {
		log.Info("not killing yourself")
		return false
	}

	exe, err := p.Exe()
	if err != nil {
		log.Warnf("failed to get executable name for PID %d: %s", pid, err)
		// Continue processing even if we can't get the executable name
	} else {
		exeName := filepath.Base(exe)
		if _, ok := exceptByName[exeName]; ok {
			log.Infof("not killing, excluded by name %s", exeName)
			return false
		}
	}

	if isChildrenOfExceptByParent(p) {
		return false
	}

	return true
}

func isChildrenOfExceptByParent(p *process.Process) bool {
	pid := int32(p.Pid)
	if pid == 1 {
		return false
	}

	exe, err := p.Exe()
	if err != nil {
		log.Warnf("failed to get executable name for PID %d: %s", pid, err)
	} else {
		exeName := filepath.Base(exe)
		if _, ok := exceptByParent[exeName]; ok {
			log.Infof("not killing, children of %s", exeName)
			return true
		}
	}

	ppid, err := p.Ppid()
	if err != nil {
		log.Errorf("fail to get parent PID for process %d : %s", pid, err)
		return false
	}

	parent, err := process.NewProcess(ppid)
	if err != nil {
		log.Errorf("fail to find process %d : %s", ppid, err)
		return false
	}

	if parent == nil {
		return false
	}

	return isChildrenOfExceptByParent(parent)
}

func main() {
	log.Infof("clean service started sha=%s pid=%d ppid=%d", CommitString, os.Getpid(), os.Getppid())
	processes, err := process.Processes()
	if err != nil {
		log.Errorf("fail to list processes: %s", err)
		os.Exit(1)
	}

	for _, p := range processes {
		exe, err := p.Exe()
		if err != nil {
			log.Warnf("failed to get executable name for PID %d: %s", p.Pid, err)
			exe = "unknown"
		} else {
			exe = filepath.Base(exe)
		}
		log.Infof("checking process %s", exe)

		if !shouldKill(p) {
			continue
		}
		log.Infof("killing process %s", exe)

		pr, err := os.FindProcess(int(p.Pid))
		if err != nil {
			log.Errorf("fail to find process %d : %s", p.Pid, err)
			continue
		}

		if err := pr.Kill(); err != nil {
			log.Errorf("fail to kill process %d : %s", p.Pid, err)
		}
	}

	log.Infof("clean service finished")
}
