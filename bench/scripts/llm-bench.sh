#!/usr/bin/env bash
# Raw LLM throughput (prompt processing + generation) via llama-bench.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
BENCH="$ROOT/vendor/llama.cpp/build/bin/llama-bench"
OUT="$ROOT/bench/results/llm-bench.md"
mkdir -p "$(dirname "$OUT")"

"$BENCH" \
  -m "$ROOT/models/llm/Qwen3-4B-Q4_K_M.gguf" \
  -m "$ROOT/models/llm/Llama-3.2-3B-Instruct-Q4_K_M.gguf" \
  -p 512,2048 -n 128 -r 3 -o md | tee "$OUT"
