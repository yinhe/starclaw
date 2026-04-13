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

    motif_defs = [
        ("AU_rich_like", "ATTTA"),
        ("cytoplasmic_polyadenylation_like", "TTTTAAT"),
        ("Pumilio_like", "TGTAAATA"),
        ("musashi_like", "GTAG"),
    ]
    hits = []
    for motif_name, motif in motif_defs:
        for match in re.finditer(motif, sequence):
            start = match.start()
            hits.append({
                "motif": motif_name,
                "pattern": motif,
                "start": start + 1,
                "end": start + len(motif),
                "context": sequence[max(0, start - 12):min(len(sequence), start + len(motif) + 12)],
            })

    hits.sort(key=lambda item: (item["start"], item["motif"]))
    motif_counts = {}
    for item in hits:
        motif_counts[item["motif"]] = motif_counts.get(item["motif"], 0) + 1

    warnings = []
    if not hits:
        warnings.append("未发现常见 UTR-like motif，可结合真实转录本上下文和物种背景继续复核")
    if len(sequence) < 50:
        warnings.append("序列较短，UTR motif 扫描结果仅供快速参考")

    return {
        "panel": "utr_motif_scan",
        "sequence_name": sequence_name,
        "sequence_type": "dna",
        "input_length": len(sequence),
        "motif_count": len(hits),
        "motif_summary": motif_counts,
        "motifs": hits[:30],
        "warnings": warnings,
        "recommended_next_steps": [
            "如用于 3' 或 5' UTR 初筛，可继续结合转录本注释、保守性和 RBP 背景复核",
            "如命中多个候选 motif，可进一步结合表达调控场景和实验背景评估优先级",
        ],
    }


def main():
    payload = read_payload()
    result = build_result(payload)
    print(json.dumps(result, ensure_ascii=False), flush=True)


if __name__ == "__main__":
    main()
