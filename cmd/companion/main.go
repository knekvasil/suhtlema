package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"suhtlema/internal/config"
	"suhtlema/internal/llm"
	"suhtlema/internal/servers"
	"suhtlema/internal/stt"
)

const systemPrompt = "Eres un compañero de conversación en español. " +
	"Responde en español, en 1-2 frases cortas y naturales. " +
	"No expliques, solo conversa."

func usage() {
	fmt.Fprint(os.Stderr, `suhtlema — local Spanish conversation companion

usage:
  companion stt <file.wav>     transcribe Spanish audio
  companion chat <text...>     send a message to the companion
  companion turn <file.wav>    transcribe then reply (STT -> LLM)
`)
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
	ctx := context.Background()
	stop, err := servers.EnsureAll(ctx, cfg)
	if err != nil {
		fatal(err)
	}
	defer stop()

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
	case "turn":
		if len(os.Args) < 3 {
			fatal(fmt.Errorf("turn needs a wav path"))
		}
		runTurn(ctx, cfg, os.Args[2])
	default:
		usage()
		os.Exit(2)
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

func runTurn(ctx context.Context, cfg *config.Config, wav string) {
	sttClient := stt.New(cfg.WhisperEndpoint)
	llmClient := llm.New(cfg.LlamaEndpoint)

	learner, sttDur, err := sttClient.Transcribe(ctx, wav)
	if err != nil {
		fatal(err)
	}
	fmt.Printf("learner: %s\n", strings.TrimSpace(learner))

	msgs := []llm.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: strings.TrimSpace(learner)},
	}
	reply, t, d, err := llmClient.Chat(ctx, msgs, 100)
	if err != nil {
		fatal(err)
	}
	fmt.Printf("companion: %s\n", strings.TrimSpace(reply))
	fmt.Printf("\nT0->T1 (stt)  : %s\n", sttDur.Round(time.Millisecond))
	fmt.Printf("T1->T2 (ttft) : %.0fms\n", t.PromptMS)
	fmt.Printf("T2->end(gen)  : %.0fms  (%.1f tok/s)\n", t.PredictedMS, t.PredictedPerSecond)
	fmt.Printf("turn wall     : %s\n", (sttDur + d).Round(time.Millisecond))
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}
