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
	stateDim   = 64
)

// Silero wraps the Silero VAD v4 ONNX session (separate h/c LSTM state).
type Silero struct {
	mu  sync.Mutex
	ses *ort.AdvancedSession
	in  *ort.Tensor[float32]
	h   *ort.Tensor[float32]
	c   *ort.Tensor[float32]
	sr  *ort.Tensor[int64]
	out *ort.Tensor[float32]
	hn  *ort.Tensor[float32]
	cn  *ort.Tensor[float32]
	hst []float32
	cst []float32
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

// New loads the Silero VAD v4 model into its own session.
func New(modelPath string) (*Silero, error) {
	if err := envInit(); err != nil {
		return nil, fmt.Errorf("onnxruntime init: %w", err)
	}
	s := &Silero{hst: make([]float32, 2*stateDim), cst: make([]float32, 2*stateDim)}

	var created []ort.Value
	cleanup := func() {
		for _, v := range created {
			_ = v.Destroy()
		}
		if s.ses != nil {
			_ = s.ses.Destroy()
		}
	}
	set := func(t **ort.Tensor[float32], dims ...int64) error {
		var err error
		*t, err = ort.NewTensor(ort.NewShape(dims...), make([]float32, prod64(dims)))
		if err != nil {
			cleanup()
			return err
		}
		created = append(created, *t)
		return nil
	}

	if err := set(&s.in, 1, window); err != nil {
		return nil, fmt.Errorf("vad tensor: %w", err)
	}
	if err := set(&s.h, 2, 1, stateDim); err != nil {
		return nil, fmt.Errorf("vad tensor: %w", err)
	}
	if err := set(&s.c, 2, 1, stateDim); err != nil {
		return nil, fmt.Errorf("vad tensor: %w", err)
	}
	if err := set(&s.out, 1, 1); err != nil {
		return nil, fmt.Errorf("vad tensor: %w", err)
	}
	if err := set(&s.hn, 2, 1, stateDim); err != nil {
		return nil, fmt.Errorf("vad tensor: %w", err)
	}
	if err := set(&s.cn, 2, 1, stateDim); err != nil {
		return nil, fmt.Errorf("vad tensor: %w", err)
	}

	var err error
	if s.sr, err = ort.NewTensor(ort.NewShape(1), []int64{sampleRate}); err != nil {
		cleanup()
		return nil, fmt.Errorf("vad sr tensor: %w", err)
	}
	created = append(created, s.sr)

	s.ses, err = ort.NewAdvancedSession(
		modelPath,
		[]string{"input", "h", "c", "sr"},
		[]string{"output", "hn", "cn"},
		[]ort.Value{s.in, s.h, s.c, s.sr},
		[]ort.Value{s.out, s.hn, s.cn},
		nil,
	)
	if err != nil {
		cleanup()
		return nil, fmt.Errorf("vad session: %w", err)
	}
	return s, nil
}

// Prob scores a 512-sample window (normalized ~[-1,1]) and advances state.
func (s *Silero) Prob(frame []float32) (float32, error) {
	if len(frame) != window {
		return 0, fmt.Errorf("vad expects %d samples, got %d", window, len(frame))
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	copy(s.in.GetData(), frame)
	copy(s.h.GetData(), s.hst)
	copy(s.c.GetData(), s.cst)
	s.sr.GetData()[0] = sampleRate
	if err := s.ses.Run(); err != nil {
		return 0, err
	}
	p := s.out.GetData()[0]
	copy(s.hst, s.hn.GetData())
	copy(s.cst, s.cn.GetData())
	return p, nil
}

// Reset clears the LSTM state.
func (s *Silero) Reset() {
	for i := range s.hst {
		s.hst[i] = 0
		s.cst[i] = 0
	}
}

// Close releases tensors and the session. The environment is process-lifetime.
func (s *Silero) Close() {
	for _, v := range []ort.Value{s.in, s.h, s.c, s.sr, s.out, s.hn, s.cn} {
		if v != nil {
			_ = v.Destroy()
		}
	}
	if s.ses != nil {
		_ = s.ses.Destroy()
	}
}

func prod64(dims []int64) int {
	p := 1
	for _, d := range dims {
		p *= int(d)
	}
	return p
}
