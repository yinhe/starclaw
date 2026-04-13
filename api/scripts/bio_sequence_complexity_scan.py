#!/usr/bin/env python3
import json
import math
import re
import sys
from collections import Counter


def read_payload():
    return json.load(sys.stdin)


def normalize_sequence(raw: str):
    kept = []
    for line in (raw or "").splitlines():
        if line.startswith(">"):
            continue
        kept.extend(re.findall(r"[A-Za-z]", line))
    return "".join(kept).upper()


def classify_sequence(seq: str, hint: str):
    hint = (hint or "").strip().lower()
    if hint in {"dna", "rna", "protein"}:
        return hint
    letters = set(seq)
    if letters and letters.issubset(set("ACGTNRYWSKMBDHV")):
        return "dna"
    if letters and letters.issubset(set("ACGUNRYWSKMBDHV")):
        return "rna"
    if "U" in letters and "T" not in letters:
        return "rna"
    if "T" in letters and "U" not in letters:
        return "dna"
    return "protein"


def shannon_entropy(seq: str):
    if not seq:
        return 0.0
    counts = Counter(seq)
    total = len(seq)
    entropy = 0.0
    for count in counts.values():
        p = count / total
        entropy -= p * math.log2(p)
    return round(entropy, 4)


def build_result(payload):
    raw_sequence = payload.get("sequence", "")
    sequence_name = payload.get("sequence_name") or "sequence"
    sequence = normalize_sequence(raw_sequence)
    if not sequence:
        raise ValueError("sequence is empty after normalization")

    seq_type = classify_sequence(sequence, payload.get("sequence_type_hint", ""))
    window_size = int(payload.get("window_size") or 30)
    step_size = int(payload.get("step_size") or max(window_size // 2, 1))
    if window_size < 5:
        window_size = 5
    if step_size < 1:
        step_size = 1

    windows = []
    if len(sequence) <= window_size:
        windows.append({"start": 1, "end": len(sequence), "entropy": shannon_entropy(sequence)})
    else:
        for start in range(0, len(sequence) - window_size + 1, step_size):
            chunk = sequence[start:start + window_size]
            windows.append({
                "start": start + 1,
                "end": start + len(chunk),
                "entropy": shannon_entropy(chunk),
            })
        if windows and windows[-1]["end"] < len(sequence):
            chunk = sequence[-window_size:]
            windows.append({
                "start": len(sequence) - len(chunk) + 1,
                "end": len(sequence),
                "entropy": shannon_entropy(chunk),
            })

    low_complexity = sorted(windows, key=lambda item: (item["entropy"], item["start"]))[:5]
    high_complexity = sorted(windows, key=lambda item: (-item["entropy"], item["start"]))[:5]
    average_entropy = round(sum(item["entropy"] for item in windows) / len(windows), 4) if windows else 0.0

    warnings = []
    if len(sequence) < window_size:
        warnings.append("序列长度小于窗口大小，已退化为整体复杂度评估")
    if average_entropy < 1.2:
        warnings.append("整体序列复杂度偏低，可留意低复杂度片段或重复富集")

    return {
        "panel": "sequence_complexity_scan",
        "sequence_name": sequence_name,
        "sequence_type": seq_type,
        "input_length": len(sequence),
        "window_size": window_size,
        "step_size": step_size,
        "window_count": len(windows),
        "average_entropy": average_entropy,
        "low_complexity_windows": low_complexity,
        "high_complexity_windows": high_complexity,
        "warnings": warnings,
        "recommended_next_steps": [
            "如存在低复杂度区段，可结合重复扫描、k-mer 统计或比对策略继续复核",
            "如用于实验设计，可避开极低复杂度区段以减少非特异性风险",
        ],
    }


def main():
    payload = read_payload()
    result = build_result(payload)
    print(json.dumps(result, ensure_ascii=False), flush=True)


if __name__ == "__main__":
    main()
