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


def build_result(payload):
    raw_sequence = payload.get("sequence", "")
    sequence_name = payload.get("sequence_name") or "sequence"
    sequence = normalize_sequence(raw_sequence)
    if not sequence:
        raise ValueError("sequence is empty after normalization")

    tracts = []
    for match in re.finditer(r"[CT]{6,}", sequence):
        start = match.start()
        tract = match.group(0)
        tracts.append({
            "start": start + 1,
            "end": start + len(tract),
            "length": len(tract),
            "tract": tract,
            "context": sequence[max(0, start - 10):min(len(sequence), start + len(tract) + 10)],
        })

    tracts.sort(key=lambda item: (-item["length"], item["start"]))
    warnings = []
    if not tracts:
        warnings.append("未发现明显 polypyrimidine tract，可结合剪接位点或更长内含子区域继续复核")
    if len(sequence) < 50:
        warnings.append("序列较短，polypyrimidine tract 扫描结果仅供快速参考")

    return {
        "panel": "polypyrimidine_tract_scan",
        "sequence_name": sequence_name,
        "sequence_type": "dna",
        "input_length": len(sequence),
        "tract_count": len(tracts),
        "longest_tract": tracts[0] if tracts else None,
        "tracts": tracts[:30],
        "warnings": warnings,
        "recommended_next_steps": [
            "如用于剪接相关分析，可继续结合 acceptor 位点、branch point 和转录本注释复核",
            "如 tract 较长且靠近候选剪接区域，可进一步结合变异和保守性评估影响",
        ],
    }


def main():
    payload = read_payload()
    result = build_result(payload)
    print(json.dumps(result, ensure_ascii=False), flush=True)


if __name__ == "__main__":
    main()
