package ksync

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"

	"github.com/vapor-ware/ksync/pkg/cli"
	"github.com/vapor-ware/ksync/pkg/debug"
	"github.com/vapor-ware/ksync/pkg/syncthing"
)

type Syncthing struct {
	cmd *exec.Cmd
}

func NewSyncthing() *Syncthing {
	return &Syncthing{}
}

func (s *Syncthing) String() string {
	return debug.YamlString(s)
}

// Fields returns a set of structured fields for logging.
func (s *Syncthing) Fields() log.Fields {
	return debug.StructFields(s)
}

func (s *Syncthing) errHandler(logger func(...interface{})) error {
	stderr, err := s.cmd.StderrPipe()
	if err != nil {
		return err
	}

	scanner := bufio.NewScanner(stderr)

	go func() {
		for scanner.Scan() {
			logger(scanner.Text())
		}
	}()

	return nil
}

func (s *Syncthing) lineHandler(logger func(...interface{})) error {
	stdout, err := s.cmd.StdoutPipe()
	if err != nil {
		return err
	}

	scanner := bufio.NewScanner(stdout)

	go func() {
		for scanner.Scan() {
			line := scanner.Text()
			logger(line)
		}
	}()

	return nil
}

func (s *Syncthing) initLogs() error {
	logger := log.WithFields(log.Fields{
		"name": "syncthing",
	})

	if err := s.errHandler(logger.Warn); err != nil {
		return err
	}

	return s.lineHandler(logger.Debug)
}

func (s *Syncthing) binPath() string {
	return filepath.Join(cli.ConfigPath(), "bin", "syncthing")
}

func (s *Syncthing) HasBinary() bool {
	if _, err := os.Stat(s.binPath()); err != nil {
		return false
	}

	return true
}

// TODO: Not sure this should be here at all. Just kinda convenient since
// binPath() is.
func (s *Syncthing) Fetch() error {
	return syncthing.Fetch(s.binPath())
}

func (s *Syncthing) resetState() error {
	base := filepath.Join(cli.ConfigPath(), "syncthing")
	if err := os.RemoveAll(base); err != nil {
		return err
	}

	return syncthing.ResetConfig(filepath.Join(base, "config.xml"))
}

func (s *Syncthing) Run() error {
	if !s.HasBinary() {
		return fmt.Errorf("missing pre-requisites, run init to fix")
	}

	if err := s.resetState(); err != nil {
		return err
	}

	path := filepath.Join(
		filepath.Dir(viper.ConfigFileUsed()), "bin", "syncthing")

	address := fmt.Sprintf("localhost:%d", viper.GetInt("syncthing-port"))

	cmdArgs := []string{
		"-gui-address", address,
		"-gui-apikey", viper.GetString("apikey"),
		"-home", filepath.Join(filepath.Dir(viper.ConfigFileUsed()), "syncthing"),
		"-no-browser",
	}

	s.cmd = exec.Command(path, cmdArgs...)

	if err := s.initLogs(); err != nil {
		return err
	}

	if err := s.cmd.Start(); err != nil {
		return err
	}

	log.WithFields(log.Fields{
		"cmd":  s.cmd.Path,
		"args": s.cmd.Args,
	}).Debug("starting syncthing")

	return nil
}

// Stop halts the background process and cleans up.
func (s *Syncthing) Stop() error {
	defer s.cmd.Process.Wait() //nolint: errcheck
	return s.cmd.Process.Kill()
}
