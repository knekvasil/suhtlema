#!/usr/bin/env bash
# Download model weights into models/. Idempotent: skips files already present.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
MODELS="$ROOT/models"
mkdir -p "$MODELS/llm" "$MODELS/whisper" "$MODELS/tts"

fetch() {
  local url="$1" dest="$2"
  if [ -f "$dest" ]; then
    echo "have  $(basename "$dest")"
    return
  fi
  echo "fetch $(basename "$dest")"
  curl -L --fail -sS -C - -o "$dest" "$url"
  echo "  done ($(du -h "$dest" | cut -f1))"
}

# LLM (GGUF, Q4_K_M). Qwen3.5 4B is the primary; Llama 3.2 3B is the
# memory fallback.
fetch "https://huggingface.co/unsloth/Qwen3.5-4B-GGUF/resolve/main/Qwen3.5-4B-Q4_K_M.gguf" \
      "$MODELS/llm/Qwen3.5-4B-Q4_K_M.gguf"
fetch "https://huggingface.co/bartowski/Llama-3.2-3B-Instruct-GGUF/resolve/main/Llama-3.2-3B-Instruct-Q4_K_M.gguf" \
      "$MODELS/llm/Llama-3.2-3B-Instruct-Q4_K_M.gguf"

# STT (whisper.cpp GGML, multilingual)
for m in tiny base small; do
  fetch "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-${m}.bin" \
        "$MODELS/whisper/ggml-${m}.bin"
done

# TTS (Kokoro)
fetch "https://github.com/thewh1teagle/kokoro-onnx/releases/download/model-files-v1.0/kokoro-v1.0.fp16.onnx" \
      "$MODELS/tts/kokoro-v1.0.fp16.onnx"
fetch "https://github.com/thewh1teagle/kokoro-onnx/releases/download/model-files-v1.0/voices-v1.0.bin" \
      "$MODELS/tts/voices-v1.0.bin"

echo "==> models:"
du -h "$MODELS"/*/* | sort -h
