package tts

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
)

// Kokoro is a warm client for the Kokoro TTS daemon.
type Kokoro struct {
	mu         sync.Mutex
	cmd        *exec.Cmd
	stdin      io.WriteCloser
	stdout     io.Reader
	SampleRate int
}

// Start launches the daemon with a voice resident.
func Start(model, python, script, voices, voice string, sampleRate int) (*Kokoro, error) {
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
	return &Kokoro{cmd: cmd, stdin: stdin, stdout: stdout, SampleRate: sampleRate}, nil
}

// Synth synthesises text and returns int16 mono PCM at SampleRate.
func (p *Kokoro) Synth(text string) ([]int16, error) {
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
func (p *Kokoro) Stop() {
	if p == nil || p.cmd == nil || p.cmd.Process == nil {
		return
	}
	_ = p.cmd.Process.Kill()
}
