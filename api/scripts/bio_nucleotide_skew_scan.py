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
    return "".join(kept).upper().replace("U", "T")


def safe_skew(a: int, b: int):
    total = a + b
    if total == 0:
        return 0.0
    return round((a - b) / total, 4)


def build_result(payload):
    raw_sequence = payload.get("sequence", "")
    sequence_name = payload.get("sequence_name") or "sequence"
    sequence = normalize_sequence(raw_sequence)
    if not sequence:
        raise ValueError("sequence is empty after normalization")

    window_size = int(payload.get("window_size") or 100)
    step_size = int(payload.get("step_size") or max(window_size // 2, 1))
    if window_size < 20:
        window_size = 20
    if step_size < 1:
        step_size = 1
    if window_size > len(sequence):
        window_size = len(sequence)

    windows = []
    for start in range(0, len(sequence) - window_size + 1, step_size):
        chunk = sequence[start:start + window_size]
        g = chunk.count("G")
        c = chunk.count("C")
        a = chunk.count("A")
        t = chunk.count("T")
        windows.append({
            "start": start + 1,
            "end": start + window_size,
            "gc_skew": safe_skew(g, c),
            "at_skew": safe_skew(a, t),
        })

    if not windows:
        windows.append({
            "start": 1,
            "end": len(sequence),
            "gc_skew": safe_skew(sequence.count("G"), sequence.count("C")),
            "at_skew": safe_skew(sequence.count("A"), sequence.count("T")),
        })

    max_gc = max(windows, key=lambda item: item["gc_skew"])
    min_gc = min(windows, key=lambda item: item["gc_skew"])
    max_at = max(windows, key=lambda item: item["at_skew"])
    min_at = min(windows, key=lambda item: item["at_skew"])
    avg_gc = round(sum(item["gc_skew"] for item in windows) / len(windows), 4)
    avg_at = round(sum(item["at_skew"] for item in windows) / len(windows), 4)

    warnings = []
    if len(sequence) < 100:
        warnings.append("序列较短，核苷酸偏斜结果更适合做快速参考")

    return {
        "panel": "nucleotide_skew_scan",
        "sequence_name": sequence_name,
        "sequence_type": "dna",
        "input_length": len(sequence),
        "window_size": window_size,
        "step_size": step_size,
        "window_count": len(windows),
        "average_gc_skew": avg_gc,
        "average_at_skew": avg_at,
        "highest_gc_skew_window": max_gc,
        "lowest_gc_skew_window": min_gc,
        "highest_at_skew_window": max_at,
        "lowest_at_skew_window": min_at,
        "windows": windows[:30],
        "warnings": warnings,
        "recommended_next_steps": [
            "如发现偏斜明显的区段，可继续结合复制起点、链偏好或局部功能区域复核",
            "如用于比较分析，可对不同区域或不同样本执行同样窗口参数后再对比",
        ],
    }


def main():
    payload = read_payload()
    result = build_result(payload)
    print(json.dumps(result, ensure_ascii=False), flush=True)


if __name__ == "__main__":
    main()
