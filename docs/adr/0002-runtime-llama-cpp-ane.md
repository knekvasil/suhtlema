# Inference runtime: llama.cpp + whisper.cpp with Neural Engine offload; MLX rejected from the critical path

Status: accepted

We use whisper.cpp for STT and llama.cpp for the LLM, offload the Whisper encoder and Kokoro TTS to Apple's Neural Engine via Core ML, and orchestrate in Go. MLX was rejected for the latency-critical path despite being ~1.3–2× faster on Apple Silicon, because it is Python-first and would drag an interpreter into the critical path, undermining the Go orchestration and control over the real-time loop. Keeping the LLM on the GPU and STT/TTS on the ANE also prevents three stages from contending for the same silicon on a fanless chip.

## Considered options

- **MLX** — fastest raw tokens/sec, rejected: Python in the hot path, breaks Go orchestration.
- **PyTorch MPS** — rejected: higher memory overhead and slower than both alternatives.
