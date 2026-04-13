#!/usr/bin/env python3
import json
import re
import sys
from collections import Counter


CODON_TABLE = {
    "TTT": "F", "TTC": "F", "TTA": "L", "TTG": "L",
    "CTT": "L", "CTC": "L", "CTA": "L", "CTG": "L",
    "ATT": "I", "ATC": "I", "ATA": "I", "ATG": "M",
    "GTT": "V", "GTC": "V", "GTA": "V", "GTG": "V",
    "TCT": "S", "TCC": "S", "TCA": "S", "TCG": "S",
    "CCT": "P", "CCC": "P", "CCA": "P", "CCG": "P",
    "ACT": "T", "ACC": "T", "ACA": "T", "ACG": "T",
    "GCT": "A", "GCC": "A", "GCA": "A", "GCG": "A",
    "TAT": "Y", "TAC": "Y", "TAA": "*", "TAG": "*",
    "CAT": "H", "CAC": "H", "CAA": "Q", "CAG": "Q",
    "AAT": "N", "AAC": "N", "AAA": "K", "AAG": "K",
    "GAT": "D", "GAC": "D", "GAA": "E", "GAG": "E",
    "TGT": "C", "TGC": "C", "TGA": "*", "TGG": "W",
    "CGT": "R", "CGC": "R", "CGA": "R", "CGG": "R",
    "AGT": "S", "AGC": "S", "AGA": "R", "AGG": "R",
    "GGT": "G", "GGC": "G", "GGA": "G", "GGG": "G",
}


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


def build_result(payload):
    raw_sequence = payload.get("sequence", "")
    sequence_name = payload.get("sequence_name") or "sequence"
    sequence = normalize_sequence(raw_sequence)
    if not sequence:
        raise ValueError("sequence is empty after normalization")

    seq_type = classify_sequence(sequence, payload.get("sequence_type_hint", ""))
    frame = int(payload.get("frame") or 1)
    if frame not in {1, 2, 3}:
        frame = 1

    dna = sequence.replace("U", "T")
    codons = []
    for i in range(frame - 1, len(dna) - 2, 3):
        codon = dna[i:i + 3]
        if len(codon) == 3:
            codons.append(codon)

    counts = Counter(codons)
    amino_counts = Counter(CODON_TABLE.get(codon, "X") for codon in codons)
    top_codons = [
        {"codon": codon, "count": count, "amino_acid": CODON_TABLE.get(codon, "X")}
        for codon, count in counts.most_common(12)
    ]

    warnings = []
    if seq_type not in {"dna", "rna"}:
        warnings.append("codon usage 更适合 DNA/RNA 序列；当前字符集可能不是标准核酸")
    if len(dna) < 30:
        warnings.append("序列较短，密码子频率统计代表性有限")
    if not codons:
        warnings.append("未形成完整密码子，建议检查 frame 或序列长度")

    gc3_values = [1 for codon in codons if codon[2] in {"G", "C"}]
    gc3_percent = round(sum(gc3_values) / len(codons) * 100, 2) if codons else 0.0

    return {
        "panel": "codon_usage",
        "sequence_name": sequence_name,
        "sequence_type": seq_type,
        "frame": frame,
        "input_length": len(sequence),
        "codon_count": len(codons),
        "gc3_percent": gc3_percent,
        "top_codons": top_codons,
        "amino_acid_counts": dict(sorted(amino_counts.items())),
        "warnings": warnings,
        "recommended_next_steps": [
            "如用于表达设计，可继续对照宿主偏好做 codon optimization 评估",
            "如需功能解释，可结合 ORF 扫描和翻译结果一起复核阅读框",
        ],
    }


def main():
    payload = read_payload()
    result = build_result(payload)
    print(json.dumps(result, ensure_ascii=False), flush=True)


if __name__ == "__main__":
    main()
