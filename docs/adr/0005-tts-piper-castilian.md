# TTS: Piper `es_ES-carlfm-x_low` (Castilian) instead of Kokoro

Status: accepted

The Companion must speak Castilian (Spain) Spanish. The plan specified Kokoro-82M, but Kokoro's Spanish voices are Latin American and synthesise at only ~3.7× realtime on this machine. We use Piper's `es_ES-carlfm-x_low` instead: a Castilian voice running on onnxruntime at ~45× realtime from a 27 MB model, needing no Neural Engine offload. The trade-off — Piper is less expressive than Kokoro — was accepted for the correct accent and an order-of-magnitude latency reduction. Other Castilian voices (Piper sharvard/davefx, Coqui CSS10, Mimic3) were benchmarked and kept as alternatives in `bench/results/audio-castilian/`; XTTS voice cloning is the fallback if a more natural voice is needed later.

## Considered options

- **Kokoro** — rejected: Latin American accent, ~3.7× RT.
- **Other Piper es_ES / Coqui CSS10 / Mimic3** — viable Castilian alternatives, not selected.
- **XTTS v2 (voice cloning)** — rejected for MVP: ~2 GB, PyTorch, much slower.
