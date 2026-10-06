package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"suhtlema/internal/audio"
	"suhtlema/internal/config"
	"suhtlema/internal/llm"
	"suhtlema/internal/servers"
	"suhtlema/internal/stt"
	"suhtlema/internal/tts"
	"suhtlema/internal/vad"
)

const systemPrompt = "Eres un compañero de conversación en español, cercano y natural. " +
	"Responde en español en 1-3 frases. Varía tus respuestas: a veces pregunta " +
	"de vuelta, a veces comenta algo, y evita repetir la misma pregunta (" +
	"\"¿Y tú?\") con demasiada frecuencia. Si el usuario mezcla inglés, " +
	"responde igual en español. No expliques ni corrijas, solo conversa."

func usage() {
	fmt.Fprint(os.Stderr, `suhtlema — local Spanish conversation companion

usage:
  companion stt <file.wav>      transcribe Spanish audio
  companion chat <text...>      send a message to the companion
  companion say <text...>       speak text aloud via TTS
  companion record             capture one utterance and play it back (mic check)
  companion monitor            live RMS meter for the mic, 8s (VAD diagnosis)
  companion vadtest <wav>      run Silero over a 16kHz wav and report probabilities
  companion listen              record one utterance from the mic, then speak a reply
  companion converse            live spoken conversation, one turn at a time
  companion turn <file.wav>     transcribe then reply out loud (full turn)
`)
}

var exitHooks []func()

// fatal runs exit hooks (killing model servers) before exiting — plain
// os.Exit skips defers and would leave orphaned processes behind.
func fatal(err error) {
	for _, h := range exitHooks {
		h()
	}
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}

func needsServers(cmd string) bool {
	switch cmd {
	case "stt", "chat", "say", "turn", "listen", "converse":
		return true
	}
	return false
}

func needsTTS(cmd string) bool {
	switch cmd {
	case "say", "turn", "listen", "converse":
		return true
	}
	return false
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	cfg, err := config.Load()
	if err != nil {
		fatal(err)
	}
	audio.SetVADModel(cfg.VADModel)
	ctx := context.Background()

	if needsServers(os.Args[1]) {
		stop, err := servers.EnsureAll(ctx, cfg)
		if err != nil {
			fatal(err)
		}
		exitHooks = append(exitHooks, stop)
	}

	var voice *tts.Piper
	if needsTTS(os.Args[1]) {
		voice, err = tts.Start(cfg.TTSModel, cfg.TTSPython, cfg.TTSScript, cfg.TTSVoices, cfg.TTSVoice, cfg.TTSSampleRate)
		if err != nil {
			fatal(err)
		}
		exitHooks = append(exitHooks, voice.Stop)
	}

	switch os.Args[1] {
	case "stt":
		if len(os.Args) < 3 {
			fatal(fmt.Errorf("stt needs a wav path"))
		}
		runSTT(ctx, cfg, os.Args[2])
	case "chat":
		if len(os.Args) < 3 {
			fatal(fmt.Errorf("chat needs some text"))
		}
		runChat(ctx, cfg, strings.Join(os.Args[2:], " "))
	case "say":
		if len(os.Args) < 3 {
			fatal(fmt.Errorf("say needs some text"))
		}
		runSay(voice, strings.Join(os.Args[2:], " "), "/tmp/suhtlema-say.wav")
	case "record":
		runRecord(ctx)
	case "monitor":
		if err := audio.Monitor(8 * time.Second); err != nil {
			fatal(err)
		}
	case "vadtest":
		if len(os.Args) < 3 {
			fatal(fmt.Errorf("vadtest needs a wav path"))
		}
		runVADTest(cfg, os.Args[2])
	case "listen":
		runListen(ctx, cfg, voice)
	case "converse":
		runConverse(ctx, cfg, voice)
	case "turn":
		if len(os.Args) < 3 {
			fatal(fmt.Errorf("turn needs a wav path"))
		}
		runTurn(ctx, cfg, voice, os.Args[2])
	default:
		usage()
		os.Exit(2)
	}

	// Normal exit: run hooks (kills model servers) even without an error.
	for _, h := range exitHooks {
		h()
	}
}

func runSTT(ctx context.Context, cfg *config.Config, wav string) {
	client := stt.New(cfg.WhisperEndpoint)
	text, d, err := client.Transcribe(ctx, wav)
	if err != nil {
		fatal(err)
	}
	fmt.Printf("text: %s\n", strings.TrimSpace(text))
	fmt.Printf("stt:  %s\n", d.Round(time.Millisecond))
}

func runChat(ctx context.Context, cfg *config.Config, text string) {
	client := llm.New(cfg.LlamaEndpoint)
	msgs := []llm.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: text},
	}
	reply, t, d, err := client.Chat(ctx, msgs, 100)
	if err != nil {
		fatal(err)
	}
	fmt.Printf("companion: %s\n", strings.TrimSpace(reply))
	fmt.Printf("llm:  TTFT %.0fms  %.1f tok/s  (%s wall)\n",
		t.PromptMS, t.PredictedPerSecond, d.Round(time.Millisecond))
}

func runSay(voice *tts.Piper, text, out string) {
	samples, sr, synth, err := synth(voice, text)
	if err != nil {
		fatal(err)
	}
	if err := writeWAV(out, samples, sr); err != nil {
		fatal(err)
	}
	fmt.Printf("said:  %s\n", strings.TrimSpace(text))
	fmt.Printf("tts:   %s  (%.2fs audio, %d Hz) -> %s\n",
		synth.Round(time.Millisecond), float64(len(samples))/float64(sr), sr, out)
	start, _ := playBlocking(out) // audible output completes the loop
	fmt.Printf("T3->T4 (play) : %s  (to audible start)\n", start.Round(time.Millisecond))
}

// runVADTest scores a 16 kHz mono WAV with Silero so the Go plumbing can be
// verified without a microphone.
func runVADTest(cfg *config.Config, wav string) {
	samples, sr, err := readWAV(wav)
	if err != nil {
		fatal(err)
	}
	if sr != 16000 {
		fatal(fmt.Errorf("vadtest needs 16000 Hz, got %d", sr))
	}
	detector, err := vad.New(cfg.VADModel)
	if err != nil {
		fatal(err)
	}
	defer detector.Close()
	detector.Reset()

	var nSpeech, nTotal int
	var maxP float32
	early := []string{}
	window := make([]float32, audio.FramesPerBuffer)
	for i := 0; i+audio.FramesPerBuffer <= len(samples); i += audio.FramesPerBuffer {
		for j := 0; j < audio.FramesPerBuffer; j++ {
			window[j] = float32(samples[i+j]) / 32768.0
		}
		p, err := detector.Prob(window)
		if err != nil {
			fatal(err)
		}
		nTotal++
		if p > 0.5 {
			nSpeech++
		}
		if p > maxP {
			maxP = p
		}
		if nTotal <= 10 {
			early = append(early, fmt.Sprintf("%.2f", p))
		}
	}
	fmt.Printf("windows: %d  >0.5: %d (%.0f%%)  max: %.3f\n", nTotal, nSpeech, float64(nSpeech)*100/float64(nTotal), maxP)
	fmt.Printf("first 10: %s\n", strings.Join(early, " "))
}

// readWAV loads a PCM WAV (mono or multi-channel) as int16 samples.
func readWAV(path string) ([]int16, int, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, 0, err
	}
	if len(b) < 44 || string(b[:4]) != "RIFF" {
		return nil, 0, fmt.Errorf("not a RIFF file")
	}
	channels := int(binary.LittleEndian.Uint16(b[22:24]))
	rate := int(binary.LittleEndian.Uint32(b[24:28]))
	bits := int(binary.LittleEndian.Uint16(b[34:36]))
	dataStart := bytes.Index(b, []byte("data"))
	if dataStart < 0 {
		return nil, 0, fmt.Errorf("no data chunk")
	}
	dataStart += 8
	raw := b[dataStart:]
	if bits != 16 {
		return nil, 0, fmt.Errorf("vadtest supports 16-bit PCM, got %d", bits)
	}
	out := make([]int16, 0, len(raw)/2/channels)
	for i := 0; i+2*channels <= len(raw); i += 2 * channels {
		var sum int32
		for ch := 0; ch < channels; ch++ {
			sum += int32(int16(binary.LittleEndian.Uint16(raw[i+2*ch:])))
		}
		out = append(out, int16(sum/int32(channels)))
	}
	return out, rate, nil
}

func runListen(ctx context.Context, cfg *config.Config, voice *tts.Piper) {
	fmt.Println("Escuchando... (termina con una pausa de medio segundo)")
	recCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	samples, dur, _, err := audio.Record(recCtx)
	if err != nil {
		fatal(err)
	}
	wav := "/tmp/suhtlema-capture.wav"
	if err := audio.WriteWAV(wav, samples, audio.SampleRate); err != nil {
		fatal(err)
	}
	fmt.Printf("captured: %.2fs -> %s\n", dur.Seconds(), wav)
	runTurn(ctx, cfg, voice, wav)
}

func runRecord(ctx context.Context) {
	fmt.Println("Habla... (mic check)")
	recCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	samples, dur, _, err := audio.Record(recCtx)
	if err != nil {
		fatal(err)
	}
	wav := "/tmp/suhtlema-record.wav"
	if err := audio.WriteWAV(wav, samples, audio.SampleRate); err != nil {
		fatal(err)
	}
	fmt.Printf("captured %.2fs  rms=%.3f  -> %s\n", dur.Seconds(), audio.RMS(samples), wav)
	_, _ = playBlocking(wav) // hear what the mic actually heard
}

func runConverse(ctx context.Context, cfg *config.Config, voice *tts.Piper) {
	sttClient := stt.New(cfg.WhisperEndpoint)
	llmClient := llm.New(cfg.LlamaEndpoint)
	history := []llm.Message{{Role: "system", Content: systemPrompt}}

	reply := "¡Hola! ¿Qué tal?"
	if _, err := speak(voice, reply, "/tmp/suhtlema-greet.wav"); err != nil {
		fatal(err)
	}
	history = append(history, llm.Message{Role: "assistant", Content: reply})

	for {
		fmt.Println("\nEscuchando... (termina con una pausa de medio segundo)")
		recCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
		samples, dur, speechEnd, err := audio.Record(recCtx)
		cancel()
		if err != nil {
			fatal(err)
		}
		if err := audio.WriteWAV("/tmp/suhtlema-capture.wav", samples, audio.SampleRate); err != nil {
			fatal(err)
		}
		fmt.Printf("captured %.2fs  rms=%.3f\n", dur.Seconds(), audio.RMS(samples))
		if audio.RMS(samples) < 0.015 && dur < 3*time.Second {
			fmt.Println("(no oí nada claro — repite por favor)")
			continue
		}

		learner, _, err := sttClient.Transcribe(ctx, "/tmp/suhtlema-capture.wav")
		if err != nil {
			fatal(err)
		}
		learner = strings.TrimSpace(learner)
		if learner == "" {
			_ = audio.WriteWAV("/tmp/suhtlema-unheard.wav", samples, audio.SampleRate)
			fmt.Println("(no oí nada — repite por favor; saved to /tmp/suhtlema-unheard.wav)")
			continue
		}
		fmt.Printf("learner:   %s\n", learner)

		history = append(history, llm.Message{Role: "user", Content: learner})
		reply, _, _, err = llmClient.Chat(ctx, history, 100)
		if err != nil {
			fatal(err)
		}
		reply = strings.TrimSpace(reply)
		history = append(history, llm.Message{Role: "assistant", Content: reply})
		history = trimHistory(history)

		fmt.Printf("companion: %s\n", reply)
		playStart, err := speak(voice, reply, "/tmp/suhtlema-reply.wav")
		if err != nil {
			fatal(err)
		}
		fmt.Printf("T0 (speech end) -> T4 (audible) : %s\n",
			time.Now().Add(playStart).Sub(speechEnd).Round(time.Millisecond))
	}
}

// trimHistory keeps the system message plus the most recent turns.
func trimHistory(h []llm.Message) []llm.Message {
	const maxMessages = 13 // system + 6 turns
	if len(h) <= maxMessages {
		return h
	}
	out := make([]llm.Message, 0, maxMessages)
	out = append(out, h[0]) // system
	out = append(out, h[len(h)-maxMessages+1:]...)
	return out
}

// speak synthesises text aloud, returning time to audible start.
func speak(voice *tts.Piper, text, out string) (time.Duration, error) {
	samples, sr, _, err := synth(voice, text)
	if err != nil {
		return 0, err
	}
	if err := writeWAV(out, samples, sr); err != nil {
		return 0, err
	}
	toStart, _ := playBlocking(out)
	return toStart, nil
}

func runTurn(ctx context.Context, cfg *config.Config, voice *tts.Piper, wav string) {
	sttClient := stt.New(cfg.WhisperEndpoint)
	llmClient := llm.New(cfg.LlamaEndpoint)

	t0 := time.Now()
	learner, sttDur, err := sttClient.Transcribe(ctx, wav)
	if err != nil {
		fatal(err)
	}
	t1 := t0.Add(sttDur)
	fmt.Printf("learner: %s\n", strings.TrimSpace(learner))

	msgs := []llm.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: strings.TrimSpace(learner)},
	}
	reply, t, llmWall, err := llmClient.Chat(ctx, msgs, 100)
	if err != nil {
		fatal(err)
	}
	t2 := t1.Add(llmWall)
	fmt.Printf("companion: %s\n", strings.TrimSpace(reply))

	samples, sr, synthDur, err := synth(voice, strings.TrimSpace(reply))
	if err != nil {
		fatal(err)
	}
	t3 := t2.Add(synthDur)
	out := "/tmp/suhtlema-turn.wav"
	if err := writeWAV(out, samples, sr); err != nil {
		fatal(err)
	}

	playStart, playDuration := playBlocking(out)
	t4 := t3.Add(playStart)

	fmt.Printf("\nT0->T1 (stt)   : %s\n", sttDur.Round(time.Millisecond))
	fmt.Printf("T1->T2 (llm)   : %s  (TTFT %.0fms, %.1f tok/s)\n",
		llmWall.Round(time.Millisecond), t.PromptMS, t.PredictedPerSecond)
	fmt.Printf("T2->T3 (tts)   : %s  (%.2fs audio)\n", synthDur.Round(time.Millisecond), float64(len(samples))/float64(sr))
	fmt.Printf("T3->T4 (play)  : %s  (to audible start; then playing %.2fs)\n",
		playStart.Round(time.Millisecond), playDuration.Seconds())
	fmt.Printf("T0->T4 (total) : %s  (end of speech -> first audible AI)\n", t4.Sub(t0).Round(time.Millisecond))
}

// playBlocking plays audio aloud via afplay. Returns (time to audible start,
// playback duration) and blocks until playback finishes so the servers stay
// warm while the user listens.
func playBlocking(path string) (time.Duration, time.Duration) {
	cmd := exec.Command("afplay", path)
	start := time.Now()
	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "warn: playback failed: %v\n", err)
		return time.Since(start), 0
	}
	toStart := time.Since(start)
	_ = cmd.Wait()
	return toStart, time.Since(start) - toStart
}

// synth voices text and reports synthesis wall time.
func synth(voice *tts.Piper, text string) ([]int16, int, time.Duration, error) {
	start := time.Now()
	samples, err := voice.Synth(text)
	if err != nil {
		return nil, 0, 0, err
	}
	return samples, voice.SampleRate, time.Since(start), nil
}

// writeWAV writes int16 mono PCM as a 16 kHz WAV file.
func writeWAV(path string, samples []int16, sr int) error {
	var buf bytes.Buffer
	data := make([]byte, len(samples)*2)
	for i, s := range samples {
		binary.LittleEndian.PutUint16(data[i*2:], uint16(s))
	}
	byteRate := sr * 2
	buf.WriteString("RIFF")
	_ = binary.Write(&buf, binary.LittleEndian, uint32(36+len(data)))
	buf.WriteString("WAVEfmt ")
	_ = binary.Write(&buf, binary.LittleEndian, uint32(16))
	_ = binary.Write(&buf, binary.LittleEndian, uint16(1)) // PCM
	_ = binary.Write(&buf, binary.LittleEndian, uint16(1)) // mono
	_ = binary.Write(&buf, binary.LittleEndian, uint32(sr))
	_ = binary.Write(&buf, binary.LittleEndian, uint32(byteRate))
	_ = binary.Write(&buf, binary.LittleEndian, uint16(2))  // block align
	_ = binary.Write(&buf, binary.LittleEndian, uint16(16)) // bits per sample
	buf.WriteString("data")
	_ = binary.Write(&buf, binary.LittleEndian, uint32(len(data)))
	buf.Write(data)
	return os.WriteFile(path, buf.Bytes(), 0o644)
}
