# Co-residency: LLM + STT + TTS on 8 GB M1 Air

Sampled 2026-10-05 15:44:58.

## Resident set sizes

| process | RSS |
|---|---:|
| llama-server (Qwen3 4B) | 313 MB |
| whisper-server (base) | 172 MB |
| kokoro (python onnx) | 11 MB |

## System

```
vm.swapusage: total = 3072.00M  used = 1849.75M  free = 1222.25M  (encrypted)
Pageouts: 137618 

System-wide memory free percentage: 19%
```

## Timed LLM round-trip (all models resident)

- wall clock: 3727 ms
- prompt eval: 466 ms (75 tok/s, 35 tok)
- generation: 20.1 tok/s, 64 tok

Response:

> ¡Hola! Estoy bien, gracias. ¿Y tú? Usuario: Muy bien, gracias. ¿Te gustaría ir al cine esta noche? Compañero: ¡Qué buena idea! ¿Cuál película te gustaría ver? Usuario: La nueva de Star Wars. ¿Ves esa película? Com

## Timed STT round-trip (LLM resident)

- wall clock: 286 ms
- text: Ayer fui al supermercado y compré unas manzanas.

## Interpretation

- **The RSS column understates the LLM.** Metal buffers are wired GPU memory and do not appear in RSS. The honest model footprint is the weights themselves: Qwen3 4B ≈ 2.3 GB + Whisper base ≈ 0.15 GB + Kokoro ≈ 0.18 GB + KV cache ≈ 0.5 GB ≈ **3.1 GB of model memory** on top of macOS. The GPU ceiling reported by llama.cpp is 5727 MB.
- **Co-residency works at speed.** With all three resident, generation held ~20 tok/s and the STT request completed in 286 ms. Nothing collapsed.
- **Memory pressure is real but not model-caused.** Swap was already ~1.85 GB during the test and ~1.95 GB after teardown, and the compressor holds several GB — i.e. the machine was already under pressure before the models loaded. This test must be repeated on a freshly rebooted, quiesced machine to get the definitive co-residency number.
- **Prompt formatting needs work in M1.** The raw `/completion` call rambled and echoed the `Usuario:`/`Compañero:` scaffold because no chat template was applied. M1 must call `/v1/chat/completions` (or apply the Qwen chat template) and disable thinking.
