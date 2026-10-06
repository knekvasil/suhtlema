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
| TTS | ~~Kokoro fp16~~ → **Piper `es_ES-carlfm-x_low`** | **~45× RT** (CPU ONNX) | ~70 ms for a 2.8 s utterance | Castilian accent; listen in `audio-castilian/` |

Note: Kokoro measured **~4× RT, not the ~20× the plan assumed** (that figure was MLX on an M1 Max), and its Spanish accent is Latin American. Piper Castilian is both correct-accent and ~45×, so it replaces Kokoro (ADR-0005) and removes the need for ANE offload.

## Configuration comparison

| Configuration | STT | LLM | TTS | Sustained LLM | STT warm | TTS RTF |
|---|---|---|---|---|---|---|
| A | base | Llama 3.2 3B | Piper carlfm | 23.9 tok/s | 187 ms | 45× |
| **B** | base | **Qwen3 4B** | Piper carlfm | **19.1 tok/s** | 187 ms | 45× |
| C | small | Qwen3 4B | Piper carlfm | 19.1 tok/s | 511 ms | 45× |

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
| Whisper base + Qwen3.5 4B + Kokoro co-reside | ✅ **FUNCTIONAL but at the edge** (unrebooted box: 14% free, swap ~4 GB/5.1 GB, LLM 14–21 tok/s). Models alone ≈ 3.4 GB + macOS 2.5+ GB ≈ 6.5/8 GB — no headroom. Re-test after a clean reboot and with other apps closed. |

**Verdict: proceed to M1.** The stack is viable on this hardware. Two open items before trusting the memory number:

1. Re-run `bench/scripts/residency.sh` immediately after a reboot with nothing else open.
2. **Listen** to `bench/results/audio-castilian/es_ES-carlfm-x_low_castilian.wav` to confirm the chosen Castilian voice; alternatives are alongside it.
