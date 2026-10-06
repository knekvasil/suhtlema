# Suhtlema

A spoken language companion. The Learner holds a conversation with the
Companion in a target language, and the Companion answers aloud. Everything
runs on the machine: speech recognition, language model, and speech synthesis
are all local, with no cloud calls.

## How it works

A single Go binary orchestrates everything. Three inference pieces run as
resident child processes; two run in-process.

```
mic and speaker (malgo / CoreAudio, 16 kHz)
    |
    v
Silero VAD v4 (ONNX, in-process)   ends capture on a half-second pause
    |
    v
whisper-server (Whisper base multilingual, localhost:8081)
    |
    v
llama-server (Qwen3.5 4B Q4_K_M, Metal, localhost:8080, OpenAI-compatible API)
    |
    v
Kokoro TTS daemon (kokoro-onnx, Python subprocess, voice ef_dora, 24 kHz)
    |
    v
speaker
```

- STT and LLM run as persistent servers from llama.cpp and whisper.cpp. They
  load their models once and stay warm for the whole session.
- VAD runs in-process on onnxruntime.
- TTS runs as a warm Python daemon (`bench/scripts/tts_daemon.py`) that keeps
  the Kokoro voice resident and streams raw PCM back over stdout.
- Every Turn is instrumented with T0 to T4 timings: last voiced audio frame,
  final STT result, first LLM token, first TTS audio sample, audio actually
  played.

## Repository layout

```
cmd/companion         the CLI entry point
internal/audio        mic capture and playback (malgo), recording exports
internal/vad          Silero VAD v4 on onnxruntime
internal/stt          whisper-server client
internal/llm          llama-server chat client (OpenAI-compatible)
internal/tts          warm Kokoro daemon client
internal/servers      spawn and supervise llama-server and whisper-server
internal/config       paths, endpoints, and env overrides
bench/scripts         model fetch, toolchain build, and benchmark harnesses
bench/results         measured numbers; SUMMARY.md is the index
models                model weights (downloaded, not tracked)
third_party           cloned and built llama.cpp and whisper.cpp
```

## Requirements

- macOS on Apple Silicon (developed on an M1 Air, 8 GB).
- Homebrew.
- Go 1.24 or newer.
- CMake (installed by the build script when missing).
- onnxruntime shared library for the in-process VAD:
  `brew install onnxruntime` (or set `ONNXRUNTIME_LIBRARY_PATH`).
- A Python 3.12 venv at `bench/.venv` with `kokoro-onnx` and `numpy` for the
  Kokoro TTS daemon.

## Setup

Clone and build the inference toolchains with Metal support:

```
bench/scripts/build-toolchains.sh
```

Download the model weights (LLM GGUFs, Whisper GGML models, Kokoro voices):

```
bench/scripts/fetch-models.sh
```

The app expects weights at these paths (`internal/config/config.go`):

- LLM: `models/llm/Qwen3.5-4B-Q4_K_M.gguf`
- Whisper: `models/whisper/ggml-base.bin`
- Kokoro: `models/tts/kokoro-v1.0.fp16.onnx` and `models/tts/voices-v1.0.bin`
- VAD: `models/tts/silero_vad.onnx`

Build the Companion binary:

```
go build -o bin/companion ./cmd/companion
```

## Usage

```
companion converse     live spoken conversation, one Turn at a time
companion listen       record one utterance, then speak a reply aloud
companion turn `<wav>` transcribe a file's speech, then reply out loud (full Turn)
companion stt `<wav>`  transcribe Spanish audio from a file
companion chat <text>  send a message to the Companion (no audio)
companion say <text>   speak text aloud via TTS
companion record       capture one utterance and play it back (mic check)
companion monitor      live RMS meter for the mic, 8s (VAD diagnosis)
companion vadtest `<wav>` run Silero over a 16 kHz wav and report probabilities
```

`converse`, `listen`, `turn`, `stt`, `chat`, and `say` auto-start and stop the
model servers. Capture and reply WAVs land in `/tmp` for inspection. The
Companion speaks Spanish and replies in 1 to 3 sentences. Diagnostics print the
T0 to T4 timings per Turn.

## Configuration

| Variable | Meaning |
|---|---|
| `SUHTLEMA_ROOT` | repository root; overrides automatic discovery |
| `SUHTLEMA_HANGOVER_MS` | VAD pause duration that ends capture, default 500 ms |
| `SUHTLEMA_DEBUG` | set to `1` for VAD debug tracing |
| `ONNXRUNTIME_LIBRARY_PATH` | path to `libonnxruntime.dylib` for the in-process VAD |

## Benchmarks

`bench/scripts/` and `bench/results/` hold the reasoning and raw numbers behind
the model choices; `bench/results/SUMMARY.md` indexes them. Highlights, all
measured on an M1 MacBook Air with 8 GB of RAM:

- STT: Whisper base multilingual, warm latency about 187 ms, about 12 to 15
  times real time.
- LLM: Qwen3.5 4B Q4_K_M, about 18 tok/s sustained after a 10-minute soak.
- TTS: Kokoro `ef_dora`, about 4 times real time, natural voice.
- Co-residency (all three models resident): PASS. About 2.1 GB resident set
  size, zero swap, about a 3 second LLM round-trip. Runtime overhead is roughly
  2.1 GB on top of macOS; a browser will not fit on an 8 GB machine.

The measuring scripts are `llm-bench.sh`, `stt-bench.py`, `residency.sh`,
`soak.sh`, and the TTS timing scripts, all under `bench/scripts/`.