#!/usr/bin/env python3
import json
import re
import sys
from collections import Counter


def read_payload():
    return json.load(sys.stdin)


def normalize_sequence(raw: str):
    lines = raw.splitlines()
    kept = []
    removed = 0
    for line in lines:
        if line.startswith(">"):
            continue
        letters = re.findall(r"[A-Za-z\*]", line)
        removed += len(line) - len(letters)
        kept.extend(letters)
    return "".join(kept).upper(), removed


def classify_sequence(seq: str, hint: str):
    hint = (hint or "").strip().lower()
    if hint in {"dna", "rna", "protein"}:
        return hint

    letters = set(seq)
    dna_letters = set("ACGTNRYWSKMBDHV")
    rna_letters = set("ACGUNRYWSKMBDHV")
    protein_letters = set("ACDEFGHIKLMNPQRSTVWYBXZJUO*")

    if letters and letters.issubset(dna_letters):
        return "dna"
    if letters and letters.issubset(rna_letters):
        return "rna"
    if letters and letters.issubset(protein_letters) and any(c not in set("ACGTUNRYWSKMBDHV") for c in letters):
        return "protein"
    if "U" in letters and "T" not in letters:
        return "rna"
    if "T" in letters and "U" not in letters:
        return "dna"
    return "mixed"


def build_result(payload):
    raw_sequence = payload.get("sequence", "")
    sequence_name = payload.get("sequence_name") or "sequence"
    sequence, removed_count = normalize_sequence(raw_sequence)
    if not sequence:
        raise ValueError("sequence is empty after normalization")

    seq_type = classify_sequence(sequence, payload.get("sequence_type_hint", ""))
    counts = Counter(sequence)
    gc_count = counts.get("G", 0) + counts.get("C", 0)
    gc_denominator = len(sequence) or 1
    gc_percent = round(gc_count / gc_denominator * 100, 2)
    ambiguous_letters = sorted([base for base in counts if base not in set("ACGTUN")])

    warnings = []
    if removed_count > 0:
        warnings.append(f"已移除 {removed_count} 个非字母字符或空白符号")
    if seq_type == "mixed":
        warnings.append("序列字符集混合，建议确认是核酸还是蛋白序列")
    if len(sequence) < 20:
        warnings.append("序列较短，部分下游比对或注释工具可能信息不足")
    if ambiguous_letters:
        warnings.append("存在模糊或扩展字符：" + ", ".join(ambiguous_letters))

    if seq_type in {"dna", "rna"}:
        next_steps = ["可继续做 BLAST/比对/QC", "如为 FASTA/FASTQ，可补充格式校验或批量统计"]
    elif seq_type == "protein":
        next_steps = ["可继续查 UniProt / PDB / AlphaFold 数据库", "可补充保守结构域或功能注释"]
    else:
        next_steps = ["建议先确认序列类型，再决定走核酸还是蛋白分析流程"]

    return {
        "panel": "sequence_basic_stats",
        "sequence_name": sequence_name,
        "sequence_type": seq_type,
        "sequence_length": len(sequence),
        "gc_percent": gc_percent,
        "ambiguous_count": sum(counts[c] for c in ambiguous_letters),
        "base_counts": dict(sorted(counts.items())),
        "warnings": warnings,
        "recommended_next_steps": next_steps,
    }


def main():
    payload = read_payload()
    result = build_result(payload)
    print(json.dumps(result, ensure_ascii=False), flush=True)


if __name__ == "__main__":
    main()
