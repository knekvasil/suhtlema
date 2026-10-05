# Project Plan — Low-Latency Language Companion (Suhtlema)

## 1. Concept

A real-time spoken language companion that feels more like a fluent person sitting next to you than an AI tutor. The core interaction is:

```
Talk  →  encounter a gap  →  ask  →  get help  →  keep talking
```

The primary engineering constraint is **conversational latency**, not model size. The goal is the best combination of quality + first-audio latency + cost on a consumer machine.

## 2. Hardware reality (verified)

Measured on the development machine, not assumed:

| | Value |
|---|---|
| Model | MacBook Air `MacBookAir10,1` |
| Chip | Apple M1, 8 cores (4P + 4E) |
| GPU | 8-core M1 GPU, Metal 3 |
| Memory | 8 GB unified |
| OS | macOS 15.6.1 |
| Disk free | 78 GB |

Two facts the plan must respect:

1. **Fanless.** Public benchmarks are on actively cooled M1 Pro/Max/M-series parts. This machine throttles under sustained load, so a cold-start benchmark is meaningless. Every model choice is validated with a 10-minute soak.
2. **~5–6 GB usable, not 8.** macOS uses 2–3 GB and the GPU wire limit caps Metal-usable memory below total RAM. All resident models must fit the remainder, and nothing else (browser, editor) may run during measurement. See ADR-0001.

## 3. Scope

### MVP (Milestone 1: sequential loop)

- Mic → VAD → STT → LLM → TTS → Speaker, **one Turn at a time**, fully sequential.
- Understand spoken Spanish, reply in Spanish, maintain short conversational context.
- Speak replies aloud. No interruptions of any kind.

### Explicitly out of scope for MVP

- Barge-in, Aside (formerly "mom mode"), streaming STT, speculative LLM start, commit/re-anchor, interruption state machine.
- Echo cancellation, cloud inference, ElevenLabs, pronunciation scoring, vision, accounts, mobile app, rich UI, fine-tuning, curriculum generation.
- Corrections, learner profile, conversation summaries, vocabulary tracking — these are Milestone 3.

## 4. Model stack

All models stay resident (reloading per Turn is fatal). Runtime choices and rationale: ADR-0002, ADR-0003.

### STT — whisper.cpp

| Model | Disk | RAM | M1 speed | Role |
|---|---|---|---|---|
| tiny multilingual | 39 MB | ~0.3 GB | ~32× RT | rejected: Spanish WER too high |
| **base multilingual** | 142 MB | ~0.4 GB | ~20× RT | **chosen** |
| small | 466 MB | ~0.85 GB | ~12× RT | fallback if WER demands |

Multilingual, not `.en`: Spanish is a core use case.

### LLM — llama.cpp

| Model | Quant | RAM | M1 speed | Role |
|---|---|---|---|---|
| Llama 3.2 1B | Q4 | ~0.9 GB | ~50 tok/s | rejected: too weak for Spanish |
| **Qwen3 4B** | Q4_K_M | ~2.7 GB | ~28 tok/s | **primary** |
| Llama 3.2 3B | Q4_K_M | ~2.2 GB | ~25–35 tok/s | memory fallback only |
| Qwen 3.5 4B | Q4 | ~2.7 GB | ~25–35 tok/s | optional newer alternative |

Qwen3 is a reasoning model: **thinking must be disabled** (`/no_think` or the non-thinking chat template) or it will wreck both latency and conversational tone. If Qwen3 4B + Whisper base + Kokoro will not co-reside within the GPU wire limit, drop the LLM to 3B — do **not** shrink Whisper.

### TTS — Kokoro-82M

- `kokoro-v1.0.fp16.onnx` (169 MB, ~0.5 GB resident, ~20× RT) via ONNX Runtime; encoder/TTS on the ANE.
- Spanish voices: `ef_dora` (female), `em_alex` (male), `em_santa`.
- **Known risk:** Kokoro's Spanish voices are trained on far less data than its English voices. Sentence-level naturalness must be verified by human listen, not assumed. This is the leading quality threat to the "natural voice" success criterion.

### Memory budget

| Component | RAM |
|---|---|
| Whisper base | ~0.4 GB |
| Qwen3 4B Q4_K_M | ~2.7 GB |
| Kokoro fp16 | ~0.5 GB |
| KV cache (2048 ctx) | ~0.5 GB |
| macOS + Go + ONNX | ~2.5 GB |
| **Total** | **~6.6 GB** |

## 5. Architecture (Milestone 1)

```
                 single Go binary
┌────────────────────────────────────────────────┐
│  malgo audio in/out                            │
│  Silero VAD v5 (ONNX, in-process)              │
│  Kokoro TTS (ONNX, in-process)                 │
│  orchestrator / metrics                        │
└───────┬───────────────────────────┬────────────┘
        │ localhost                 │ localhost
   ┌────▼─────┐               ┌─────▼──────┐
   │ whisper- │               │ llama-     │
   │ server   │               │ server     │
   │ (base ml)│               │ (Qwen3 4B) │
   └──────────┘               └────────────┘
```

- LLM + STT: persistent servers, models loaded once, **warmed with a dummy inference at startup**.
- VAD + TTS: in-process ONNX (cgo accepted) because they sit in the real-time path.
- Audio: `malgo` (miniaudio) — wraps CoreAudio, no brew dependency, exposes a flushable buffer for later M2 work.
- No browser, no WebSocket gateway in M1. The Go binary is the client.

## 6. Latency instrumentation

Measure from day one, even in the slow sequential milestone:

| Symbol | Event |
|---|---|
| T0 | last voiced audio frame |
| T1 | final STT result |
| T2 | first LLM token |
| T3 | first TTS audio sample |
| T4 | audio actually played |

Record: STT latency, LLM TTFT, LLM tokens/sec, TTS TTFA, VAD hangover, total Turn latency.

**Metric of record: T0 → T4** (last voiced frame → first audible AI sample).

| Target (M2) | Rating |
|---|---|
| < 500 ms | excellent (stretch; unreachable sequential) |
| p50 < 800 ms, p95 < 1200 ms | **the bar, for Milestone 2** |
| > 1200 ms | conversationally poor |

VAD hangover is measured as its own number; it is the largest and most-forgotten cost. In the sequential M1 there is no latency bar — the goal is a working end-to-end loop with trustworthy numbers.

### Indicative M2 budget (for later)

| Segment | Budget |
|---|---|
| VAD hangover | 150–250 ms |
| STT finalize | 100–200 ms |
| LLM TTFT | 200–400 ms |
| TTS first chunk | 60–150 ms |
| Playback start | 20–40 ms |
| **Total** | **~530–1040 ms** |

Hitting < 800 ms requires overlapping stages (streaming STT, speculative start, commit-before-speak) — that is Milestone 2, not M1.

## 7. VAD and endpointing (M1)

- **Silero VAD v5 via ONNX**, in-process, 20–30 ms frames.
- Start of speech: ~3 consecutive voiced frames.
- **Hangover 500 ms** (deliberately generous for M1; tighten toward 250 ms in M2).
- Minimum utterance 250 ms; hard stop 30 s.
- No backchannel or barge-in discrimination: any sustained speech is a Turn.

## 8. Context and generation policy (M1)

System prompt (minimal):

> Eres un compañero de conversación en español. Responde en español, en 1–2 frases cortas y naturales. No expliques, solo conversa. No uses razonamiento interno (`/no_think`).

- History: sliding window of the last ~10 Turns, hard-capped at 2048 tokens; drop oldest first.
- Response cap: `max_tokens ≈ 100`.
- The LLM sees **only** system prompt + raw recent Turns. No corrections, no learner profile, no summary.

## 9. Milestones

### Milestone 0 — Model benchmark

Establish what this specific machine does. **Vendor benches plus a soak, no bespoke harness:**

- `llama-bench` → pp/tg tok/s per model/quant/context.
- `whisper-bench` → RTF; plus a fixed Spanish audio set for WER.
- Kokoro timing script → RTFA + audio dumps for a human quality listen.
- **10-minute continuous soak** per model, recording tok/s at minute 1/5/10.

**Go/no-go:** Qwen3 4B holds ≥ ~15 tok/s after the soak, and Whisper base + Qwen3 4B + Kokoro co-reside without swapping.

Deliverable: benchmark table (below).

### Milestone 1 — Local sequential conversation

Mic → VAD → STT → LLM → TTS → Speaker, one Turn at a time, with full T0–T4 instrumentation.

Deliverable: a 5-minute spoken Spanish conversation with recorded latency numbers. No latency bar yet.

### Milestone 2 — Streaming

Add streaming STT (stable-prefix emit), speculative LLM start, commit-before-speak, chunked TTS, immediate playback, then barge-in with the interrupt state machine. Headphones required (ADR-0004).

Deliverable: T0 → T4 comfortably under the p50 < 800 ms bar.

### Milestone 3 — Companion behaviour

Corrections, vocabulary lookup, grammar questions, learner profile, conversation summary, English/Spanish switching, correction modes.

Deliverable: natural 15–20 minute conversation.

### Milestone 4 — Benchmark against alternatives

Run identical conversations locally and (optionally, later) against cloud GPU or API. Compare latency, quality, cost. **Cloud and ElevenLabs are out of scope while everything stays on-machine.**

Deliverable: architecture decision.

### Milestone 5 — Pronunciation

Phoneme alignment, acoustic analysis, prosody, targeted feedback. Only after conversation is good.

## 10. Benchmark matrix

| Configuration | STT | LLM | TTS | TTFA | p50 T0→T4 | Quality |
|---|---|---|---|---|---|---|
| A | base ml | Llama 3.2 3B | Kokoro fp16 | — | — | — |
| B | base ml | **Qwen3 4B** | Kokoro fp16 | — | — | — |
| C | small | Qwen3 4B | Kokoro fp16 | — | — | — |

(Rows for Llama 3.2 1B, ElevenLabs, and cloud configurations are removed: 1B cannot hold Spanish conversation, and nothing leaves the machine.)

## 11. Risks

| Risk | Impact | Mitigation |
|---|---|---|
| Fanless thermal throttling | tok/s collapses after minutes | 10-min soak in M0; keep context and replies small |
| Kokoro Spanish voice quality | "natural voice" goal unmet | human listen in M0; it is the fallback to revisit, not the default assumption |
| Model co-residency / GPU wire limit | swap, latency spikes | measured budget (~6.6 GB); drop LLM to 3B before shrinking Whisper |
| Qwen3 thinking mode | latency + tone | disable thinking explicitly; verify in M0 |
| Whisper partial cost (M2) | starves LLM on shared GPU | Whisper encoder on ANE; stable-prefix emit |
| Commit/re-anchor mismatch rate (M2) | wasted prefill, or wrong speech | generate-early/commit-before-speak; fall back to waiting for final if mismatch is high |
| Echo without AEC | barge-in false triggers | headphones mandated for MVP (ADR-0004) |

## 12. Definition of success

The project succeeds if you can forget you are talking to a pipeline. The test is not "is the model impressive?" but "can I have a 20-minute conversation without becoming annoyed by the latency or the interaction?" Latency is the primary engineering metric.

The first thing to build is not the application — it is the answer to: **can an 8 GB fanless M1 Air run Whisper + a 4B LLM + Kokoro quickly enough to feel like a conversation?** Milestone 0 answers that. Everything else follows.
