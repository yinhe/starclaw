#!/usr/bin/env python3
import json
import re
import sys


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
    return "mixed"


def gc_percent(seq: str):
    if not seq:
        return 0.0
    gc = sum(1 for ch in seq if ch in {"G", "C"})
    return round(gc / len(seq) * 100, 2)


def build_result(payload):
    raw_sequence = payload.get("sequence", "")
    sequence_name = payload.get("sequence_name") or "sequence"
    sequence = normalize_sequence(raw_sequence)
    if not sequence:
        raise ValueError("sequence is empty after normalization")

    seq_type = classify_sequence(sequence, payload.get("sequence_type_hint", ""))
    window_size = int(payload.get("window_size") or 50)
    step_size = int(payload.get("step_size") or max(window_size // 2, 1))
    if window_size < 5:
        window_size = 5
    if step_size < 1:
        step_size = 1

    seq = sequence.replace("U", "T")
    windows = []
    if len(seq) <= window_size:
        windows.append({
            "start": 1,
            "end": len(seq),
            "gc_percent": gc_percent(seq),
        })
    else:
        for start in range(0, len(seq) - window_size + 1, step_size):
            chunk = seq[start:start + window_size]
            windows.append({
                "start": start + 1,
                "end": start + len(chunk),
                "gc_percent": gc_percent(chunk),
            })
        if windows and windows[-1]["end"] < len(seq):
            chunk = seq[-window_size:]
            windows.append({
                "start": len(seq) - len(chunk) + 1,
                "end": len(seq),
                "gc_percent": gc_percent(chunk),
            })

    windows.sort(key=lambda item: (-item["gc_percent"], item["start"]))
    top_gc_windows = windows[:5]
    low_gc_windows = sorted(windows, key=lambda item: (item["gc_percent"], item["start"]))[:5]
    average_gc = round(sum(item["gc_percent"] for item in windows) / len(windows), 2) if windows else 0.0

    warnings = []
    if seq_type not in {"dna", "rna"}:
        warnings.append("GC window scan 更适合 DNA/RNA 序列；当前字符集可能不是标准核酸")
    if len(seq) < window_size:
        warnings.append("序列长度小于窗口大小，已退化为整体 GC 扫描")

    return {
        "panel": "gc_window_scan",
        "sequence_name": sequence_name,
        "sequence_type": seq_type,
        "input_length": len(sequence),
        "window_size": window_size,
        "step_size": step_size,
        "window_count": len(windows),
        "average_gc_percent": average_gc,
        "top_gc_windows": top_gc_windows,
        "low_gc_windows": low_gc_windows,
        "warnings": warnings,
        "recommended_next_steps": [
            "如用于引物或载体设计，可继续关注极端 GC 区段与重复序列",
            "如用于测序或扩增评估，可结合 motif、限制性位点和局部复杂度继续复核",
        ],
    }


def main():
    payload = read_payload()
    result = build_result(payload)
    print(json.dumps(result, ensure_ascii=False), flush=True)


if __name__ == "__main__":
    main()
