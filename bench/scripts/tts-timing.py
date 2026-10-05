#!/usr/bin/env python3
"""Kokoro TTS timing + Spanish sample generation.

Measures time-to-first-audio (whole-utterance synthesis here, since M1 is
sequential) and real-time factor for the Spanish voices, and dumps WAVs for a
human quality listen.
"""
import time
import wave
from pathlib import Path

import numpy as np
from kokoro_onnx import Kokoro

ROOT = Path(__file__).resolve().parents[2]
MODEL = ROOT / "models" / "tts" / "kokoro-v1.0.fp16.onnx"
VOICES = ROOT / "models" / "tts" / "voices-v1.0.bin"
OUT = ROOT / "bench" / "results"
AUDIO = OUT / "audio"
AUDIO.mkdir(parents=True, exist_ok=True)

VOICE_IDS = ["ef_dora", "em_alex", "em_santa"]
SENTENCES = {
    "short": "¿Cómo estás?",
    "medium": "Ayer fui al supermercado y compré unas manzanas.",
    "long": (
        "Bueno, pues ayer fui al supermercado porque quería preparar "
        "una cena especial para mis amigos, y al final compré muchas "
        "cosas que no necesitaba."
    ),
}


def write_wav(path: Path, samples, sr: int) -> None:
    pcm = np.clip(samples, -1.0, 1.0)
    pcm = (pcm * 32767).astype(np.int16)
    with wave.open(str(path), "wb") as w:
        w.setnchannels(1)
        w.setsampwidth(2)
        w.setframerate(sr)
        w.writeframes(pcm.tobytes())


def main() -> None:
    t0 = time.perf_counter()
    kokoro = Kokoro(str(MODEL), str(VOICES))
    load_s = time.perf_counter() - t0

    kokoro.create("Hola, ¿qué tal?", voice=VOICE_IDS[0], lang="es")  # warm-up

    rows = []
    for voice in VOICE_IDS:
        for key, text in SENTENCES.items():
            t0 = time.perf_counter()
            samples, sr = kokoro.create(text, voice=voice, lang="es")
            gen_s = time.perf_counter() - t0
            dur_s = len(samples) / sr
            rtf = dur_s / gen_s if gen_s else float("nan")
            write_wav(AUDIO / f"{voice}_{key}.wav", samples, sr)
            rows.append((voice, key, dur_s, gen_s, rtf))
            print(f"  {voice:9s} {key:6s} audio={dur_s:5.2f}s gen={gen_s*1000:6.0f}ms rtf={rtf:5.1f}x")

    lines = [
        "# Kokoro TTS timing (Spanish)",
        "",
        f"Model load: {load_s:.2f}s  ·  sr={sr} Hz",
        "",
        "| voice | sentence | audio (s) | gen (ms) | RTF |",
        "|---|---|---:|---:|---:|",
    ]
    for voice, key, dur, gen, rtf in rows:
        lines.append(f"| {voice} | {key} | {dur:.2f} | {gen*1000:.0f} | {rtf:.1f}x |")
    (OUT / "tts-kokoro.md").write_text("\n".join(lines) + "\n")
    print(f"\nwrote {OUT / 'tts-kokoro.md'}")
    print(f"samples in {AUDIO}")


if __name__ == "__main__":
    main()
