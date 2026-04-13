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


def gc_clamp_length(sequence: str):
    count = 0
    for base in reversed(sequence):
        if base in {"G", "C"}:
            count += 1
        else:
            break
    return count


def build_result(payload):
    raw_sequence = payload.get("sequence", "")
    sequence_name = payload.get("sequence_name") or "sequence"
    sequence = normalize_sequence(raw_sequence)
    if not sequence:
        raise ValueError("sequence is empty after normalization")

    tail = sequence[-5:] if len(sequence) >= 5 else sequence
    clamp = gc_clamp_length(sequence)
    tail_gc = 0.0
    if tail:
        tail_gc = round(sum(1 for base in tail if base in {"G", "C"}) / len(tail), 4)

    warnings = []
    if clamp == 0:
        warnings.append("3' 端未观察到 GC clamp，可结合 primer 设计目标再复核")
    elif clamp > 3:
        warnings.append("3' 端 GC clamp 偏强，需留意引物二聚体或非特异结合风险")
    if len(sequence) < 12:
        warnings.append("序列较短，GC clamp 结果仅供快速参考")

    return {
        "panel": "gc_clamp_scan",
        "sequence_name": sequence_name,
        "sequence_type": "dna",
        "input_length": len(sequence),
        "tail_window": tail,
        "tail_gc_fraction": tail_gc,
        "gc_clamp_length": clamp,
        "recommended_range": "1-3 nt GC clamp at 3' end",
        "warnings": warnings,
        "recommended_next_steps": [
            "如用于 primer 设计，可继续结合 Tm、二聚体、hairpin 和扩增产物长度综合评估",
            "如 3' 端 GC 过强或过弱，可尝试微调末端碱基后重新复核",
        ],
    }


def main():
    payload = read_payload()
    result = build_result(payload)
    print(json.dumps(result, ensure_ascii=False), flush=True)


if __name__ == "__main__":
    main()
