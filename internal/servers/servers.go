package servers

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"time"

	"suhtlema/internal/config"
)

// Process is a managed child process (a model server).
type Process struct {
	Name string
	cmd  *exec.Cmd
}

// StartLlama launches llama-server with the configured model.
func StartLlama(cfg *config.Config) (*Process, error) {
	args := []string{
		"-m", cfg.LlamaModel,
		"-c", "2048",
		"-ngl", "99",
		"--jinja",
		"--host", "127.0.0.1",
		"--port", "8080",
	}
	return start("llama-server", cfg.LlamaServer, args, cfg.LlamaEndpoint+"/health")
}

// StartWhisper launches whisper-server with the configured model.
func StartWhisper(cfg *config.Config) (*Process, error) {
	args := []string{
		"-m", cfg.WhisperModel,
		"--host", "127.0.0.1",
		"--port", "8081",
	}
	return start("whisper-server", cfg.WhisperServer, args, cfg.WhisperEndpoint+"/")
}

func start(name, bin string, args []string, healthURL string) (*Process, error) {
	if _, err := os.Stat(bin); err != nil {
		return nil, fmt.Errorf("%s binary not found at %s (run bench/scripts/build-toolchains.sh)", name, bin)
	}
	cmd := exec.Command(bin, args...)
	if f, err := os.Create(fmt.Sprintf("/tmp/suhtlema-%s.log", name)); err == nil {
		cmd.Stdout = f
		cmd.Stderr = f
	} else {
		cmd.Stdout = os.Stderr
		cmd.Stderr = os.Stderr
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	p := &Process{Name: name, cmd: cmd}
	if err := waitHealthy(healthURL, 120*time.Second); err != nil {
		_ = p.Stop()
		return nil, fmt.Errorf("%s did not become healthy: %w", name, err)
	}
	return p, nil
}

func waitHealthy(url string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: 2 * time.Second}
	for time.Now().Before(deadline) {
		resp, err := client.Get(url)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode < 500 {
				return nil
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("timeout after %s", timeout)
}

// Stop terminates the process.
func (p *Process) Stop() error {
	if p == nil || p.cmd == nil || p.cmd.Process == nil {
		return nil
	}
	return p.cmd.Process.Kill()
}

// EnsureAll starts whichever model servers are not already healthy, and
// returns a stopper for the ones it started.
func EnsureAll(ctx context.Context, cfg *config.Config) (func(), error) {
	var started []*Process
	if !healthy(cfg.WhisperEndpoint + "/") {
		p, err := StartWhisper(cfg)
		if err != nil {
			return nil, err
		}
		started = append(started, p)
	}
	if !healthy(cfg.LlamaEndpoint + "/health") {
		p, err := StartLlama(cfg)
		if err != nil {
			for _, s := range started {
				_ = s.Stop()
			}
			return nil, err
		}
		started = append(started, p)
	}
	return func() {
		for _, p := range started {
			_ = p.Stop()
		}
	}, nil
}

func healthy(url string) bool {
	client := &http.Client{Timeout: time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode < 500
}
