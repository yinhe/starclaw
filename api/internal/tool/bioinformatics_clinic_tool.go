package tool

import (
	"context"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
)

type BioinformaticsClinicTool struct {
	db *gorm.DB
}

func NewBioinformaticsClinicTool(db *gorm.DB) *BioinformaticsClinicTool {
	return &BioinformaticsClinicTool{db: db}
}

func (t *BioinformaticsClinicTool) Execute(userID string, params map[string]interface{}) (string, error) {
	action, _ := params["action"].(string)
	switch action {
	case "sequence_basic_stats":
		return t.sequenceBasicStats(params)
	case "fasta_sanity_check":
		return t.fastaSanityCheck(params)
	case "sequence_transform":
		return t.sequenceTransform(params)
	case "orf_scan":
		return t.orfScan(params)
	case "primer_check":
		return t.primerCheck(params)
	case "motif_scan":
		return t.motifScan(params)
	case "restriction_site_scan":
		return t.restrictionSiteScan(params)
	case "codon_usage":
		return t.codonUsage(params)
	case "gc_window_scan":
		return t.gcWindowScan(params)
	case "repeat_scan":
		return t.repeatScan(params)
	case "protein_property_scan":
		return t.proteinPropertyScan(params)
	case "kmer_profile":
		return t.kmerProfile(params)
	case "sequence_complexity_scan":
		return t.sequenceComplexityScan(params)
	case "cpg_island_scan":
		return t.cpgIslandScan(params)
	case "splice_signal_scan":
		return t.spliceSignalScan(params)
	case "polya_signal_scan":
		return t.polyaSignalScan(params)
	case "nucleotide_skew_scan":
		return t.nucleotideSkewScan(params)
	case "promoter_motif_scan":
		return t.promoterMotifScan(params)
	case "kozak_scan":
		return t.kozakScan(params)
	case "uorf_scan":
		return t.uorfScan(params)
	case "gc_clamp_scan":
		return t.gcClampScan(params)
	case "utr_motif_scan":
		return t.utrMotifScan(params)
	case "polypyrimidine_tract_scan":
		return t.polypyrimidineTractScan(params)
	default:
		return "", fmt.Errorf("unknown bioinformatics_clinic action: %s", action)
	}
}

func (t *BioinformaticsClinicTool) sequenceBasicStats(params map[string]interface{}) (string, error) {
	sequence, _ := params["sequence"].(string)
	sequenceName, _ := params["sequence_name"].(string)
	typeHint, _ := params["sequence_type_hint"].(string)
	if strings.TrimSpace(sequence) == "" {
		return "", fmt.Errorf("sequence is required")
	}

	payload := map[string]interface{}{
		"sequence":           sequence,
		"sequence_name":      strings.TrimSpace(sequenceName),
		"sequence_type_hint": strings.TrimSpace(typeHint),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	return runPythonJSONScript(ctx, "bio_sequence_stats.py", payload)
}

func (t *BioinformaticsClinicTool) fastaSanityCheck(params map[string]interface{}) (string, error) {
	sequence, _ := params["sequence"].(string)
	sequenceName, _ := params["sequence_name"].(string)
	typeHint, _ := params["sequence_type_hint"].(string)
	if strings.TrimSpace(sequence) == "" {
		return "", fmt.Errorf("sequence is required")
	}

	payload := map[string]interface{}{
		"sequence":           sequence,
		"sequence_name":      strings.TrimSpace(sequenceName),
		"sequence_type_hint": strings.TrimSpace(typeHint),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	return runPythonJSONScript(ctx, "bio_fasta_sanity.py", payload)
}

func (t *BioinformaticsClinicTool) sequenceTransform(params map[string]interface{}) (string, error) {
	sequence, _ := params["sequence"].(string)
	sequenceName, _ := params["sequence_name"].(string)
	transform, _ := params["transform"].(string)
	typeHint, _ := params["sequence_type_hint"].(string)
	if strings.TrimSpace(sequence) == "" {
		return "", fmt.Errorf("sequence is required")
	}

	payload := map[string]interface{}{
		"sequence":           sequence,
		"sequence_name":      strings.TrimSpace(sequenceName),
		"transform":          strings.TrimSpace(transform),
		"sequence_type_hint": strings.TrimSpace(typeHint),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	return runPythonJSONScript(ctx, "bio_sequence_transform.py", payload)
}

func (t *BioinformaticsClinicTool) orfScan(params map[string]interface{}) (string, error) {
	sequence, _ := params["sequence"].(string)
	sequenceName, _ := params["sequence_name"].(string)
	typeHint, _ := params["sequence_type_hint"].(string)
	if strings.TrimSpace(sequence) == "" {
		return "", fmt.Errorf("sequence is required")
	}

	payload := map[string]interface{}{
		"sequence":           sequence,
		"sequence_name":      strings.TrimSpace(sequenceName),
		"sequence_type_hint": strings.TrimSpace(typeHint),
		"min_aa_length":      params["min_aa_length"],
		"scan_reverse":       params["scan_reverse"],
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	return runPythonJSONScript(ctx, "bio_orf_scan.py", payload)
}

func (t *BioinformaticsClinicTool) primerCheck(params map[string]interface{}) (string, error) {
	forwardPrimer, _ := params["forward_primer"].(string)
	reversePrimer, _ := params["reverse_primer"].(string)
	templateSequence, _ := params["template_sequence"].(string)
	targetName, _ := params["target_name"].(string)
	typeHint, _ := params["sequence_type_hint"].(string)
	if strings.TrimSpace(forwardPrimer) == "" {
		return "", fmt.Errorf("forward_primer is required")
	}

	payload := map[string]interface{}{
		"forward_primer":     strings.TrimSpace(forwardPrimer),
		"reverse_primer":     strings.TrimSpace(reversePrimer),
		"template_sequence":  strings.TrimSpace(templateSequence),
		"target_name":        strings.TrimSpace(targetName),
		"sequence_type_hint": strings.TrimSpace(typeHint),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	return runPythonJSONScript(ctx, "bio_primer_check.py", payload)
}

func (t *BioinformaticsClinicTool) motifScan(params map[string]interface{}) (string, error) {
	sequence, _ := params["sequence"].(string)
	sequenceName, _ := params["sequence_name"].(string)
	typeHint, _ := params["sequence_type_hint"].(string)
	if strings.TrimSpace(sequence) == "" {
		return "", fmt.Errorf("sequence is required")
	}

	payload := map[string]interface{}{
		"sequence":           sequence,
		"sequence_name":      strings.TrimSpace(sequenceName),
		"sequence_type_hint": strings.TrimSpace(typeHint),
		"motifs":             params["motifs"],
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	return runPythonJSONScript(ctx, "bio_motif_scan.py", payload)
}

func (t *BioinformaticsClinicTool) restrictionSiteScan(params map[string]interface{}) (string, error) {
	sequence, _ := params["sequence"].(string)
	sequenceName, _ := params["sequence_name"].(string)
	typeHint, _ := params["sequence_type_hint"].(string)
	if strings.TrimSpace(sequence) == "" {
		return "", fmt.Errorf("sequence is required")
	}

	payload := map[string]interface{}{
		"sequence":           sequence,
		"sequence_name":      strings.TrimSpace(sequenceName),
		"sequence_type_hint": strings.TrimSpace(typeHint),
		"enzymes":            params["enzymes"],
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	return runPythonJSONScript(ctx, "bio_restriction_site_scan.py", payload)
}

func (t *BioinformaticsClinicTool) codonUsage(params map[string]interface{}) (string, error) {
	sequence, _ := params["sequence"].(string)
	sequenceName, _ := params["sequence_name"].(string)
	typeHint, _ := params["sequence_type_hint"].(string)
	if strings.TrimSpace(sequence) == "" {
		return "", fmt.Errorf("sequence is required")
	}

	payload := map[string]interface{}{
		"sequence":           sequence,
		"sequence_name":      strings.TrimSpace(sequenceName),
		"sequence_type_hint": strings.TrimSpace(typeHint),
		"frame":              params["frame"],
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	return runPythonJSONScript(ctx, "bio_codon_usage.py", payload)
}

func (t *BioinformaticsClinicTool) gcWindowScan(params map[string]interface{}) (string, error) {
	sequence, _ := params["sequence"].(string)
	sequenceName, _ := params["sequence_name"].(string)
	typeHint, _ := params["sequence_type_hint"].(string)
	if strings.TrimSpace(sequence) == "" {
		return "", fmt.Errorf("sequence is required")
	}

	payload := map[string]interface{}{
		"sequence":           sequence,
		"sequence_name":      strings.TrimSpace(sequenceName),
		"sequence_type_hint": strings.TrimSpace(typeHint),
		"window_size":        params["window_size"],
		"step_size":          params["step_size"],
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	return runPythonJSONScript(ctx, "bio_gc_window_scan.py", payload)
}

func (t *BioinformaticsClinicTool) repeatScan(params map[string]interface{}) (string, error) {
	sequence, _ := params["sequence"].(string)
	sequenceName, _ := params["sequence_name"].(string)
	typeHint, _ := params["sequence_type_hint"].(string)
	if strings.TrimSpace(sequence) == "" {
		return "", fmt.Errorf("sequence is required")
	}

	payload := map[string]interface{}{
		"sequence":           sequence,
		"sequence_name":      strings.TrimSpace(sequenceName),
		"sequence_type_hint": strings.TrimSpace(typeHint),
		"motif_sizes":        params["motif_sizes"],
		"min_repeat_count":   params["min_repeat_count"],
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	return runPythonJSONScript(ctx, "bio_repeat_scan.py", payload)
}

func (t *BioinformaticsClinicTool) proteinPropertyScan(params map[string]interface{}) (string, error) {
	sequence, _ := params["sequence"].(string)
	sequenceName, _ := params["sequence_name"].(string)
	typeHint, _ := params["sequence_type_hint"].(string)
	if strings.TrimSpace(sequence) == "" {
		return "", fmt.Errorf("sequence is required")
	}

	payload := map[string]interface{}{
		"sequence":           sequence,
		"sequence_name":      strings.TrimSpace(sequenceName),
		"sequence_type_hint": strings.TrimSpace(typeHint),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	return runPythonJSONScript(ctx, "bio_protein_property_scan.py", payload)
}

func (t *BioinformaticsClinicTool) kmerProfile(params map[string]interface{}) (string, error) {
	sequence, _ := params["sequence"].(string)
	sequenceName, _ := params["sequence_name"].(string)
	typeHint, _ := params["sequence_type_hint"].(string)
	if strings.TrimSpace(sequence) == "" {
		return "", fmt.Errorf("sequence is required")
	}

	payload := map[string]interface{}{
		"sequence":           sequence,
		"sequence_name":      strings.TrimSpace(sequenceName),
		"sequence_type_hint": strings.TrimSpace(typeHint),
		"k":                  params["k"],
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	return runPythonJSONScript(ctx, "bio_kmer_profile.py", payload)
}

func (t *BioinformaticsClinicTool) sequenceComplexityScan(params map[string]interface{}) (string, error) {
	sequence, _ := params["sequence"].(string)
	sequenceName, _ := params["sequence_name"].(string)
	typeHint, _ := params["sequence_type_hint"].(string)
	if strings.TrimSpace(sequence) == "" {
		return "", fmt.Errorf("sequence is required")
	}

	payload := map[string]interface{}{
		"sequence":           sequence,
		"sequence_name":      strings.TrimSpace(sequenceName),
		"sequence_type_hint": strings.TrimSpace(typeHint),
		"window_size":        params["window_size"],
		"step_size":          params["step_size"],
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	return runPythonJSONScript(ctx, "bio_sequence_complexity_scan.py", payload)
}

func (t *BioinformaticsClinicTool) cpgIslandScan(params map[string]interface{}) (string, error) {
	sequence, _ := params["sequence"].(string)
	sequenceName, _ := params["sequence_name"].(string)
	typeHint, _ := params["sequence_type_hint"].(string)
	if strings.TrimSpace(sequence) == "" {
		return "", fmt.Errorf("sequence is required")
	}

	payload := map[string]interface{}{
		"sequence":           sequence,
		"sequence_name":      strings.TrimSpace(sequenceName),
		"sequence_type_hint": strings.TrimSpace(typeHint),
		"window_size":        params["window_size"],
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	return runPythonJSONScript(ctx, "bio_cpg_island_scan.py", payload)
}

func (t *BioinformaticsClinicTool) spliceSignalScan(params map[string]interface{}) (string, error) {
	sequence, _ := params["sequence"].(string)
	sequenceName, _ := params["sequence_name"].(string)
	typeHint, _ := params["sequence_type_hint"].(string)
	if strings.TrimSpace(sequence) == "" {
		return "", fmt.Errorf("sequence is required")
	}

	payload := map[string]interface{}{
		"sequence":           sequence,
		"sequence_name":      strings.TrimSpace(sequenceName),
		"sequence_type_hint": strings.TrimSpace(typeHint),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	return runPythonJSONScript(ctx, "bio_splice_signal_scan.py", payload)
}

func (t *BioinformaticsClinicTool) polyaSignalScan(params map[string]interface{}) (string, error) {
	sequence, _ := params["sequence"].(string)
	sequenceName, _ := params["sequence_name"].(string)
	typeHint, _ := params["sequence_type_hint"].(string)
	if strings.TrimSpace(sequence) == "" {
		return "", fmt.Errorf("sequence is required")
	}

	payload := map[string]interface{}{
		"sequence":           sequence,
		"sequence_name":      strings.TrimSpace(sequenceName),
		"sequence_type_hint": strings.TrimSpace(typeHint),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	return runPythonJSONScript(ctx, "bio_polya_signal_scan.py", payload)
}

func (t *BioinformaticsClinicTool) nucleotideSkewScan(params map[string]interface{}) (string, error) {
	sequence, _ := params["sequence"].(string)
	sequenceName, _ := params["sequence_name"].(string)
	typeHint, _ := params["sequence_type_hint"].(string)
	if strings.TrimSpace(sequence) == "" {
		return "", fmt.Errorf("sequence is required")
	}

	payload := map[string]interface{}{
		"sequence":           sequence,
		"sequence_name":      strings.TrimSpace(sequenceName),
		"sequence_type_hint": strings.TrimSpace(typeHint),
		"window_size":        params["window_size"],
		"step_size":          params["step_size"],
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	return runPythonJSONScript(ctx, "bio_nucleotide_skew_scan.py", payload)
}

func (t *BioinformaticsClinicTool) promoterMotifScan(params map[string]interface{}) (string, error) {
	sequence, _ := params["sequence"].(string)
	sequenceName, _ := params["sequence_name"].(string)
	typeHint, _ := params["sequence_type_hint"].(string)
	if strings.TrimSpace(sequence) == "" {
		return "", fmt.Errorf("sequence is required")
	}

	payload := map[string]interface{}{
		"sequence":           sequence,
		"sequence_name":      strings.TrimSpace(sequenceName),
		"sequence_type_hint": strings.TrimSpace(typeHint),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	return runPythonJSONScript(ctx, "bio_promoter_motif_scan.py", payload)
}

func (t *BioinformaticsClinicTool) kozakScan(params map[string]interface{}) (string, error) {
	sequence, _ := params["sequence"].(string)
	sequenceName, _ := params["sequence_name"].(string)
	typeHint, _ := params["sequence_type_hint"].(string)
	if strings.TrimSpace(sequence) == "" {
		return "", fmt.Errorf("sequence is required")
	}

	payload := map[string]interface{}{
		"sequence":           sequence,
		"sequence_name":      strings.TrimSpace(sequenceName),
		"sequence_type_hint": strings.TrimSpace(typeHint),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	return runPythonJSONScript(ctx, "bio_kozak_scan.py", payload)
}

func (t *BioinformaticsClinicTool) uorfScan(params map[string]interface{}) (string, error) {
	sequence, _ := params["sequence"].(string)
	sequenceName, _ := params["sequence_name"].(string)
	typeHint, _ := params["sequence_type_hint"].(string)
	if strings.TrimSpace(sequence) == "" {
		return "", fmt.Errorf("sequence is required")
	}

	payload := map[string]interface{}{
		"sequence":           sequence,
		"sequence_name":      strings.TrimSpace(sequenceName),
		"sequence_type_hint": strings.TrimSpace(typeHint),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	return runPythonJSONScript(ctx, "bio_uorf_scan.py", payload)
}

func (t *BioinformaticsClinicTool) gcClampScan(params map[string]interface{}) (string, error) {
	sequence, _ := params["sequence"].(string)
	sequenceName, _ := params["sequence_name"].(string)
	typeHint, _ := params["sequence_type_hint"].(string)
	if strings.TrimSpace(sequence) == "" {
		return "", fmt.Errorf("sequence is required")
	}

	payload := map[string]interface{}{
		"sequence":           sequence,
		"sequence_name":      strings.TrimSpace(sequenceName),
		"sequence_type_hint": strings.TrimSpace(typeHint),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	return runPythonJSONScript(ctx, "bio_gc_clamp_scan.py", payload)
}

func (t *BioinformaticsClinicTool) utrMotifScan(params map[string]interface{}) (string, error) {
	sequence, _ := params["sequence"].(string)
	sequenceName, _ := params["sequence_name"].(string)
	typeHint, _ := params["sequence_type_hint"].(string)
	if strings.TrimSpace(sequence) == "" {
		return "", fmt.Errorf("sequence is required")
	}

	payload := map[string]interface{}{
		"sequence":           sequence,
		"sequence_name":      strings.TrimSpace(sequenceName),
		"sequence_type_hint": strings.TrimSpace(typeHint),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	return runPythonJSONScript(ctx, "bio_utr_motif_scan.py", payload)
}

func (t *BioinformaticsClinicTool) polypyrimidineTractScan(params map[string]interface{}) (string, error) {
	sequence, _ := params["sequence"].(string)
	sequenceName, _ := params["sequence_name"].(string)
	typeHint, _ := params["sequence_type_hint"].(string)
	if strings.TrimSpace(sequence) == "" {
		return "", fmt.Errorf("sequence is required")
	}

	payload := map[string]interface{}{
		"sequence":           sequence,
		"sequence_name":      strings.TrimSpace(sequenceName),
		"sequence_type_hint": strings.TrimSpace(typeHint),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	return runPythonJSONScript(ctx, "bio_polypyrimidine_tract_scan.py", payload)
}
