#!/usr/bin/env bash
# Thermal soak: continuous LLM generation for a fixed duration, logging prompt
# throughput (and implicitly temperature) over time. Run on a fanless Air this
# is the number that matters, not the cold-start bench.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
BENCH="$ROOT/third_party/llama.cpp/build/bin/llama-bench"
OUTDIR="$ROOT/bench/results"
DURATION="${1:-600}"   # seconds per model
TOKENS=512
mkdir -p "$OUTDIR"

soak_model() {
  local model="$1" name="$2"
  local out="$OUTDIR/soak-${name}.csv"
  echo "==> soaking ${name} for ${DURATION}s"
  echo "elapsed_s,gen_tok_s" > "$out"
  local start now elapsed line ts
  start=$(date +%s)
  while :; do
    now=$(date +%s); elapsed=$((now - start))
    [ "$elapsed" -ge "$DURATION" ] && break
    line=$("$BENCH" -m "$model" -n "$TOKENS" -r 1 -o md 2>/dev/null)
    ts=$(awk -F'|' '/\| *tg[0-9]/ {split($8,a," "); print a[1]; exit}' <<<"$line")
    echo "$elapsed,$ts" | tee -a "$out"
  done
  awk -F, 'NR>1{if(min==""||$2<min)min=$2; sum+=$2; n++} END{printf "  min=%.1f  avg=%.1f tok/s over %d samples\n", min, sum/n, n}' "$out"
}

soak_model "$ROOT/models/llm/Qwen3-4B-Q4_K_M.gguf" qwen3-4b
soak_model "$ROOT/models/llm/Llama-3.2-3B-Instruct-Q4_K_M.gguf" llama-3.2-3b
