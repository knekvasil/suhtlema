#!/usr/bin/env python3
"""Generate Castilian (es_ES) samples with Piper for comparison against Kokoro.

Piper has explicit Spain Spanish voices; Kokoro's Spanish set is Latin American.
"""
import time
import wave
from pathlib import Path

from piper import PiperVoice
from piper.download_voices import download_voice

ROOT = Path(__file__).resolve().parents[2]
DL = ROOT / "models" / "tts" / "piper"
OUT = ROOT / "bench" / "results" / "audio-castilian"
DL.mkdir(parents=True, exist_ok=True)
OUT.mkdir(parents=True, exist_ok=True)

VOICES = [
    "es_ES-sharvard-medium",   # male
    "es_ES-davefx-medium",     # male
    "es_ES-carlfm-x_low",      # male, fast/low
    "es_ES-mls_10246-low",     # female
]

SENTENCES = {
    "medium": "Ayer fui al supermercado y compré unas manzanas.",
    "castilian": "Gracias, ¿me pones una cerveza y cinco tapas, por favor?",
    "long": (
        "Bueno, pues ayer fui al supermercado porque quería preparar "
        "una cena especial para mis amigos, y al final compré muchas "
        "cosas que no necesitaba."
    ),
}


def main():
    rows = []
    for voice_name in VOICES:
        onnx = DL / f"{voice_name}.onnx"
        if not onnx.exists():
            print(f"  downloading {voice_name}")
            download_voice(voice_name, DL)
        voice = PiperVoice.load(str(onnx))
        # Warm-up.
        with wave.open(str(OUT / "_warm.wav"), "wb") as wf:
            voice.synthesize_wav("Hola, ¿qué tal?", wf)
        for key, text in SENTENCES.items():
            path = OUT / f"{voice_name}_{key}.wav"
            t0 = time.perf_counter()
            with wave.open(str(path), "wb") as wf:
                voice.synthesize_wav(text, wf)
            gen = time.perf_counter() - t0
            with wave.open(str(path), "rb") as wf:
                dur = wf.getnframes() / wf.getframerate()
            rtf = dur / gen if gen else float("nan")
            rows.append((voice_name, key, dur, gen, rtf))
            print(f"  {voice_name:24s} {key:9s} audio={dur:5.2f}s gen={gen*1000:6.0f}ms rtf={rtf:5.1f}x")

    lines = [
        "# Piper Castilian (es_ES) samples",
        "",
        "| voice | sentence | audio (s) | gen (ms) | RTF |",
        "|---|---|---:|---:|---:|",
    ]
    for v, k, d, g, r in rows:
        lines.append(f"| {v} | {k} | {d:.2f} | {g*1000:.0f} | {r:.1f}x |")
    (ROOT / "bench" / "results" / "tts-piper-castilian.md").write_text("\n".join(lines) + "\n")
    print(f"\nwrote samples to {OUT}")


if __name__ == "__main__":
    main()
