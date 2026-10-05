#!/usr/bin/env python3
"""Generate a fixed Spanish STT test set with Kokoro and write a manifest."""
import json
import wave
from pathlib import Path

import numpy as np
from kokoro_onnx import Kokoro

ROOT = Path(__file__).resolve().parents[2]
MODEL = ROOT / "models" / "tts" / "kokoro-v1.0.fp16.onnx"
VOICES = ROOT / "models" / "tts" / "voices-v1.0.bin"
OUT = ROOT / "bench" / "results" / "stt-audio"
OUT.mkdir(parents=True, exist_ok=True)

# (id, voice, text) — varied vocabulary, including the kinds of words a
# learner would actually fumble.
CLIPS = [
    ("01", "ef_dora", "Ayer fui al supermercado y compré unas manzanas."),
    ("02", "em_alex", "¿Me puedes dar el recibo, por favor?"),
    ("03", "ef_dora", "No me di cuenta de que había un problema."),
    ("04", "em_alex", "Quería quedarme en casa, pero al final salí."),
    ("05", "ef_dora", "El niño estaba durmiendo cuando llegamos."),
    ("06", "em_alex", "¿Has probado el gazpacho alguna vez?"),
    ("07", "ef_dora", "Me gustaría reservar una mesa para dos personas."),
    ("08", "em_alex", "Creo que olvidé las llaves en el coche."),
]


def resample(x, sr_in, sr_out):
    if sr_in == sr_out:
        return x
    n = int(round(len(x) * sr_out / sr_in))
    idx = np.linspace(0, len(x) - 1, n)
    return np.interp(idx, np.arange(len(x)), x).astype(np.float32)


def write_wav16k(path, samples, sr):
    y = resample(samples, sr, 16000)
    pcm = np.clip(y, -1.0, 1.0)
    pcm = (pcm * 32767).astype(np.int16)
    with wave.open(str(path), "wb") as w:
        w.setnchannels(1)
        w.setsampwidth(2)
        w.setframerate(16000)
        w.writeframes(pcm.tobytes())


def main():
    kokoro = Kokoro(str(MODEL), str(VOICES))
    manifest = []
    for cid, voice, text in CLIPS:
        samples, sr = kokoro.create(text, voice=voice, lang="es")
        path = OUT / f"{cid}.wav"
        write_wav16k(path, samples, sr)
        manifest.append({"id": cid, "file": path.name, "text": text, "voice": voice})
        print(f"  {cid} {voice} {len(samples)/sr:.2f}s")
    (OUT / "manifest.json").write_text(json.dumps(manifest, ensure_ascii=False, indent=2))
    print(f"wrote {OUT / 'manifest.json'}")


if __name__ == "__main__":
    main()
