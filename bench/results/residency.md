# Co-residency: LLM + STT + TTS on 8 GB M1 Air (Qwen3.5-4B-Q4_K_M.gguf)

Sampled 2026-10-06 18:48:17.

## Resident set sizes

| process | RSS |
|---|---:|
| llama-server (Qwen3.5-4B-Q4_K_M.gguf) | 1682 MB |
| whisper-server (base) | 177 MB |
| kokoro (python onnx) | 263 MB |

## System

```
vm.swapusage: total = 0.00M  used = 0.00M  free = 0.00M  (encrypted)
Pageouts: 2663 

System-wide memory free percentage: 24%
```

## Timed LLM round-trip (all models resident)

- wall clock: 2851 ms
- prompt eval: 447 ms (72 tok/s, 32 tok)
- generation: 18.3 tok/s, 43 tok

Response:

> ¡Hola! Estoy muy bien, gracias por preguntar. ¿Y tú cómo estás? ¿Cómo te va todo hoy?  ¿Hay algo en especial en lo que quieras hablar o de qué te gustaría conversar?

## Timed STT round-trip (LLM resident)

- wall clock: 301 ms
- text: Ayer fui al supermercado y compré unas manzanas.
