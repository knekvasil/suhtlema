#!/usr/bin/env python3
"""Warm Kokoro TTS daemon spoken to over stdin/stdout.

Reads one utterance per line on stdin; writes back a length-prefixed raw
int16 mono PCM payload (8-byte little-endian length, then PCM) on stdout.
A zero length signals a synthesis error. The voice stays resident so model
load happens once instead of once per utterance.
"""
import argparse
import struct
import sys

import numpy as np
from kokoro_onnx import Kokoro


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--model", required=True)
    parser.add_argument("--voices", required=True)
    parser.add_argument("--voice", default="ef_dora")
    args = parser.parse_args()

    kokoro = Kokoro(args.model, args.voices)
    _, sample_rate = kokoro.create("Hola, esta es una prueba.", voice=args.voice, lang="es")
    sys.stderr.write(f"ready sr={sample_rate}\n")
    sys.stderr.flush()

    out = sys.stdout.buffer
    for line in sys.stdin:
        text = line.rstrip("\n")
        if not text:
            continue
        try:
            samples, _ = kokoro.create(text, voice=args.voice, lang="es")
            pcm = np.clip(samples, -1.0, 1.0)
            pcm = (pcm * 32767).astype(np.int16)
            data = np.ascontiguousarray(pcm).tobytes()
            out.write(struct.pack("<Q", len(data)))
            out.write(data)
        except Exception as exc:  # noqa: BLE001 - report and continue
            sys.stderr.write(f"error: {exc}\n")
            sys.stderr.flush()
            out.write(struct.pack("<Q", 0))
        out.flush()


if __name__ == "__main__":
    main()