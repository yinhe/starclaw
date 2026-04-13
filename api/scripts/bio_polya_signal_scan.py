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


def build_result(payload):
    raw_sequence = payload.get("sequence", "")
    sequence_name = payload.get("sequence_name") or "sequence"
    sequence = normalize_sequence(raw_sequence)
    if not sequence:
        raise ValueError("sequence is empty after normalization")

    motifs = ["AATAAA", "ATTAAA", "TATAAA", "AGTAAA", "AAGAAA", "CATAAA"]
    hits = []
    for motif in motifs:
        for match in re.finditer(motif, sequence):
            start = match.start()
            hits.append({
                "motif": motif,
                "start": start + 1,
                "end": start + len(motif),
                "context": sequence[max(0, start - 10):min(len(sequence), start + len(motif) + 10)],
            })

    hits.sort(key=lambda item: (item["start"], item["motif"]))

    warnings = []
    if not hits:
        warnings.append("未发现常见 poly(A) 信号 motif，可结合转录本注释或延长区域继续复核")
    if len(sequence) < 60:
        warnings.append("序列较短，poly(A) 信号扫描结果仅供快速参考")

    return {
        "panel": "polya_signal_scan",
        "sequence_name": sequence_name,
        "sequence_type": "dna",
        "input_length": len(sequence),
        "signal_count": len(hits),
        "signals": hits[:20],
        "warnings": warnings,
        "recommended_next_steps": [
            "如定位到 poly(A) 信号，可继续结合 3' UTR、转录本注释或 reads 证据复核",
            "如用于构建设计，可进一步检查信号附近是否有下游 U/GU-rich 元件",
        ],
    }


def main():
    payload = read_payload()
    result = build_result(payload)
    print(json.dumps(result, ensure_ascii=False), flush=True)


if __name__ == "__main__":
    main()
