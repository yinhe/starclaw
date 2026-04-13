#!/usr/bin/env python3
import json
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


def build_result(payload):
    raw_sequence = payload.get("sequence", "")
    sequence_name = payload.get("sequence_name") or "sequence"
    sequence = normalize_sequence(raw_sequence)
    if not sequence:
        raise ValueError("sequence is empty after normalization")

    seq_type = classify_sequence(sequence, payload.get("sequence_type_hint", ""))
    k = int(payload.get("k") or 3)
    if k < 1:
        k = 1
    if k > 6:
        k = 6

    kmers = [sequence[i:i + k] for i in range(0, len(sequence) - k + 1)]
    counts = Counter(kmers)
    total = len(kmers)
    top_kmers = [
        {"kmer": kmer, "count": count, "percent": round(count / total * 100, 2) if total else 0.0}
        for kmer, count in counts.most_common(20)
    ]

    warnings = []
    if total == 0:
        warnings.append("序列长度短于 k 值，未形成有效 k-mer")
    if len(sequence) < 30:
        warnings.append("序列较短，k-mer 频率统计代表性有限")

    return {
        "panel": "kmer_profile",
        "sequence_name": sequence_name,
        "sequence_type": seq_type,
        "input_length": len(sequence),
        "k": k,
        "total_kmers": total,
        "unique_kmers": len(counts),
        "top_kmers": top_kmers,
        "warnings": warnings,
        "recommended_next_steps": [
            "如需比较样本差异，可继续对比不同序列的 k-mer 偏好分布",
            "如发现异常富集片段，可结合重复扫描或复杂度分析进一步复核",
        ],
    }


def main():
    payload = read_payload()
    result = build_result(payload)
    print(json.dumps(result, ensure_ascii=False), flush=True)


if __name__ == "__main__":
    main()
