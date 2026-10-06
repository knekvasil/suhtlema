#!/usr/bin/env bash
# Clone and build the inference toolchains (llama.cpp, whisper.cpp) with Metal.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
VENDOR="$ROOT/third_party"
JOBS="$(sysctl -n hw.ncpu)"
mkdir -p "$VENDOR"

if ! command -v cmake >/dev/null 2>&1; then
  echo "==> installing cmake"
  brew install cmake
fi

echo "==> llama.cpp"
if [ ! -d "$VENDOR/llama.cpp/.git" ]; then
  git clone --depth 1 https://github.com/ggml-org/llama.cpp "$VENDOR/llama.cpp"
fi
cmake -S "$VENDOR/llama.cpp" -B "$VENDOR/llama.cpp/build" \
  -DGGML_METAL=ON -DCMAKE_BUILD_TYPE=Release
cmake --build "$VENDOR/llama.cpp/build" -j"$JOBS" \
  --target llama-bench llama-server llama-cli

echo "==> whisper.cpp"
if [ ! -d "$VENDOR/whisper.cpp/.git" ]; then
  git clone --depth 1 https://github.com/ggml-org/whisper.cpp "$VENDOR/whisper.cpp"
fi
cmake -S "$VENDOR/whisper.cpp" -B "$VENDOR/whisper.cpp/build" \
  -DGGML_METAL=ON -DWHISPER_METAL=ON -DCMAKE_BUILD_TYPE=Release
cmake --build "$VENDOR/whisper.cpp/build" -j"$JOBS" \
  --target whisper-cli whisper-bench whisper-server

echo "==> done"
echo "llama:   $VENDOR/llama.cpp/build/bin"
echo "whisper: $VENDOR/whisper.cpp/build/bin"
