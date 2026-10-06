package tts

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
)

// Piper is a warm client for the Piper TTS daemon.
type Piper struct {
	mu         sync.Mutex
	cmd        *exec.Cmd
	stdin      io.WriteCloser
	stdout     io.Reader
	SampleRate int
}

// SampleRate reads the voice's sample rate from its config JSON.
func SampleRate(configJSON string) (int, error) {
	var c struct {
		Audio struct {
			SampleRate int `json:"sample_rate"`
		} `json:"audio"`
	}
	f, err := os.Open(configJSON)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	if err := json.NewDecoder(f).Decode(&c); err != nil {
		return 0, err
	}
	if c.Audio.SampleRate == 0 {
		return 0, fmt.Errorf("no sample_rate in %s", filepath.Base(configJSON))
	}
	return c.Audio.SampleRate, nil
}

// Start launches the daemon with a voice resident.
func Start(model, python, script, voices, voice string, sampleRate int) (*Piper, error) {
	cmd := exec.Command(python, script, "--model", model, "--voices", voices, "--voice", voice)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return &Piper{cmd: cmd, stdin: stdin, stdout: stdout, SampleRate: sampleRate}, nil
}

// Synth synthesises text and returns int16 mono PCM at SampleRate.
func (p *Piper) Synth(text string) ([]int16, error) {
	if p == nil {
		return nil, errors.New("tts not started")
	}
	p.mu.Lock()
	defer p.mu.Unlock()

	if _, err := fmt.Fprintf(p.stdin, "%s\n", text); err != nil {
		return nil, err
	}

	var header [8]byte
	if _, err := io.ReadFull(p.stdout, header[:]); err != nil {
		return nil, err
	}
	length := binary.LittleEndian.Uint64(header[:])
	if length == 0 {
		return nil, errors.New("tts synthesis failed")
	}
	const maxPayload = 20 << 20 // 20 MiB
	if length > maxPayload || length%2 != 0 {
		return nil, fmt.Errorf("tts payload malformed: %d bytes", length)
	}
	payload := make([]byte, length)
	if _, err := io.ReadFull(p.stdout, payload); err != nil {
		return nil, err
	}
	out := make([]int16, length/2)
	for i := range out {
		out[i] = int16(binary.LittleEndian.Uint16(payload[i*2:]))
	}
	return out, nil
}

// Stop terminates the daemon.
func (p *Piper) Stop() {
	if p == nil || p.cmd == nil || p.cmd.Process == nil {
		return
	}
	_ = p.cmd.Process.Kill()
}
