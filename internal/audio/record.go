package audio

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"math"
	"os"
	"sync"
	"time"

	"github.com/gen2brain/malgo"
)

const (
	SampleRate      = 16000
	FramesPerBuffer = 512 // ~32 ms
	frameDur        = time.Duration(FramesPerBuffer) * time.Second / SampleRate
	preRollFrames   = 16 // ~512 ms captured before onset fires
	startThreshold  = 0.020
	endThreshold    = 0.015
	startFrames     = 3 // ~96 ms of speech to start
	Hangover        = 500 * time.Millisecond
	MinDuration     = 250 * time.Millisecond
	MaxDuration     = 30 * time.Second
)

// ErrNoSpeech is returned when the microphone never crossed the onset
// threshold.
var ErrNoSpeech = errors.New("no speech detected")

// Record captures a single utterance from the default microphone, stopping
// after a hanging end-of-speech. Returns the utterance samples, its duration,
// and the wall-clock moment end-of-speech was detected (T0).
//
// VAD is a simple energy endpoint detector for now; Silero replaces it later.
func Record(ctx context.Context) (samples []int16, dur time.Duration, speechEnd time.Time, err error) {
	mctx, err := malgo.InitContext(nil, malgo.ContextConfig{}, nil)
	if err != nil {
		return nil, 0, time.Time{}, err
	}
	defer mctx.Uninit()

	cfg := malgo.DefaultDeviceConfig(malgo.Capture)
	cfg.Capture.Format = malgo.FormatS16
	cfg.Capture.Channels = 1
	cfg.SampleRate = SampleRate
	cfg.PeriodSizeInFrames = FramesPerBuffer

	var (
		mu       sync.Mutex
		buf      []int16
		done     = make(chan struct{})
		finished time.Time
	)
	detailed := struct {
		active       bool
		startIdx     int
		speechFrames int
		silentFrames int
	}{}

	callbacks := malgo.DeviceCallbacks{
		Data: func(_, input []byte, framecount uint32) {
			n := int(framecount)
			frames := make([]int16, n)
			for i := 0; i < n; i++ {
				j := i * 2
				frames[i] = int16(input[j]) | int16(input[j+1])<<8
			}
			energy := rms(frames)

			mu.Lock()
			defer mu.Unlock()
			if !detailed.active {
				if energy >= startThreshold {
					detailed.speechFrames++
					if detailed.speechFrames >= startFrames {
						detailed.active = true
						detailed.speechFrames = 0
						detailed.silentFrames = 0
						detailed.startIdx = len(buf)
					}
				} else {
					detailed.speechFrames = 0
				}
				buf = append(buf, frames...)
				// Keep idle capture to a short pre-roll.
				cut := preRollFrames * n
				if len(buf) > cut {
					buf = append([]int16(nil), buf[len(buf)-cut:]...)
				}
				return
			}

			buf = append(buf, frames...)
			if energy < endThreshold {
				detailed.silentFrames++
			} else {
				detailed.silentFrames = 0
			}

			speech := time.Duration(len(buf)-detailed.startIdx) * frameDur
			silent := time.Duration(detailed.silentFrames) * frameDur
			if (silent >= Hangover && speech >= MinDuration) || speech >= MaxDuration {
				finished = time.Now()
				close(done)
			}
		},
	}

	dev, err := malgo.InitDevice(mctx.Context, cfg, callbacks)
	if err != nil {
		return nil, 0, time.Time{}, err
	}
	defer dev.Uninit()
	if err := dev.Start(); err != nil {
		return nil, 0, time.Time{}, err
	}
	defer dev.Stop()

	select {
	case <-done:
	case <-ctx.Done():
		return nil, 0, time.Time{}, ctx.Err()
	case <-time.After(60 * time.Second):
		return nil, 0, time.Time{}, ErrNoSpeech
	}

	mu.Lock()
	out := append([]int16(nil), buf[detailed.startIdx:]...)
	n := len(out)
	end := finished
	mu.Unlock()
	return out, time.Duration(n) * frameDur, end, nil
}

// WriteWAV writes int16 mono PCM as a WAV file.
func WriteWAV(path string, samples []int16, sampleRate int) error {
	var buf bytes.Buffer
	data := make([]byte, len(samples)*2)
	for i, s := range samples {
		binary.LittleEndian.PutUint16(data[i*2:], uint16(s))
	}
	byteRate := sampleRate * 2
	buf.WriteString("RIFF")
	_ = binary.Write(&buf, binary.LittleEndian, uint32(36+len(data)))
	buf.WriteString("WAVEfmt ")
	_ = binary.Write(&buf, binary.LittleEndian, uint32(16))
	_ = binary.Write(&buf, binary.LittleEndian, uint16(1))
	_ = binary.Write(&buf, binary.LittleEndian, uint16(1))
	_ = binary.Write(&buf, binary.LittleEndian, uint32(sampleRate))
	_ = binary.Write(&buf, binary.LittleEndian, uint32(byteRate))
	_ = binary.Write(&buf, binary.LittleEndian, uint16(2))
	_ = binary.Write(&buf, binary.LittleEndian, uint16(16))
	buf.WriteString("data")
	_ = binary.Write(&buf, binary.LittleEndian, uint32(len(data)))
	buf.Write(data)
	return os.WriteFile(path, buf.Bytes(), 0o644)
}

func rms(frames []int16) float64 {
	if len(frames) == 0 {
		return 0
	}
	var sum float64
	for _, s := range frames {
		x := float64(s) / 32768.0
		sum += x * x
	}
	return math.Sqrt(sum / float64(len(frames)))
}

// RMS reports the overall normalized root-mean-square level of a signal
// (0..1). Useful for diagnosing VAD sensitivity.
func RMS(samples []int16) float64 {
	return rms(samples)
}
