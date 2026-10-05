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
| TTS | Kokoro fp16, `ef_dora`/`em_alex` | **~3.7× RT** (CPU ONNX) | ~260 ms for a 1 s clause | ⚠️ needs human listen |

Note: TTS is **~4× RT, not the ~20× the plan assumed** — that figure was MLX on an M1 Max. On onnxruntime CPU it is ~4×. CoreML/ANE offload is required to close that gap.

## Configuration comparison

| Configuration | STT | LLM | TTS | Sustained LLM | STT warm | TTS RTF |
|---|---|---|---|---|---|---|
| A | base | Llama 3.2 3B | Kokoro | 23.9 tok/s | 187 ms | 3.7× |
| **B** | base | **Qwen3 4B** | Kokoro | **19.1 tok/s** | 187 ms | 3.7× |
| C | small | Qwen3 4B | Kokoro | 19.1 tok/s | 511 ms | 3.7× |

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
| Whisper base + Qwen3 4B + Kokoro co-reside without swapping | ⚠️ **INCONCLUSIVE** — inference held speed, but the machine was already ~1.9 GB into swap before models loaded. Re-test on a freshly rebooted, quiesced machine. |

**Verdict: proceed to M1.** The stack is viable on this hardware. Two open items before trusting the memory number:

1. Re-run `bench/scripts/residency.sh` immediately after a reboot with nothing else open.
2. **Listen** to `bench/results/audio/*.wav` and judge Kokoro's Spanish — this is a human check that no benchmark can make.
