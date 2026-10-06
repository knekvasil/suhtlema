#!/usr/bin/env python3
"""Warm Piper TTS daemon spoken to over stdin/stdout.

Reads one utterance per line on stdin; writes back a length-prefixed raw
int16 mono WAV payload (8-byte little-endian length, then PCM) on stdout.
A zero length signals a synthesis error. Keeps the voice resident so model
load happens once instead of once per utterance.
"""
import argparse
import struct
import sys

import numpy as np
from piper import PiperVoice


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--model", required=True)
    args = parser.parse_args()

    voice = PiperVoice.load(args.model)
    sample_rate = voice.config.sample_rate
    # Warm up once so the first real request is not penalised by lazy init.
    for _ in voice.synthesize("Hola."):
        pass
    sys.stderr.write(f"ready sr={sample_rate}\n")
    sys.stderr.flush()

    out = sys.stdout.buffer
    for line in sys.stdin:
        text = line.rstrip("\n")
        if not text:
            continue
        try:
            chunks = [c.audio_int16_array for c in voice.synthesize(text)]
            pcm = np.concatenate(chunks) if chunks else np.zeros(0, dtype=np.int16)
            data = np.ascontiguousarray(pcm, dtype=np.int16).tobytes()
            out.write(struct.pack("<Q", len(data)))
            out.write(data)
        except Exception as exc:  # noqa: BLE001 - report and continue
            sys.stderr.write(f"error: {exc}\n")
            sys.stderr.flush()
            out.write(struct.pack("<Q", 0))
        out.flush()


if __name__ == "__main__":
    main()