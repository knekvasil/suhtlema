package audio

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"os"
	"sync"
	"time"

	"github.com/gen2brain/malgo"
)

func debugf(format string, args ...any) {
	if os.Getenv("SUHTLEMA_DEBUG") == "1" {
		fmt.Fprintf(os.Stderr, "[vad] "+format+"\n", args...)
	}
}

const (
	SampleRate      = 16000
	FramesPerBuffer = 512                // ~32 ms
	preRollFrames   = 16                 // ~512 ms captured before onset fires
	primeFrames     = 12                 // idle frames (~384 ms) before onset allowed
	startFrames     = 3                  // ~96 ms of speech to start
	Hangover        = 500 * time.Millisecond
	MinDuration     = 250 * time.Millisecond
	MaxDuration     = 30 * time.Second

	// Adaptive noise-gate constants.
	minOnsetLevel     = 0.018 // absolute RMS the signal must clear to start
	onsetFloorRatio   = 3.0   // onset level = max(minOnsetLevel, floor*ratio)
	offsetFloorRatio  = 1.6   // ending = signal below floor*ratio
	floorAttackAlpha  = 0.20  // how fast the floor tracks falling level
	floorDecayAlpha   = 0.002 // how slowly the floor tracks rising level
)

// durationOf converts a frame count to a wall duration at SampleRate.
func durationOf(frames int) time.Duration {
	return time.Duration(frames) * time.Second / SampleRate
}

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
		active        bool
		startIdx      int
		onsetFrames   int // consecutive buffers above onset
		silentSamples int // samples below offset while active
		runSamples    int // samples since activation
		floor         float64
		idleFrames    int
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
				// Track a slow noise floor while idle.
				detailed.idleFrames++
				if detailed.floor == 0 {
					detailed.floor = energy
				} else if energy < detailed.floor {
					detailed.floor += floorAttackAlpha * (energy - detailed.floor)
				} else {
					detailed.floor += floorDecayAlpha * (energy - detailed.floor)
				}

				onset := energy >= minOnsetLevel && energy >= onsetFloorRatio*detailed.floor
				if onset && detailed.idleFrames >= primeFrames {
					detailed.onsetFrames++
					if detailed.onsetFrames == startFrames {
						debugf("onset fired (energy=%.4f floor=%.4f idle=%d)", energy, detailed.floor, detailed.idleFrames)
					}
					if detailed.onsetFrames >= startFrames {
						detailed.active = true
						detailed.onsetFrames = 0
						detailed.silentSamples = 0
						detailed.runSamples = 0
						detailed.startIdx = len(buf)
						debugf("ACTIVE (energy=%.4f floor=%.4f)", energy, detailed.floor)
					}
				} else {
					detailed.onsetFrames = 0
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
			detailed.runSamples += n
			offset := offsetFloorRatio * detailed.floor
			if offset < minOnsetLevel/2 {
				offset = minOnsetLevel / 2
			}
			if energy < offset {
				detailed.silentSamples += n
				if durationOf(detailed.silentSamples) >= Hangover/4 {
					debugf("silent=%.0fms run=%.2fs (energy=%.4f floor=%.4f)",
						durationOf(detailed.silentSamples).Seconds()*1000,
						durationOf(detailed.runSamples).Seconds(), energy, detailed.floor)
				}
			} else {
				detailed.silentSamples = 0
			}

			speech := durationOf(detailed.runSamples)
			silent := durationOf(detailed.silentSamples)
			if (silent >= Hangover && speech >= MinDuration) || speech >= MaxDuration {
				finished = time.Now()
				debugf("END (speech=%.2fs silent=%.0fms)", speech.Seconds(), silent.Seconds()*1000)
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
	return out, durationOf(n), end, nil
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
