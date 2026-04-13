#!/usr/bin/env python3
import json
import re
import sys
from collections import Counter


AA_WEIGHTS = {
    "A": 89.09, "R": 174.20, "N": 132.12, "D": 133.10, "C": 121.15,
    "Q": 146.15, "E": 147.13, "G": 75.07, "H": 155.16, "I": 131.17,
    "L": 131.17, "K": 146.19, "M": 149.21, "F": 165.19, "P": 115.13,
    "S": 105.09, "T": 119.12, "W": 204.23, "Y": 181.19, "V": 117.15,
}
HYDROPHOBIC = set("AILMFWVPGC")
CHARGED_POS = set("KRH")
CHARGED_NEG = set("DE")
POLAR = set("STNQY")


def read_payload():
    return json.load(sys.stdin)


def normalize_sequence(raw: str):
    kept = []
    for line in (raw or "").splitlines():
        if line.startswith(">"):
            continue
        kept.extend(re.findall(r"[A-Za-z\*]", line))
    return "".join(kept).upper().replace("*", "")


def build_result(payload):
    raw_sequence = payload.get("sequence", "")
    sequence_name = payload.get("sequence_name") or "protein"
    sequence = normalize_sequence(raw_sequence)
    if not sequence:
        raise ValueError("sequence is empty after normalization")

    counts = Counter(sequence)
    length = len(sequence)
    supported = [aa for aa in sequence if aa in AA_WEIGHTS]
    mw = round(sum(AA_WEIGHTS.get(aa, 0.0) for aa in supported) - max(len(supported) - 1, 0) * 18.015, 2)
    hydrophobic_count = sum(counts.get(aa, 0) for aa in HYDROPHOBIC)
    polar_count = sum(counts.get(aa, 0) for aa in POLAR)
    positive_count = sum(counts.get(aa, 0) for aa in CHARGED_POS)
    negative_count = sum(counts.get(aa, 0) for aa in CHARGED_NEG)
    aromatic_count = sum(counts.get(aa, 0) for aa in "FWY")

    hydrophobic_percent = round(hydrophobic_count / length * 100, 2) if length else 0.0
    polar_percent = round(polar_count / length * 100, 2) if length else 0.0
    net_charge_index = positive_count - negative_count

    composition_rank = [
        {"amino_acid": aa, "count": count, "percent": round(count / length * 100, 2)}
        for aa, count in counts.most_common(10)
    ]

    warnings = []
    non_standard = sorted({aa for aa in sequence if aa not in AA_WEIGHTS})
    if non_standard:
        warnings.append("存在非常规氨基酸字符：" + ", ".join(non_standard))
    if length < 30:
        warnings.append("蛋白序列较短，部分理化特征仅供快速参考")
    if hydrophobic_percent > 55:
        warnings.append("疏水残基占比偏高，可留意跨膜段或聚集倾向")

    return {
        "panel": "protein_property_scan",
        "sequence_name": sequence_name,
        "sequence_type": "protein",
        "input_length": length,
        "molecular_weight_da": mw,
        "hydrophobic_percent": hydrophobic_percent,
        "polar_percent": polar_percent,
        "positive_residue_count": positive_count,
        "negative_residue_count": negative_count,
        "aromatic_residue_count": aromatic_count,
        "net_charge_index": net_charge_index,
        "top_composition": composition_rank,
        "warnings": warnings,
        "recommended_next_steps": [
            "如需功能解释，可继续查 UniProt、InterPro 或结构域数据库",
            "如怀疑跨膜或分泌特征，可进一步做信号肽、跨膜段或亚细胞定位分析",
        ],
    }


def main():
    payload = read_payload()
    result = build_result(payload)
    print(json.dumps(result, ensure_ascii=False), flush=True)


if __name__ == "__main__":
    main()
