package ssh

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/creack/pty"
	"github.com/gliderlabs/ssh"
	"github.com/google/uuid"
	"github.com/pkg/sftp"
	log "github.com/sirupsen/logrus"
)

var (
	// ErrEOF is the error when the terminal exits
	ErrEOF = errors.New("EOF")
)

// Server holds the ssh server configuration
type Server struct {
	Port           int
	Shell          string
	AuthorizedKeys []ssh.PublicKey
}

func getExitStatusFromError(err error) int {
	if err == nil {
		return 0
	}

	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		return 1
	}

	waitStatus, ok := exitErr.Sys().(syscall.WaitStatus)
	if !ok {
		if exitErr.Success() {
			return 0
		}

		return 1
	}

	return waitStatus.ExitStatus()
}

func setWinsize(f *os.File, w, h int) {
	syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), uintptr(syscall.TIOCSWINSZ),
		uintptr(unsafe.Pointer(&struct{ h, w, x, y uint16 }{uint16(h), uint16(w), 0, 0})))
}

func handlePTY(logger *log.Entry, cmd *exec.Cmd, s ssh.Session, ptyReq ssh.Pty, winCh <-chan ssh.Window) error {
	if len(ptyReq.Term) > 0 {
		cmd.Env = append(cmd.Env, fmt.Sprintf("TERM=%s", ptyReq.Term))
	}

	f, err := pty.Start(cmd)
	if err != nil {
		logger.WithError(err).Error("failed to start pty session")
		return err
	}

	go func() {
		for win := range winCh {
			setWinsize(f, win.Width, win.Height)
		}
	}()

	go func() {
		io.Copy(f, s) // stdin
	}()

	waitCh := make(chan struct{})
	go func() {
		defer close(waitCh)
		io.Copy(s, f) // stdout
	}()

	if err := cmd.Wait(); err != nil {
		logger.WithError(err).Errorf("pty command failed while waiting")
		return err
	}

	select {
	case <-waitCh:
		logger.Info("stdout finished")
	case <-time.NewTicker(1 * time.Second).C:
		logger.Info("stdout didn't finish after 1s")
	}

	return nil
}

func sendErrAndExit(logger *log.Entry, s ssh.Session, err error) {
	msg := strings.TrimPrefix(err.Error(), "exec: ")
	if _, err := s.Stderr().Write([]byte(msg)); err != nil {
		logger.WithError(err).Errorf("failed to write error back to session")
	}

	if err := s.Exit(getExitStatusFromError(err)); err != nil {
		logger.WithError(err).Errorf("pty session failed to exit")
	}
}

func handleNoTTY(logger *log.Entry, cmd *exec.Cmd, s ssh.Session) error {
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		logger.WithError(err).Errorf("couldn't get StdoutPipe")
		return err
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		logger.WithError(err).Errorf("couldn't get StderrPipe")
		return err
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		logger.WithError(err).Errorf("couldn't get StdinPipe")
		return err
	}

	if err = cmd.Start(); err != nil {
		logger.WithError(err).Errorf("couldn't start command '%s'", cmd.String())
		return err
	}

	go func() {
		defer stdin.Close()
		if _, err := io.Copy(stdin, s); err != nil {
			logger.WithError(err).Errorf("failed to write session to stdin.")
		}
	}()

	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		if _, err := io.Copy(s, stdout); err != nil {
			logger.WithError(err).Errorf("failed to write stdout to session.")
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		if _, err := io.Copy(s.Stderr(), stderr); err != nil {
			logger.WithError(err).Errorf("failed to write stderr to session.")
		}
	}()

	wg.Wait()

	if err := cmd.Wait(); err != nil {
		logger.WithError(err).Errorf("command failed while waiting")
		return err
	}

	return nil
}

func (srv *Server) connectionHandler(s ssh.Session) {
	sessionID := uuid.New().String()
	logger := log.WithFields(log.Fields{"session.id": sessionID})
	defer func() {
		s.Close()
		logger.Info("session closed")
	}()

	logger.Infof("starting ssh session with command '%+v'", s.RawCommand())

	cmd := srv.buildCmd(s)

	if ssh.AgentRequested(s) {
		logger.Info("agent requested")
		l, err := ssh.NewAgentListener()
		if err != nil {
			logger.WithError(err).Error("failed to start agent")
			return
		}

		defer l.Close()
		go ssh.ForwardAgentConnections(l, s)
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", "SSH_AUTH_SOCK", l.Addr().String()))
	}

	ptyReq, winCh, isPty := s.Pty()
	if isPty {
		logger.Println("handling PTY session")
		if err := handlePTY(logger, cmd, s, ptyReq, winCh); err != nil {
			sendErrAndExit(logger, s, err)
			return
		}

		s.Exit(0)
		return
	}

	logger.Println("handling non PTY session")
	if err := handleNoTTY(logger, cmd, s); err != nil {
		sendErrAndExit(logger, s, err)
		return
	}

	s.Exit(0)
}

// LoadAuthorizedKeys loads path as an array.
// It will return nil if path doesn't exist.
func LoadAuthorizedKeys(path string) ([]ssh.PublicKey, error) {
	authorizedKeysBytes, err := ioutil.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}

		return nil, err
	}

	authorizedKeys := []ssh.PublicKey{}
	for len(authorizedKeysBytes) > 0 {
		pubKey, _, _, rest, err := ssh.ParseAuthorizedKey(authorizedKeysBytes)
		if err != nil {
			return nil, err
		}

		authorizedKeys = append(authorizedKeys, pubKey)
		authorizedKeysBytes = rest
	}

	if len(authorizedKeys) == 0 {
		return nil, fmt.Errorf("%s was empty", path)
	}

	return authorizedKeys, nil
}

func (srv *Server) authorize(ctx ssh.Context, key ssh.PublicKey) bool {
	for _, k := range srv.AuthorizedKeys {
		if ssh.KeysEqual(key, k) {
			return true
		}
	}

	log.Println("access denied")
	return false
}

// ListenAndServe starts the SSH server using port
func (srv *Server) ListenAndServe() error {
	server := srv.getServer()
	return server.ListenAndServe()
}

func (srv *Server) getServer() *ssh.Server {
	forwardHandler := &ssh.ForwardedTCPHandler{}

	server := &ssh.Server{
		Addr:    fmt.Sprintf(":%d", srv.Port),
		Handler: srv.connectionHandler,
		ChannelHandlers: map[string]ssh.ChannelHandler{
			"direct-tcpip": ssh.DirectTCPIPHandler,
			"session":      ssh.DefaultSessionHandler,
		},
		LocalPortForwardingCallback: ssh.LocalPortForwardingCallback(func(ctx ssh.Context, dhost string, dport uint32) bool {
			log.Println("Accepted forward", dhost, dport)
			return true
		}),
		ReversePortForwardingCallback: ssh.ReversePortForwardingCallback(func(ctx ssh.Context, host string, port uint32) bool {
			log.Println("attempt to bind", host, port, "granted")
			return true
		}),
		RequestHandlers: map[string]ssh.RequestHandler{
			"tcpip-forward":        forwardHandler.HandleSSHRequest,
			"cancel-tcpip-forward": forwardHandler.HandleSSHRequest,
		},
		SubsystemHandlers: map[string]ssh.SubsystemHandler{
			"sftp": sftpHandler,
		},
	}

	server.SetOption(ssh.HostKeyPEM([]byte(hostKeyBytes)))

	if srv.AuthorizedKeys != nil {
		server.PublicKeyHandler = srv.authorize
	}

	return server
}

func sftpHandler(sess ssh.Session) {
	debugStream := ioutil.Discard
	serverOptions := []sftp.ServerOption{
		sftp.WithDebug(debugStream),
	}
	server, err := sftp.NewServer(
		sess,
		serverOptions...,
	)
	if err != nil {
		log.Printf("sftp server init error: %s\n", err)
		return
	}
	if err := server.Serve(); err == io.EOF {
		server.Close()
		log.Println("sftp client exited session.")
	} else if err != nil {
		log.Println("sftp server completed with error:", err)
	}
}

func (srv Server) buildCmd(s ssh.Session) *exec.Cmd {
	var cmd *exec.Cmd

	if len(s.RawCommand()) == 0 {
		cmd = exec.Command(srv.Shell)
	} else {
		args := []string{"-c", s.RawCommand()}
		cmd = exec.Command(srv.Shell, args...)
	}

	cmd.Env = append(cmd.Env, os.Environ()...)
	cmd.Env = append(cmd.Env, s.Environ()...)

	fmt.Println(cmd.String())
	return cmd
}
