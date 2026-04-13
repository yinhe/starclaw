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


def normalize_motif_sizes(raw):
    sizes = []
    if isinstance(raw, list):
        for item in raw:
            try:
                value = int(item)
            except (TypeError, ValueError):
                continue
            if 1 <= value <= 6:
                sizes.append(value)
    elif raw is not None:
        try:
            value = int(raw)
        except (TypeError, ValueError):
            value = 0
        if 1 <= value <= 6:
            sizes.append(value)
    return sorted(set(sizes or [1, 2, 3]))


def scan_repeats(sequence: str, motif_sizes, min_repeat_count: int):
    hits = []
    seen = set()
    for motif_size in motif_sizes:
        max_start = len(sequence) - motif_size * min_repeat_count + 1
        for start in range(max(max_start, 0)):
            motif = sequence[start:start + motif_size]
            if len(set(motif)) == 1 and motif_size > 1:
                continue
            count = 1
            pos = start + motif_size
            while sequence[pos:pos + motif_size] == motif:
                count += 1
                pos += motif_size
            if count < min_repeat_count:
                continue
            key = (start, motif_size, motif, count)
            if key in seen:
                continue
            seen.add(key)
            hits.append({
                "motif": motif,
                "motif_size": motif_size,
                "repeat_count": count,
                "start": start + 1,
                "end": start + motif_size * count,
                "span": motif_size * count,
            })
    hits.sort(key=lambda item: (-item["span"], -item["repeat_count"], item["start"], item["motif"]))
    return hits


def build_result(payload):
    raw_sequence = payload.get("sequence", "")
    sequence_name = payload.get("sequence_name") or "sequence"
    sequence = normalize_sequence(raw_sequence)
    if not sequence:
        raise ValueError("sequence is empty after normalization")

    seq_type = classify_sequence(sequence, payload.get("sequence_type_hint", ""))
    motif_sizes = normalize_motif_sizes(payload.get("motif_sizes"))
    min_repeat_count = int(payload.get("min_repeat_count") or 3)
    if min_repeat_count < 2:
        min_repeat_count = 2

    hits = scan_repeats(sequence, motif_sizes, min_repeat_count)
    summary = {}
    for item in hits[:50]:
        key = str(item["motif_size"])
        summary[key] = summary.get(key, 0) + 1

    warnings = []
    if not hits:
        warnings.append("未发现满足阈值的串联重复，可尝试放宽 motif size 或 repeat count")
    if len(sequence) < 30:
        warnings.append("序列较短，重复扫描结果代表性有限")

    return {
        "panel": "repeat_scan",
        "sequence_name": sequence_name,
        "sequence_type": seq_type,
        "input_length": len(sequence),
        "motif_sizes": motif_sizes,
        "min_repeat_count": min_repeat_count,
        "repeat_hit_count": len(hits),
        "repeat_summary_by_motif_size": summary,
        "top_repeats": hits[:20],
        "warnings": warnings,
        "recommended_next_steps": [
            "如重复区段较长，可继续人工检查是否影响引物、扩增或比对稳定性",
            "如怀疑功能性重复，可结合文献、浏览器轨道或蛋白注释继续复核",
        ],
    }


def main():
    payload = read_payload()
    result = build_result(payload)
    print(json.dumps(result, ensure_ascii=False), flush=True)


if __name__ == "__main__":
    main()
