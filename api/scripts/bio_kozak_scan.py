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


def classify_kozak(sequence: str, start_index: int):
    minus3 = sequence[start_index - 3] if start_index - 3 >= 0 else ""
    plus4 = sequence[start_index + 3] if start_index + 3 < len(sequence) else ""
    score = 0
    if minus3 in {"A", "G"}:
        score += 1
    if plus4 == "G":
        score += 1
    if score == 2:
        strength = "strong"
    elif score == 1:
        strength = "moderate"
    else:
        strength = "weak"
    return minus3, plus4, strength


def build_result(payload):
    raw_sequence = payload.get("sequence", "")
    sequence_name = payload.get("sequence_name") or "sequence"
    sequence = normalize_sequence(raw_sequence)
    if not sequence:
        raise ValueError("sequence is empty after normalization")

    candidates = []
    for match in re.finditer("ATG", sequence):
        start = match.start()
        minus3, plus4, strength = classify_kozak(sequence, start)
        candidates.append({
            "start_codon": "ATG",
            "start": start + 1,
            "end": start + 3,
            "minus_3_base": minus3,
            "plus_4_base": plus4,
            "strength": strength,
            "context": sequence[max(0, start - 6):min(len(sequence), start + 9)],
        })

    strength_counts = {"strong": 0, "moderate": 0, "weak": 0}
    for item in candidates:
        strength_counts[item["strength"]] += 1

    warnings = []
    if not candidates:
        warnings.append("未发现 ATG 起始密码子，无法评估 Kozak 上下文")
    if len(sequence) < 30:
        warnings.append("序列较短，Kozak 扫描结果仅供快速参考")

    return {
        "panel": "kozak_scan",
        "sequence_name": sequence_name,
        "sequence_type": "dna",
        "input_length": len(sequence),
        "candidate_count": len(candidates),
        "strength_summary": strength_counts,
        "candidates": candidates[:30],
        "warnings": warnings,
        "recommended_next_steps": [
            "如需评估翻译起始可信度，可继续结合 ORF、保守性和转录本注释复核",
            "如存在多个候选起始位点，可对 strong 或 moderate Kozak 背景位点优先关注",
        ],
    }


def main():
    payload = read_payload()
    result = build_result(payload)
    print(json.dumps(result, ensure_ascii=False), flush=True)


if __name__ == "__main__":
    main()
