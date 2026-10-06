# Inference runtime: llama.cpp + whisper.cpp with Neural Engine offload; MLX rejected from the critical path

Status: accepted

We use whisper.cpp for STT and llama.cpp for the LLM, keep TTS on onnxruntime, and orchestrate in Go. MLX was rejected for the latency-critical path despite being ~1.3–2× faster on Apple Silicon, because it is Python-first and would drag an interpreter into the critical path, undermining the Go orchestration and control over the real-time loop. The Whisper encoder can additionally be offloaded to the Neural Engine via Core ML so that it does not contend with the LLM for the GPU on a fanless chip.

## Considered options

- **MLX** — fastest raw tokens/sec, rejected: Python in the hot path, breaks Go orchestration.
- **PyTorch MPS** — rejected: higher memory overhead and slower than both alternatives.
