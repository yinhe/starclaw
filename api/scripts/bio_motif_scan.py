#!/usr/bin/env python3
import json
import re
import sys


DNA_IUPAC = {
    "A": "A", "C": "C", "G": "G", "T": "T", "U": "U",
    "R": "[AG]", "Y": "[CTU]", "S": "[GC]", "W": "[ATU]",
    "K": "[GTU]", "M": "[AC]", "B": "[CGTU]", "D": "[AGTU]",
    "H": "[ACTU]", "V": "[ACG]", "N": "[ACGTU]",
}

DEFAULT_DNA_MOTIFS = ["ATG", "TATAAA", "AATAAA", "CCGCGG", "GGATCC"]
DEFAULT_PROTEIN_MOTIFS = ["RGD", "N[^P][ST][^P]", "KDEL"]


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
    if letters and letters.issubset(set("ACGTUNRYWSKMBDHV")):
        if "U" in letters and "T" not in letters:
            return "rna"
        return "dna"
    return "protein"


def normalize_motifs(raw, seq_type: str):
    motifs = []
    if isinstance(raw, list):
        motifs = [str(item).strip() for item in raw if str(item).strip()]
    elif isinstance(raw, str):
        for part in re.split(r"[,;\n，；、]", raw):
            part = part.strip()
            if part:
                motifs.append(part)
    if motifs:
        return motifs
    if seq_type in {"dna", "rna"}:
        return DEFAULT_DNA_MOTIFS
    return DEFAULT_PROTEIN_MOTIFS


def motif_to_regex(motif: str, seq_type: str):
    motif = motif.strip().upper()
    if seq_type in {"dna", "rna"} and re.fullmatch(r"[ACGTUNRYWSKMBDHV]+", motif):
        return "".join(DNA_IUPAC.get(ch, re.escape(ch)) for ch in motif)
    return motif


def scan_motif(sequence: str, motif: str, seq_type: str):
    regex = motif_to_regex(motif, seq_type)
    matches = []
    for match in re.finditer(f"(?=({regex}))", sequence):
        text = match.group(1)
        matches.append({
            "start": match.start() + 1,
            "end": match.start() + len(text),
            "match": text,
        })
    return {
        "motif": motif,
        "regex": regex,
        "count": len(matches),
        "matches": matches[:20],
    }


def build_result(payload):
    raw_sequence = payload.get("sequence", "")
    sequence_name = payload.get("sequence_name") or "sequence"
    sequence = normalize_sequence(raw_sequence)
    if not sequence:
        raise ValueError("sequence is empty after normalization")

    seq_type = classify_sequence(sequence, payload.get("sequence_type_hint", ""))
    motifs = normalize_motifs(payload.get("motifs"), seq_type)
    results = [scan_motif(sequence, motif, seq_type) for motif in motifs]
    results.sort(key=lambda item: (-item["count"], item["motif"]))

    warnings = []
    if not any(item["count"] > 0 for item in results):
        warnings.append("未命中所选 motif，可尝试放宽模式或确认序列方向/类型")
    if len(sequence) < 20:
        warnings.append("序列较短，motif 扫描结果可能有限")

    return {
        "panel": "motif_scan",
        "sequence_name": sequence_name,
        "sequence_type": seq_type,
        "input_length": len(sequence),
        "motif_count": len(results),
        "results": results,
        "warnings": warnings,
        "recommended_next_steps": [
            "如命中关键 motif，可继续查 PROSITE、InterPro 或文献背景",
            "如需更复杂模式，建议补充正则或保守位点库再扫描",
        ],
    }


def main():
    payload = read_payload()
    result = build_result(payload)
    print(json.dumps(result, ensure_ascii=False), flush=True)


if __name__ == "__main__":
    main()
