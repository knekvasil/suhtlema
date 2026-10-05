#!/usr/bin/env python3
"""Run whisper.cpp on the fixed Spanish set across model sizes.

Reports per-clip encode/decode/warm timing and word error rate against the
reference transcripts, then a per-model summary.

Caveat: the audio is Kokoro-synthesised (cleaner than human speech), so WER
here is optimistic; it discriminates between models, not real-world accuracy.
"""
import json
import re
import subprocess
import tempfile
import wave
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
WHISPER = ROOT / "vendor" / "whisper.cpp" / "build" / "bin" / "whisper-cli"
AUDIO = ROOT / "bench" / "results" / "stt-audio"
OUT = ROOT / "bench" / "results" / "stt-bench.md"
MODELS = {
    "tiny": ROOT / "models" / "whisper" / "ggml-tiny.bin",
    "base": ROOT / "models" / "whisper" / "ggml-base.bin",
    "small": ROOT / "models" / "whisper" / "ggml-small.bin",
}
NUM = r"([\d.]+)"


def wav_duration(path: Path) -> float:
    with wave.open(str(path), "rb") as w:
        return w.getnframes() / w.getframerate()


def normalize(s: str) -> list[str]:
    s = s.lower()
    s = re.sub(r"[¿?¡!.,;:\"()]", "", s)
    return s.split()


def wer(ref: list[str], hyp: list[str]) -> float:
    # Levenshtein on words.
    d = [[0] * (len(hyp) + 1) for _ in range(len(ref) + 1)]
    for i in range(len(ref) + 1):
        d[i][0] = i
    for j in range(len(hyp) + 1):
        d[0][j] = j
    for i in range(1, len(ref) + 1):
        for j in range(1, len(hyp) + 1):
            cost = 0 if ref[i - 1] == hyp[j - 1] else 1
            d[i][j] = min(d[i - 1][j] + 1, d[i][j - 1] + 1, d[i - 1][j - 1] + cost)
    return d[len(ref)][len(hyp)] / max(1, len(ref))


def run(model: Path, wav: Path):
    with tempfile.TemporaryDirectory() as td:
        of = str(Path(td) / "out")
        p = subprocess.run(
            [str(WHISPER), "-m", str(model), "-f", str(wav), "-l", "es",
             "-nt", "-otxt", "-of", of],
            capture_output=True, text=True,
        )
        err = p.stderr
        txt = Path(of + ".txt").read_text().strip() if Path(of + ".txt").exists() else ""
    def g(name):
        m = re.search(name + r"\s*=\s*" + NUM, err)
        return float(m.group(1)) if m else float("nan")
    return {
        "load": g("load time"),
        "encode": g("encode time"),
        "decode": g("decode time"),
        "total": g("total time"),
        "text": txt,
    }


def main():
    manifest = json.loads((AUDIO / "manifest.json").read_text())
    lines = [
        "# STT benchmark (whisper.cpp, Spanish)",
        "",
        "Warm cost = total - load (model is persistent in the real pipeline).",
        "Caveat: Kokoro-synthesised audio is cleaner than human speech; WER is optimistic.",
        "",
    ]
    summary = []
    for name, path in MODELS.items():
        rows = []
        for item in manifest:
            wav = AUDIO / item["file"]
            dur = wav_duration(wav)
            r = run(path, wav)
            warm = r["total"] - r["load"]
            e = wer(normalize(item["text"]), normalize(r["text"]))
            rows.append((item["id"], dur, r, warm, e))
        errs = [r[4] for r in rows]
        warms = [r[3] for r in rows]
        summary.append((name, sum(errs) / len(errs), sum(warms) / len(warms)))

        lines += [
            f"## {name}",
            "",
            "| clip | audio s | encode ms | decode ms | warm ms | RTF(warm) | WER |",
            "|---|---:|---:|---:|---:|---:|---:|",
        ]
        for cid, dur, r, warm, e in rows:
            lines.append(
                f"| {cid} | {dur:.2f} | {r['encode']:.0f} | {r['decode']:.0f} | "
                f"{warm:.0f} | {dur/warm*1000:.1f}x | {e*100:.0f}% |"
            )
        lines.append("")

    lines += ["## Summary", "", "| model | mean WER | mean warm ms |", "|---|---:|---:|"]
    for name, e, w in summary:
        lines.append(f"| {name} | {e*100:.1f}% | {w:.0f} |")

    OUT.write_text("\n".join(lines) + "\n")
    print("\n".join(lines))
    print(f"\nwrote {OUT}")


if __name__ == "__main__":
    main()
