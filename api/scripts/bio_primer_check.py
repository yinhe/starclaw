#!/usr/bin/env python3
import json
import re
import sys


COMPLEMENT = str.maketrans({
    "A": "T", "T": "A", "C": "G", "G": "C", "U": "A",
    "N": "N", "R": "Y", "Y": "R", "W": "W", "S": "S",
    "K": "M", "M": "K", "B": "V", "V": "B", "D": "H", "H": "D",
})


def read_payload():
    return json.load(sys.stdin)


def normalize_sequence(raw: str):
    return "".join(re.findall(r"[A-Za-z]", raw or "")).upper().replace("U", "T")


def reverse_complement(seq: str):
    return seq.translate(COMPLEMENT)[::-1]


def gc_percent(seq: str):
    if not seq:
        return 0.0
    gc = sum(1 for base in seq if base in {"G", "C"})
    return round(gc / len(seq) * 100, 2)


def tm_wallace(seq: str):
    counts = {base: seq.count(base) for base in "ATGC"}
    return 2 * (counts["A"] + counts["T"]) + 4 * (counts["G"] + counts["C"])


def longest_homopolymer(seq: str):
    best = 0
    current = 0
    last = ""
    for base in seq:
        if base == last:
            current += 1
        else:
            current = 1
            last = base
        best = max(best, current)
    return best


def longest_suffix_prefix_match(a: str, b: str):
    max_len = min(len(a), len(b))
    for size in range(max_len, 0, -1):
        if a[-size:] == b[:size]:
            return size
    return 0


def analyze_primer(name: str, seq: str):
    warnings = []
    invalid = sorted({base for base in seq if base not in set("ATGCNRYWSKMBDHV")})
    if invalid:
        warnings.append("存在非标准碱基字符：" + ", ".join(invalid))
    if len(seq) < 18:
        warnings.append("长度偏短，特异性可能不足")
    if len(seq) > 30:
        warnings.append("长度偏长，退火与扩增条件可能更敏感")
    gc = gc_percent(seq)
    if gc < 35 or gc > 65:
        warnings.append("GC% 不在常见推荐范围（35-65%）")
    tm = tm_wallace(seq)
    if tm < 50 or tm > 72:
        warnings.append("Tm 可能偏离常见 PCR 推荐范围")
    homopolymer = longest_homopolymer(seq)
    if homopolymer >= 4:
        warnings.append(f"存在长度 {homopolymer} 的连续同碱基区段")
    if seq.endswith(("G", "C")):
        clamp = "present"
    else:
        clamp = "absent"
        warnings.append("3' 端缺少 GC clamp")
    self_comp = longest_suffix_prefix_match(seq, reverse_complement(seq))
    if self_comp >= 4:
        warnings.append(f"存在约 {self_comp} bp 的自互补风险")
    return {
        "name": name,
        "sequence": seq,
        "length": len(seq),
        "gc_percent": gc,
        "tm_wallace": tm,
        "gc_clamp": clamp,
        "self_complementarity": self_comp,
        "warnings": warnings,
    }


def locate_amplicon(template: str, forward: str, reverse: str):
    if not template or not forward or not reverse:
        return None
    forward_index = template.find(forward)
    reverse_rc = reverse_complement(reverse)
    reverse_index = template.find(reverse_rc, forward_index + len(forward))
    if forward_index == -1 or reverse_index == -1:
        return None
    start = forward_index + 1
    end = reverse_index + len(reverse_rc)
    return {
        "start": start,
        "end": end,
        "amplicon_size": end - start + 1,
    }


def build_result(payload):
    target_name = payload.get("target_name") or "target"
    forward = normalize_sequence(payload.get("forward_primer", ""))
    reverse = normalize_sequence(payload.get("reverse_primer", ""))
    template = normalize_sequence(payload.get("template_sequence", ""))
    if not forward:
        raise ValueError("forward_primer is empty after normalization")

    forward_result = analyze_primer("forward", forward)
    reverse_result = analyze_primer("reverse", reverse) if reverse else None
    warnings = []
    if reverse_result:
        delta_tm = abs(forward_result["tm_wallace"] - reverse_result["tm_wallace"])
        if delta_tm > 3:
            warnings.append(f"正反向引物 Tm 差值为 {delta_tm}，建议尽量控制在 3℃ 以内")
        dimer_risk = longest_suffix_prefix_match(forward, reverse_complement(reverse))
        if dimer_risk >= 4:
            warnings.append(f"正反向引物存在约 {dimer_risk} bp 的互补风险")
    amplicon = locate_amplicon(template, forward, reverse) if reverse else None
    if template and reverse and not amplicon:
        warnings.append("未在模板中同时定位到正反向引物，可能需要检查方向、模板版本或允许错配")

    return {
        "panel": "primer_check",
        "target_name": target_name,
        "sequence_type": (payload.get("sequence_type_hint") or "dna").strip().lower() or "dna",
        "forward_primer": forward_result,
        "reverse_primer": reverse_result,
        "amplicon": amplicon,
        "warnings": warnings,
        "recommended_next_steps": [
            "如用于实验设计，建议再做特异性检索与二聚体/发卡专业评估",
            "如已有模板定位结果，可继续核对扩增片段长度和目标区域覆盖情况",
        ],
    }


def main():
    payload = read_payload()
    result = build_result(payload)
    print(json.dumps(result, ensure_ascii=False), flush=True)


if __name__ == "__main__":
    main()
