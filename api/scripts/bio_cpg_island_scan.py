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


def gc_percent(seq: str):
    if not seq:
        return 0.0
    return round((seq.count("G") + seq.count("C")) / len(seq) * 100, 2)


def observed_expected_cpg(seq: str):
    if not seq:
        return 0.0
    c = seq.count("C")
    g = seq.count("G")
    cg = sum(1 for i in range(len(seq) - 1) if seq[i:i + 2] == "CG")
    if c == 0 or g == 0:
        return 0.0
    return round((cg * len(seq)) / (c * g), 3)


def build_result(payload):
    raw_sequence = payload.get("sequence", "")
    sequence_name = payload.get("sequence_name") or "sequence"
    sequence = normalize_sequence(raw_sequence)
    if not sequence:
        raise ValueError("sequence is empty after normalization")

    window_size = int(payload.get("window_size") or 200)
    if window_size < 50:
        window_size = 50
    if window_size > len(sequence):
        window_size = len(sequence)

    islands = []
    if window_size > 0:
        for start in range(0, len(sequence) - window_size + 1):
            chunk = sequence[start:start + window_size]
            gc = gc_percent(chunk)
            oe = observed_expected_cpg(chunk)
            if gc >= 50 and oe >= 0.6:
                islands.append({
                    "start": start + 1,
                    "end": start + window_size,
                    "gc_percent": gc,
                    "observed_expected_cpg": oe,
                })

    merged = []
    for item in islands:
        if not merged or item["start"] > merged[-1]["end"] + 1:
            merged.append(dict(item))
        else:
            merged[-1]["end"] = max(merged[-1]["end"], item["end"])
            merged[-1]["gc_percent"] = max(merged[-1]["gc_percent"], item["gc_percent"])
            merged[-1]["observed_expected_cpg"] = max(merged[-1]["observed_expected_cpg"], item["observed_expected_cpg"])

    warnings = []
    if not merged:
        warnings.append("未发现满足阈值的 CpG island，可尝试调整窗口或复核序列区域")
    if len(sequence) < 200:
        warnings.append("序列较短，CpG island 扫描结果仅供快速参考")

    return {
        "panel": "cpg_island_scan",
        "sequence_name": sequence_name,
        "sequence_type": "dna",
        "input_length": len(sequence),
        "window_size": window_size,
        "island_count": len(merged),
        "islands": merged[:20],
        "warnings": warnings,
        "recommended_next_steps": [
            "如定位到 CpG island，可继续结合启动子、甲基化或表观组学数据复核",
            "如用于实验设计，可进一步结合基因组坐标和调控注释确认区域意义",
        ],
    }


def main():
    payload = read_payload()
    result = build_result(payload)
    print(json.dumps(result, ensure_ascii=False), flush=True)


if __name__ == "__main__":
    main()
