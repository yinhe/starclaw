#!/usr/bin/env python3
import json
import re
import sys
from collections import Counter


def read_payload():
    return json.load(sys.stdin)


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


def normalize_sequence_lines(lines):
    kept = []
    removed = 0
    for line in lines:
        letters = re.findall(r"[A-Za-z\*]", line)
        removed += len(line) - len(letters)
        kept.extend(letters)
    return "".join(kept).upper(), removed


def parse_fasta(raw: str):
    lines = raw.splitlines()
    headers = []
    current = []
    records = []
    nonempty_lines = [line for line in lines if line.strip()]
    plain_sequence_mode = bool(nonempty_lines) and not nonempty_lines[0].startswith(">")

    if plain_sequence_mode:
        sequence, removed = normalize_sequence_lines(nonempty_lines)
        if sequence:
            records.append({"header": "sequence", "sequence": sequence, "removed": removed})
        return records, False, plain_sequence_mode

    removed_total = 0
    for line in lines:
        stripped = line.strip()
        if not stripped:
            continue
        if stripped.startswith(">"):
            if headers or current:
                sequence, removed = normalize_sequence_lines(current)
                removed_total += removed
                records.append({"header": headers[-1] if headers else "sequence", "sequence": sequence, "removed": removed})
                current = []
            headers.append(stripped[1:].strip() or f"record_{len(headers)+1}")
        else:
            current.append(stripped)
    if headers or current:
        sequence, removed = normalize_sequence_lines(current)
        removed_total += removed
        records.append({"header": headers[-1] if headers else "sequence", "sequence": sequence, "removed": removed})

    return records, True, plain_sequence_mode


def build_result(payload):
    raw_sequence = payload.get("sequence", "")
    sequence_name = payload.get("sequence_name") or "sequence"
    records, has_headers, plain_sequence_mode = parse_fasta(raw_sequence)
    if not records:
        raise ValueError("sequence is empty after normalization")

    headers = [record["header"] for record in records]
    concatenated = "".join(record["sequence"] for record in records)
    seq_type = classify_sequence(concatenated, payload.get("sequence_type_hint", ""))

    issues = []
    if plain_sequence_mode:
        issues.append("输入未包含 FASTA header，按单条裸序列处理")
    if any(not record["sequence"] for record in records):
        issues.append("至少一条 FASTA 记录没有有效序列内容")
    if len(set(headers)) != len(headers):
        issues.append("存在重复 FASTA header，建议去重或规范命名")
    if seq_type == "mixed":
        issues.append("字符集混合，建议确认是核酸还是蛋白序列")
    if any(record["removed"] > 0 for record in records):
        issues.append("检测到非字母字符，已在规范化时移除")

    recommendations = []
    if has_headers:
        recommendations.append("FASTA header 已识别，可继续做批量统计、BLAST 或下游注释")
    else:
        recommendations.append("如需标准 FASTA，可为序列补充 >header 再保存")
    if len(records) > 1:
        recommendations.append("检测到多条序列，后续分析建议区分每条记录单独处理")
    if seq_type in {"dna", "rna"}:
        recommendations.append("核酸序列可继续做格式转换、比对或数据库检索")
    elif seq_type == "protein":
        recommendations.append("蛋白序列可继续查 UniProt、PDB 或 AlphaFold")

    return {
        "panel": "fasta_sanity_check",
        "sequence_name": sequence_name,
        "is_fasta_like": has_headers,
        "record_count": len(records),
        "headers": headers,
        "sequence_type": seq_type,
        "total_sequence_length": len(concatenated),
        "per_record_lengths": [len(record["sequence"]) for record in records],
        "issues": issues,
        "recommended_next_steps": recommendations,
    }


def main():
    payload = read_payload()
    result = build_result(payload)
    print(json.dumps(result, ensure_ascii=False), flush=True)


if __name__ == "__main__":
    main()
