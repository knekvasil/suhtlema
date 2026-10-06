# Co-residency: LLM + STT + TTS on 8 GB M1 Air (Qwen3.5-4B-Q4_K_M.gguf)

Sampled 2026-10-06 18:19:10.

## Resident set sizes

| process | RSS |
|---|---:|
| llama-server (Qwen3.5-4B-Q4_K_M.gguf) | 452 MB |
| whisper-server (base) | 173 MB |
| kokoro (python onnx) | 1 MB |

## System

```
vm.swapusage: total = 5120.00M  used = 3999.75M  free = 1120.25M  (encrypted)
Pageouts: 220641 

System-wide memory free percentage: 14%
```

## Timed LLM round-trip (all models resident)

- wall clock: 2603 ms
- prompt eval: 533 ms (60 tok/s, 32 tok)
- generation: 14.0 tok/s, 28 tok

Response:

> ¡Hola! Todo muy bien, gracias por preguntar. ¿Y tú cómo estás? ¿Hay algo en lo que pueda ayudarte hoy?

## Timed STT round-trip (LLM resident)

- wall clock: 339 ms
- text: Ayer fui al supermercado y compré unas manzanas.
