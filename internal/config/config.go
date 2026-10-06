package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// Config holds the paths and endpoints for the local inference stack.
type Config struct {
	Root string

	LlamaServer   string
	WhisperServer string

	LlamaModel   string
	WhisperModel string

	LlamaEndpoint   string
	WhisperEndpoint string
}

const (
	llamaHost   = "127.0.0.1"
	llamaPort   = 8080
	whisperHost = "127.0.0.1"
	whisperPort = 8081
)

// Load resolves the repository root and the model/binaries within it.
func Load() (*Config, error) {
	root := os.Getenv("SUHTLEMA_ROOT")
	if root == "" {
		found, err := findRoot()
		if err != nil {
			return nil, err
		}
		root = found
	}

	c := &Config{
		Root:          root,
		LlamaServer:   filepath.Join(root, "vendor/llama.cpp/build/bin/llama-server"),
		WhisperServer: filepath.Join(root, "vendor/whisper.cpp/build/bin/whisper-server"),
		LlamaModel:    filepath.Join(root, "models/llm/Qwen3-4B-Q4_K_M.gguf"),
		WhisperModel:  filepath.Join(root, "models/whisper/ggml-base.bin"),
	}
	c.LlamaEndpoint = fmt.Sprintf("http://%s:%d", llamaHost, llamaPort)
	c.WhisperEndpoint = fmt.Sprintf("http://%s:%d", whisperHost, whisperPort)
	return c, nil
}

func findRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "vendor/llama.cpp")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("could not find project root (set SUHTLEMA_ROOT)")
		}
		dir = parent
	}
}
