#!/usr/bin/env bash
# Co-residency test: hold LLM + STT + TTS resident simultaneously and measure
# memory, swap, and a timed round-trip. This is where an 8 GB machine fails.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
LBIN="$ROOT/third_party/llama.cpp/build/bin"
WBIN="$ROOT/third_party/whisper.cpp/build/bin"
RES="$ROOT/bench/results"
OUT="$RES/residency.md"
LLAMA_PORT=8080
WHISPER_PORT=8081

cleanup() { kill "${LPID:-}" "${WPID:-}" "${KPID:-}" 2>/dev/null || true; }
trap cleanup EXIT

echo "==> starting llama-server (Qwen3 4B)"
"$LBIN/llama-server" -m "$ROOT/models/llm/Qwen3-4B-Q4_K_M.gguf" -c 2048 -ngl 99 \
  --host 127.0.0.1 --port "$LLAMA_PORT" > "$RES/llama-server.log" 2>&1 &
LPID=$!

echo "==> starting whisper-server (base)"
"$WBIN/whisper-server" -m "$ROOT/models/whisper/ggml-base.bin" \
  --host 127.0.0.1 --port "$WHISPER_PORT" > "$RES/whisper-server.log" 2>&1 &
WPID=$!

for _ in $(seq 1 90); do
  curl -s "http://127.0.0.1:$LLAMA_PORT/health" >/dev/null 2>&1 && break
  sleep 1
done
for _ in $(seq 1 90); do
  curl -s "http://127.0.0.1:$WHISPER_PORT/" >/dev/null 2>&1 && break
  sleep 1
done

echo "==> holding Kokoro resident"
"$ROOT/bench/.venv/bin/python" - <<'PY' &
from pathlib import Path
from kokoro_onnx import Kokoro
import time
root = Path("/Users/kaljunekvasil/Developer/knekvasil/suhtlema")
k = Kokoro(str(root/"models/tts/kokoro-v1.0.fp16.onnx"), str(root/"models/tts/voices-v1.0.bin"))
k.create("Hola, esto es una prueba.", voice="ef_dora", lang="es")
time.sleep(180)
PY
KPID=$!
sleep 8

echo "==> sampling memory"
{
  echo "# Co-residency: LLM + STT + TTS on 8 GB M1 Air"
  echo
  echo "Sampled $(date '+%Y-%m-%d %H:%M:%S')."
  echo
  echo '## Resident set sizes'
  echo
  echo "| process | RSS |"
  echo "|---|---:|"
  for spec in "$LPID:llama-server (Qwen3 4B)" "$WPID:whisper-server (base)" "$KPID:kokoro (python onnx)"; do
    pid="${spec%%:*}"; name="${spec#*:}"
    rss=$(ps -o rss= -p "$pid" 2>/dev/null | tr -d ' ')
    if [ -n "$rss" ]; then
      echo "| $name | $((rss/1024)) MB |"
    else
      echo "| $name | (not running) |"
    fi
  done
  echo
  echo '## System'
  echo
  echo '```'
  sysctl vm.swapusage
  memory_pressure | tail -3
  echo '```'
} | tee "$OUT"

echo "==> timed LLM round-trip (while all resident)"
t0=$(python3 -c 'import time;print(time.time())')
curl -s "http://127.0.0.1:$LLAMA_PORT/completion" -H 'Content-Type: application/json' \
  -d '{"prompt":"Eres un compañero de conversación en español. Responde en 1-2 frases.\nUsuario: Hola, ¿cómo estás?\nCompañero:","n_predict":64,"temperature":0.7,"cache_prompt":true}' \
  > "$RES/llama-completion.json"
t1=$(python3 -c 'import time;print(time.time())')
python3 - "$RES/llama-completion.json" "$t0" "$t1" <<'PY' | tee -a "$OUT"
import json, sys
d = json.load(open(sys.argv[1]))
t0, t1 = float(sys.argv[2]), float(sys.argv[3])
tm = d.get("timings", {})
print()
print("## Timed LLM round-trip (all models resident)")
print()
print(f"- wall clock: {(t1-t0)*1000:.0f} ms")
print(f"- prompt eval: {tm.get('prompt_ms',0):.0f} ms ({tm.get('prompt_per_second',0):.0f} tok/s, {tm.get('prompt_n',0)} tok)")
print(f"- generation: {tm.get('predicted_per_second',0):.1f} tok/s, {tm.get('predicted_n',0)} tok")
print()
print("Response:")
print()
print("> " + d.get("content","").strip().replace("\n"," "))
PY

echo "==> timed STT round-trip (llama still resident)"
t0=$(python3 -c 'import time;print(time.time())')
curl -s "http://127.0.0.1:$WHISPER_PORT/inference" \
  -F "file=@$RES/stt-audio/01.wav" -F "language=es" > "$RES/whisper-response.json"
t1=$(python3 -c 'import time;print(time.time())')
python3 - "$RES/whisper-response.json" "$t0" "$t1" <<'PY' | tee -a "$OUT"
import json, sys
d = json.load(open(sys.argv[1]))
t0, t1 = float(sys.argv[2]), float(sys.argv[3])
print()
print("## Timed STT round-trip (LLM resident)")
print()
print(f"- wall clock: {(t1-t0)*1000:.0f} ms")
print(f"- text: {d.get('text','').strip()}")
PY
