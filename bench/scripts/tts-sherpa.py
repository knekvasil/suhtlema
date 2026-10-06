#!/usr/bin/env python3
"""Synthesise samples with non-Piper Castilian Spanish VITS models via sherpa-onnx.

Covers Coqui CSS10 (Castilian, single speaker) and Mimic3 es_ES (M-AILABS).
"""
import time
from pathlib import Path

import sherpa_onnx

ROOT = Path(__file__).resolve().parents[2]
SHERPA = ROOT / "models" / "tts" / "sherpa"
OUT = ROOT / "bench" / "results" / "audio-castilian"
OUT.mkdir(parents=True, exist_ok=True)

MODELS = {
    "coqui-css10": {
        "model": SHERPA / "vits-coqui-es-css10" / "model.onnx",
        "tokens": SHERPA / "vits-coqui-es-css10" / "tokens.txt",
    },
    "mimic3-es_ES": {
        "model": SHERPA / "vits-mimic3-es_ES-m-ailabs_low" / "es_ES-m-ailabs_low.onnx",
        "tokens": SHERPA / "vits-mimic3-es_ES-m-ailabs_low" / "tokens.txt",
        "data_dir": SHERPA / "vits-mimic3-es_ES-m-ailabs_low" / "espeak-ng-data",
    },
}

SENTENCES = {
    "medium": "Ayer fui al supermercado y compré unas manzanas.",
    "castilian": "Gracias, ¿me pones una cerveza y cinco tapas, por favor?",
    "long": (
        "Bueno, pues ayer fui al supermercado porque quería preparar "
        "una cena especial para mis amigos, y al final compré muchas "
        "cosas que no necesitaba."
    ),
}


def build(spec):
    vits = sherpa_onnx.OfflineTtsVitsModelConfig(
        model=str(spec["model"]),
        tokens=str(spec["tokens"]),
        data_dir=str(spec.get("data_dir", "")),
    )
    model = sherpa_onnx.OfflineTtsModelConfig(vits=vits, num_threads=4, debug=False, provider="cpu")
    return sherpa_onnx.OfflineTts(sherpa_onnx.OfflineTtsConfig(model=model))


def main():
    rows = []
    for name, spec in MODELS.items():
        tts = build(spec)
        tts.generate("Hola, ¿qué tal?", sid=0, speed=1.0)  # warm-up
        for key, text in SENTENCES.items():
            t0 = time.perf_counter()
            audio = tts.generate(text, sid=0, speed=1.0)
            gen = time.perf_counter() - t0
            path = OUT / f"{name}_{key}.wav"
            sherpa_onnx.write_wave(str(path), audio.samples, audio.sample_rate)
            dur = len(audio.samples) / audio.sample_rate
            rtf = dur / gen if gen else float("nan")
            rows.append((name, key, dur, gen, rtf))
            print(f"  {name:14s} {key:9s} audio={dur:5.2f}s gen={gen*1000:6.0f}ms rtf={rtf:5.1f}x")

    lines = [
        "# sherpa-onnx Castilian VITS samples",
        "",
        "| model | sentence | audio (s) | gen (ms) | RTF |",
        "|---|---|---:|---:|---:|",
    ]
    for n, k, d, g, r in rows:
        lines.append(f"| {n} | {k} | {d:.2f} | {g*1000:.0f} | {r:.1f}x |")
    (ROOT / "bench" / "results" / "tts-sherpa-castilian.md").write_text("\n".join(lines) + "\n")
    print(f"\nwrote samples to {OUT}")


if __name__ == "__main__":
    main()
