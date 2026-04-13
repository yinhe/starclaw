#!/usr/bin/env python3
import json
import re
import sys


DEFAULT_ENZYMES = {
    "EcoRI": "GAATTC",
    "BamHI": "GGATCC",
    "HindIII": "AAGCTT",
    "NotI": "GCGGCCGC",
    "XhoI": "CTCGAG",
}

COMPLEMENT = str.maketrans({
    "A": "T", "T": "A", "C": "G", "G": "C", "U": "A",
    "N": "N", "R": "Y", "Y": "R", "W": "W", "S": "S",
    "K": "M", "M": "K", "B": "V", "V": "B", "D": "H", "H": "D",
})


def read_payload():
    return json.load(sys.stdin)


def normalize_sequence(raw: str):
    kept = []
    for line in (raw or "").splitlines():
        if line.startswith(">"):
            continue
        kept.extend(re.findall(r"[A-Za-z]", line))
    return "".join(kept).upper().replace("U", "T")


def reverse_complement(seq: str):
    return seq.translate(COMPLEMENT)[::-1]


def normalize_enzymes(raw):
    if isinstance(raw, dict):
        return {str(k).strip(): normalize_sequence(str(v)) for k, v in raw.items() if str(k).strip() and normalize_sequence(str(v))}
    if isinstance(raw, list):
        result = {}
        for item in raw:
            text = str(item).strip()
            if not text:
                continue
            if ":" in text:
                name, pattern = text.split(":", 1)
                name = name.strip()
                pattern = normalize_sequence(pattern)
                if name and pattern:
                    result[name] = pattern
            elif text in DEFAULT_ENZYMES:
                result[text] = DEFAULT_ENZYMES[text]
        if result:
            return result
    if isinstance(raw, str):
        result = {}
        for part in re.split(r"[,;\n，；、]", raw):
            text = part.strip()
            if not text:
                continue
            if ":" in text:
                name, pattern = text.split(":", 1)
                name = name.strip()
                pattern = normalize_sequence(pattern)
                if name and pattern:
                    result[name] = pattern
            elif text in DEFAULT_ENZYMES:
                result[text] = DEFAULT_ENZYMES[text]
        if result:
            return result
    return DEFAULT_ENZYMES.copy()


def scan_pattern(sequence: str, pattern: str):
    matches = []
    for match in re.finditer(f"(?=({re.escape(pattern)}))", sequence):
        matches.append({
            "start": match.start() + 1,
            "end": match.start() + len(pattern),
        })
    return matches


def build_result(payload):
    raw_sequence = payload.get("sequence", "")
    sequence_name = payload.get("sequence_name") or "sequence"
    sequence = normalize_sequence(raw_sequence)
    if not sequence:
        raise ValueError("sequence is empty after normalization")

    enzymes = normalize_enzymes(payload.get("enzymes"))
    results = []
    for name, pattern in enzymes.items():
        forward_matches = scan_pattern(sequence, pattern)
        reverse_pattern = reverse_complement(pattern)
        reverse_matches = [] if reverse_pattern == pattern else scan_pattern(sequence, reverse_pattern)
        results.append({
            "enzyme": name,
            "pattern": pattern,
            "reverse_pattern": reverse_pattern,
            "count": len(forward_matches),
            "forward_matches": forward_matches[:20],
            "reverse_count": len(reverse_matches),
            "reverse_matches": reverse_matches[:20],
        })
    results.sort(key=lambda item: (-item["count"], item["enzyme"]))

    total_cuts = sum(item["count"] for item in results)
    warnings = []
    if total_cuts == 0:
        warnings.append("未检测到所选限制性位点，可尝试更换酶或确认序列版本")
    if len(sequence) < 50:
        warnings.append("序列较短，限制性位点分布参考意义有限")

    return {
        "panel": "restriction_site_scan",
        "sequence_name": sequence_name,
        "sequence_type": (payload.get("sequence_type_hint") or "dna").strip().lower() or "dna",
        "input_length": len(sequence),
        "enzyme_count": len(results),
        "total_matches": total_cuts,
        "results": results,
        "warnings": warnings,
        "recommended_next_steps": [
            "如需实验设计，建议结合载体 MCS、片段长度和方向性进一步筛选酶位点",
            "如命中较多，可继续规划消化组合或避免切断目标功能区",
        ],
    }


def main():
    payload = read_payload()
    result = build_result(payload)
    print(json.dumps(result, ensure_ascii=False), flush=True)


if __name__ == "__main__":
    main()
