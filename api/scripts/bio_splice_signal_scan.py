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

    donor_sites = []
    acceptor_sites = []
    for i in range(len(sequence) - 1):
        dinuc = sequence[i:i + 2]
        if dinuc == "GT":
            donor_sites.append({
                "position": i + 1,
                "signal": dinuc,
                "context": sequence[max(0, i - 3):min(len(sequence), i + 7)],
            })
        elif dinuc == "AG":
            acceptor_sites.append({
                "position": i + 1,
                "signal": dinuc,
                "context": sequence[max(0, i - 6):min(len(sequence), i + 4)],
            })

    warnings = []
    if not donor_sites and not acceptor_sites:
        warnings.append("未发现常见 GT/AG 剪接信号，建议复核序列方向或区域类型")
    if len(sequence) < 50:
        warnings.append("序列较短，剪接信号扫描结果仅供快速参考")

    return {
        "panel": "splice_signal_scan",
        "sequence_name": sequence_name,
        "sequence_type": "dna",
        "input_length": len(sequence),
        "donor_site_count": len(donor_sites),
        "acceptor_site_count": len(acceptor_sites),
        "donor_sites": donor_sites[:20],
        "acceptor_sites": acceptor_sites[:20],
        "warnings": warnings,
        "recommended_next_steps": [
            "如用于外显子边界分析，可继续结合转录本注释和基因组浏览器复核",
            "如怀疑剪接异常，可结合变异位点、保守性和实验背景进一步评估",
        ],
    }


def main():
    payload = read_payload()
    result = build_result(payload)
    print(json.dumps(result, ensure_ascii=False), flush=True)


if __name__ == "__main__":
    main()
