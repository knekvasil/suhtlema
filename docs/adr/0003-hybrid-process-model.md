# Hybrid process model: persistent model servers plus in-process ONNX

Status: accepted

The LLM and STT run as persistent local server processes (`llama-server`, `whisper-server`) that load their models once and stay warm; Silero VAD and Kokoro TTS run in-process in the Go binary via ONNX Runtime, with audio handled by `malgo`. The split isolates the heavy, crash-prone models from the real-time loop while keeping the tight audio-in → VAD → audio-out path free of IPC round-trips — which the deferred barge-in work will require. This accepts cgo/onnxruntime inside the Go process.

## Considered options

- **Spawn a CLI per turn** — rejected: model reload latency makes it unusable.
- **Everything in-process via cgo** — rejected: couples fragile model code to the real-time loop.
- **A third ONNX sidecar for VAD/TTS** — rejected: puts an IPC hop inside the real-time loop to avoid cgo.
