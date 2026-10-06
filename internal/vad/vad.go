package vad

import (
	"fmt"
	"os"
	"runtime"
	"sync"

	ort "github.com/yalue/onnxruntime_go"
)

const (
	sampleRate = 16000
	window     = 512 // ~32 ms at 16 kHz
	stateSize  = 128
)

// Silero wraps the Silero VAD ONNX session. A single instance keeps the
// recurrent state (h, c) between windows.
type Silero struct {
	mu    sync.Mutex
	ses   *ort.AdvancedSession
	in    *ort.Tensor[float32]
	st    *ort.Tensor[float32]
	sr    *ort.Tensor[int64]
	out   *ort.Tensor[float32]
	stN   *ort.Tensor[float32]
	state []float32
}

var (
	envOnce sync.Once
	envErr  error
)

func libraryPath() string {
	if p := os.Getenv("ONNXRUNTIME_LIBRARY_PATH"); p != "" {
		return p
	}
	if runtime.GOOS == "darwin" {
		return "/opt/homebrew/lib/libonnxruntime.dylib"
	}
	return "libonnxruntime.so"
}

func envInit() error {
	envOnce.Do(func() {
		ort.SetSharedLibraryPath(libraryPath())
		envErr = ort.InitializeEnvironment()
	})
	return envErr
}

// New loads the Silero VAD model into its own session.
func New(modelPath string) (*Silero, error) {
	if err := envInit(); err != nil {
		return nil, fmt.Errorf("onnxruntime init: %w", err)
	}
	s := &Silero{state: make([]float32, 2*stateSize)}

	var created []ort.Value
	cleanup := func() {
		for _, v := range created {
			_ = v.Destroy()
		}
		if s.ses != nil {
			_ = s.ses.Destroy()
		}
	}

	var err error
	if s.in, err = ort.NewTensor(ort.NewShape(1, window), make([]float32, window)); err != nil {
		return nil, fmt.Errorf("vad input tensor: %w", err)
	}
	created = append(created, s.in)
	if s.st, err = ort.NewTensor(ort.NewShape(2, 1, stateSize), make([]float32, 2*stateSize)); err != nil {
		cleanup()
		return nil, fmt.Errorf("vad state tensor: %w", err)
	}
	created = append(created, s.st)
	if s.sr, err = ort.NewTensor(ort.NewShape(1), []int64{sampleRate}); err != nil {
		cleanup()
		return nil, fmt.Errorf("vad sr tensor: %w", err)
	}
	created = append(created, s.sr)
	if s.out, err = ort.NewTensor(ort.NewShape(1, 1), make([]float32, 1)); err != nil {
		cleanup()
		return nil, fmt.Errorf("vad output tensor: %w", err)
	}
	created = append(created, s.out)
	if s.stN, err = ort.NewTensor(ort.NewShape(2, 1, stateSize), make([]float32, 2*stateSize)); err != nil {
		cleanup()
		return nil, fmt.Errorf("vad stateN tensor: %w", err)
	}
	created = append(created, s.stN)

	s.ses, err = ort.NewAdvancedSession(
		modelPath,
		[]string{"input", "state", "sr"},
		[]string{"output", "stateN"},
		[]ort.Value{s.in, s.st, s.sr},
		[]ort.Value{s.out, s.stN},
		nil,
	)
	if err != nil {
		cleanup()
		return nil, fmt.Errorf("vad session: %w", err)
	}
	return s, nil
}

// Prob scores a 512-sample window and advances the recurrent state. Frame
// samples should be normalized to roughly [-1, 1].
func (s *Silero) Prob(frame []float32) (float32, error) {
	if len(frame) != window {
		return 0, fmt.Errorf("vad expects %d samples, got %d", window, len(frame))
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	copy(s.in.GetData(), frame)
	copy(s.st.GetData(), s.state)
	s.sr.GetData()[0] = sampleRate
	if err := s.ses.Run(); err != nil {
		return 0, err
	}
	p := s.out.GetData()[0]
	copy(s.state, s.stN.GetData())
	return p, nil
}

// Reset clears the recurrent state.
func (s *Silero) Reset() {
	for i := range s.state {
		s.state[i] = 0
	}
}

// Close releases the session and tensors. The environment is process-lifetime.
func (s *Silero) Close() {
	for _, v := range []ort.Value{s.in, s.st, s.sr, s.out, s.stN} {
		if v != nil {
			_ = v.Destroy()
		}
	}
	if s.ses != nil {
		_ = s.ses.Destroy()
	}
}
