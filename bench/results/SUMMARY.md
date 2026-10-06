# Benchmark summary

All numbers measured on the M1 MacBook Air (8 GB, fanless), macOS 15.6.1.
Raw data: `llm-bench.md`, `soak-*.csv`, `tts-kokoro.md`, `stt-bench.md`, `residency.md`.

## Per-stage results

| Stage | Choice | Speed | Warm latency | Quality |
|---|---|---|---|---|
| STT | Whisper **base** multilingual | ~12–15× RT (2–3 s clips) | **~187 ms** | 3.6% WER (clean TTS audio) |
| STT | Whisper small | ~4–6× RT | ~511 ms — **rejected: too slow** | 1.4% WER |
| STT | Whisper tiny | ~19–23× RT | ~159 ms | 2.1% WER; weaker on real speech |
| LLM | **Qwen3 4B Q4_K_M** | **19.1 tok/s** sustained (min 16.7) | prefill ~180 tok/s | best Spanish |
| LLM | Llama 3.2 3B Q4_K_M | 23.9 tok/s sustained (min 20.8) | prefill ~238 tok/s | weaker Spanish; memory fallback |
| TTS | **Kokoro `ef_dora`** (fp16, ONNX) | **~4× RT** (CPU ONNX) | ~264 ms for a ~1 s utterance | natural voice; Latin American accent |

Note: Piper's `es_ES-carlfm-x_low` (Castilian, ~45× RT) replaced Kokoro for the accent, but **glitched and cut out during conversation**; quality won over accent and TTS is back on **Kokoro `ef_dora`** (ADR-0005 superseded). Kokoro measures ~4× RT — the ~20× figure the plan assumed was MLX on an M1 Max — which is fine for short replies in the sequential milestone.

## Configuration comparison

| Configuration | STT | LLM | TTS | Sustained LLM | STT warm | TTS RTF |
|---|---|---|---|---|---|---|
| A | base | Llama 3.2 3B | Kokoro ef_dora | 23.9 tok/s | 187 ms | ~4× |
| **B** | base | **Qwen3 4B** | Kokoro ef_dora | **19.1 tok/s** | 187 ms | ~4× |
| C | small | Qwen3 4B | Kokoro ef_dora | 19.1 tok/s | 511 ms | ~4× |

**Recommendation: Configuration B.** B is chosen over A for Spanish quality, and C's STT improvement (3.6% → 1.4% WER on *clean* audio) is not worth tripling STT latency — and that WER gap will shrink on real speech anyway.

## Sequential end-to-end estimate

Summing measured stages for one turn (500 ms VAD hangover + ~187 ms STT + LLM prefill over a growing context + first TTS clause):

```
~2.0–2.8 s   end of speech → first audible audio   (sequential M1)
```

Dominated by LLM prefill and generation. This is expected and acceptable for the sequential milestone; the streaming/speculation work is what brings it under the p50 < 800 ms bar.

## Go/no-go

| Gate | Result |
|---|---|
| Qwen3 4B ≥ ~15 tok/s after 10-min soak | ✅ **PASS** — 16.7–21.4 tok/s, no collapse |
| Whisper base + Qwen3.5 4B + Kokoro co-reside | ✅ **PASS** (clean reboot: 24% free, **0 swap**, ~2.1 GB RSS — LLM 1682 MB + Whisper 177 MB + Kokoro 263 MB; LLM 18.3 tok/s, STT 301 ms). The earlier "at the edge" reading (14% free, ~4 GB swap) was a dirty, unrebooted box. |

**Verdict: proceed to M1.** The stack is viable on this hardware. No open items.
