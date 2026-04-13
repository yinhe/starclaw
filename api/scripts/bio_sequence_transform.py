#!/usr/bin/env python3
import json
import re
import sys


DNA_COMPLEMENT = str.maketrans({
    "A": "T",
    "T": "A",
    "C": "G",
    "G": "C",
    "U": "A",
    "N": "N",
    "R": "Y",
    "Y": "R",
    "W": "W",
    "S": "S",
    "K": "M",
    "M": "K",
    "B": "V",
    "V": "B",
    "D": "H",
    "H": "D",
})

RNA_COMPLEMENT = str.maketrans({
    "A": "U",
    "U": "A",
    "C": "G",
    "G": "C",
    "T": "A",
    "N": "N",
    "R": "Y",
    "Y": "R",
    "W": "W",
    "S": "S",
    "K": "M",
    "M": "K",
    "B": "V",
    "V": "B",
    "D": "H",
    "H": "D",
})

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
    lines = raw.splitlines()
    headers = []
    kept = []
    for line in lines:
        if line.startswith(">"):
            headers.append(line[1:].strip())
            continue
        kept.extend(re.findall(r"[A-Za-z\*]", line))
    return "".join(kept).upper(), headers


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


def reverse_complement(seq: str, seq_type: str):
    if seq_type == "rna":
        return seq.translate(RNA_COMPLEMENT)[::-1]
    return seq.translate(DNA_COMPLEMENT)[::-1]


def transcribe(seq: str):
    return seq.replace("T", "U")


def back_transcribe(seq: str):
    return seq.replace("U", "T")


def translate_dna(seq: str):
    dna = back_transcribe(seq) if "U" in seq else seq
    protein = []
    trimmed_length = len(dna) - (len(dna) % 3)
    for i in range(0, trimmed_length, 3):
        codon = dna[i:i+3]
        protein.append(CODON_TABLE.get(codon, "X"))
    return "".join(protein), len(dna) - trimmed_length


def build_result(payload):
    raw_sequence = payload.get("sequence", "")
    sequence_name = payload.get("sequence_name") or "sequence"
    transform = (payload.get("transform") or "upper").strip().lower()
    sequence, headers = normalize_sequence(raw_sequence)
    if not sequence:
        raise ValueError("sequence is empty after normalization")

    seq_type = classify_sequence(sequence, payload.get("sequence_type_hint", ""))
    warnings = []

    if transform == "upper":
        transformed = sequence
    elif transform == "reverse":
        transformed = sequence[::-1]
    elif transform == "reverse_complement":
        if seq_type not in {"dna", "rna"}:
            raise ValueError("reverse_complement requires dna or rna sequence")
        transformed = reverse_complement(sequence, seq_type)
    elif transform == "transcribe":
        if seq_type != "dna":
            raise ValueError("transcribe requires dna sequence")
        transformed = transcribe(sequence)
    elif transform == "back_transcribe":
        if seq_type != "rna":
            raise ValueError("back_transcribe requires rna sequence")
        transformed = back_transcribe(sequence)
    elif transform == "translate":
        if seq_type not in {"dna", "rna"}:
            raise ValueError("translate requires dna or rna sequence")
        transformed, remainder = translate_dna(sequence)
        if remainder:
            warnings.append(f"末尾有 {remainder} 个碱基未参与翻译")
    else:
        raise ValueError(f"unsupported transform: {transform}")

    if headers:
        warnings.append("原始输入包含 FASTA header，结果仅返回规范化后的序列文本")

    return {
        "panel": "sequence_transform",
        "sequence_name": sequence_name,
        "sequence_type": seq_type,
        "transform": transform,
        "input_length": len(sequence),
        "output_length": len(transformed),
        "output_sequence": transformed,
        "warnings": warnings,
        "recommended_next_steps": [
            "如需保存为 FASTA，可补充 header 后写回文件",
            "转换后建议再做一次基础统计或数据库检索确认",
        ],
    }


def main():
    payload = read_payload()
    result = build_result(payload)
    print(json.dumps(result, ensure_ascii=False), flush=True)


if __name__ == "__main__":
    main()
