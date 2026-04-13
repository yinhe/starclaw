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


def find_stops(sequence: str, start_index: int):
    for i in range(start_index + 3, len(sequence) - 2, 3):
        codon = sequence[i:i + 3]
        if codon in {"TAA", "TAG", "TGA"}:
            return i
    return -1


def build_result(payload):
    raw_sequence = payload.get("sequence", "")
    sequence_name = payload.get("sequence_name") or "sequence"
    sequence = normalize_sequence(raw_sequence)
    if not sequence:
        raise ValueError("sequence is empty after normalization")

    uorfs = []
    for match in re.finditer("ATG", sequence):
        start = match.start()
        stop = find_stops(sequence, start)
        if stop == -1:
            continue
        length_nt = stop + 3 - start
        if length_nt < 9:
            continue
        uorfs.append({
            "start": start + 1,
            "end": stop + 3,
            "length_nt": length_nt,
            "length_aa": length_nt // 3,
            "stop_codon": sequence[stop:stop + 3],
            "frame": (start % 3) + 1,
            "context": sequence[max(0, start - 6):min(len(sequence), stop + 9)],
        })

    warnings = []
    if not uorfs:
        warnings.append("未发现满足条件的短上游 ORF，可结合更长 5' 区域继续复核")
    if len(sequence) < 60:
        warnings.append("序列较短，uORF 扫描结果仅供快速参考")

    return {
        "panel": "uorf_scan",
        "sequence_name": sequence_name,
        "sequence_type": "dna",
        "input_length": len(sequence),
        "uorf_count": len(uorfs),
        "uorfs": uorfs[:30],
        "warnings": warnings,
        "recommended_next_steps": [
            "如用于翻译调控初筛，可继续结合 Kozak 背景、主 ORF 起始位点和转录本注释复核",
            "如存在多个候选 uORF，可进一步比较长度、位置和保守性后再优先关注",
        ],
    }


def main():
    payload = read_payload()
    result = build_result(payload)
    print(json.dumps(result, ensure_ascii=False), flush=True)


if __name__ == "__main__":
    main()
