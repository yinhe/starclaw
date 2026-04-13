#!/usr/bin/env python3
import json
import re
import sys


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

COMPLEMENT = str.maketrans({
    "A": "T", "T": "A", "C": "G", "G": "C", "U": "A",
    "N": "N", "R": "Y", "Y": "R", "W": "W", "S": "S",
    "K": "M", "M": "K", "B": "V", "V": "B", "D": "H", "H": "D",
})

STOP_CODONS = {"TAA", "TAG", "TGA"}


def read_payload():
    return json.load(sys.stdin)


def normalize_sequence(raw: str):
    kept = []
    for line in raw.splitlines():
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


def reverse_complement(seq: str):
    return seq.translate(COMPLEMENT)[::-1]


def translate(seq: str):
    protein = []
    for i in range(0, len(seq), 3):
        codon = seq[i:i + 3]
        if len(codon) < 3:
            break
        aa = CODON_TABLE.get(codon, "X")
        protein.append(aa)
        if aa == "*":
            break
    return "".join(protein)


def scan_strand(seq: str, strand: str, min_aa_length: int):
    results = []
    stop_codons = STOP_CODONS
    for frame in range(3):
        start = None
        for i in range(frame, len(seq) - 2, 3):
            codon = seq[i:i + 3]
            if start is None and codon == "ATG":
                start = i
                continue
            if start is not None and codon in stop_codons:
                nt_seq = seq[start:i + 3]
                aa_seq = translate(nt_seq)
                aa_length = max(len(aa_seq) - 1, 0) if aa_seq.endswith("*") else len(aa_seq)
                if aa_length >= min_aa_length:
                    results.append({
                        "strand": strand,
                        "frame": frame + 1,
                        "start": start + 1,
                        "end": i + 3,
                        "nt_length": len(nt_seq),
                        "aa_length": aa_length,
                        "protein_preview": aa_seq[:40],
                    })
                start = None
    return results


def build_result(payload):
    raw_sequence = payload.get("sequence", "")
    sequence_name = payload.get("sequence_name") or "sequence"
    min_aa_length = int(payload.get("min_aa_length") or 30)
    scan_reverse = bool(payload.get("scan_reverse", True))
    sequence = normalize_sequence(raw_sequence)
    if not sequence:
        raise ValueError("sequence is empty after normalization")

    seq_type = classify_sequence(sequence, payload.get("sequence_type_hint", ""))
    warnings = []
    if seq_type not in {"dna", "rna"}:
        warnings.append("ORF scan 更适合 DNA/RNA 序列；当前字符集可能不是标准核酸")

    dna = sequence.replace("U", "T")
    orfs = scan_strand(dna, "+", min_aa_length)
    if scan_reverse:
        orfs.extend(scan_strand(reverse_complement(dna), "-", min_aa_length))
    orfs.sort(key=lambda item: (-item["aa_length"], item["start"], item["frame"]))

    if not orfs:
        warnings.append("未找到满足长度阈值的完整 ORF，可尝试降低 min_aa_length 或检查序列方向")

    return {
        "panel": "orf_scan",
        "sequence_name": sequence_name,
        "sequence_type": seq_type,
        "input_length": len(sequence),
        "scan_reverse": scan_reverse,
        "min_aa_length": min_aa_length,
        "orf_count": len(orfs),
        "top_orfs": orfs[:10],
        "warnings": warnings,
        "recommended_next_steps": [
            "如命中候选 ORF，可继续做翻译、BLASTP 或保守结构域检索",
            "如未命中，建议确认阅读框、链方向和是否包含完整起止密码子",
        ],
    }


def main():
    payload = read_payload()
    result = build_result(payload)
    print(json.dumps(result, ensure_ascii=False), flush=True)


if __name__ == "__main__":
    main()
