package audio

import (
	"fmt"
	"time"

	"github.com/gen2brain/malgo"
)

// Monitor streams the default microphone for a duration and prints the RMS of
// each ~200 ms window plus the running noise floor, so VAD behaviour can be
// diagnosed by eye.
func Monitor(duration time.Duration) error {
	mctx, err := malgo.InitContext(nil, malgo.ContextConfig{}, nil)
	if err != nil {
		return err
	}
	defer mctx.Uninit()

	cfg := malgo.DefaultDeviceConfig(malgo.Capture)
	cfg.Capture.Format = malgo.FormatS16
	cfg.Capture.Channels = 1
	cfg.SampleRate = SampleRate
	cfg.PeriodSizeInFrames = FramesPerBuffer

	callbacks := malgo.DeviceCallbacks{
		Data: func(_, input []byte, framecount uint32) {
			n := int(framecount)
			frames := make([]int16, n)
			for i := 0; i < n; i++ {
				j := i * 2
				frames[i] = int16(input[j]) | int16(input[j+1])<<8
			}
			e := rms(frames)
			if floor == 0 {
				floor = e
			} else if e < floor {
				floor += floorAttackAlpha * (e - floor)
			} else {
				floor += floorDecayAlpha * (e - floor)
			}
			if time.Since(lastPrint) >= 200*time.Millisecond {
				lastPrint = time.Now()
				fmt.Printf("rms=%.4f floor=%.4f would_onset=%v\n",
					e, floor, e >= minOnsetLevel && e >= onsetFloorRatio*floor)
			}
		},
	}

	dev, err := malgo.InitDevice(mctx.Context, cfg, callbacks)
	if err != nil {
		return err
	}
	defer dev.Uninit()
	fmt.Printf("negotiated sampleRate=%d  internal=%d\n", dev.SampleRate(), dev.CaptureInternalSampleRate())
	if err := dev.Start(); err != nil {
		return err
	}
	defer dev.Stop()
	<-time.After(duration)
	return nil
}

var (
	floor     float64
	lastPrint time.Time
)