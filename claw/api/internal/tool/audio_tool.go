package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/yinhe/starclaw/internal/model"
	"gorm.io/gorm"
)

// AudioTool analyzes audio files for MV production: duration, BPM, energy curve, beat timestamps.
type AudioTool struct {
	db *gorm.DB
}

func NewAudioTool(db *gorm.DB) *AudioTool {
	return &AudioTool{db: db}
}

func (t *AudioTool) Name() string { return "audio_analysis" }

func (t *AudioTool) Description() string {
	return `音频分析工具，用于 MV 制作前的音频智能分析。从音频文件中提取时长、BPM、能量曲线、节拍时间戳等信息，为节拍同步剪辑提供数据。
操作：
- analyze: 分析音频文件（支持 music_id 或 file_url），返回时长、采样率、能量曲线
- detect_beats: 检测节拍时间戳和 BPM（需要 aubio 或 ffmpeg）
- get_energy_curve: 获取逐秒能量曲线（用于场景强度匹配）
- generate_srt: 根据歌词文本 + 音频时长 + 段落结构，自动生成 SRT 字幕时间轴`
}

func (t *AudioTool) Parameters() interface{} {
	return &JSONSchema{
		Type: "object",
		Properties: map[string]Property{
			"action":      {Type: "string", Description: "Action: analyze, detect_beats, get_energy_curve, generate_srt"},
			"music_id":    {Type: "string", Description: "Music record ID (from music_generation or uploaded). Used for analyze/detect_beats/get_energy_curve."},
			"file_url":    {Type: "string", Description: "Direct URL or local path to audio file. Alternative to music_id."},
			"lyrics":      {Type: "string", Description: "For generate_srt: full lyrics text with section markers like [verse1], [chorus], [bridge], [outro]."},
			"duration":    {Type: "string", Description: "For generate_srt: total audio duration in seconds (from analyze result)."},
			"vocal_start": {Type: "string", Description: "For generate_srt: seconds when vocals/singing actually begin (e.g. after instrumental intro). Credit lines (词曲/演唱/制作人) will display before this point. Default: auto-detect."},
			"sections":    {Type: "string", Description: "For generate_srt: JSON array of sections from analyze, e.g. [{\"type\":\"verse1\",\"start\":15.2,\"end\":45.8}]"},
			"interval":    {Type: "string", Description: "For get_energy_curve: sampling interval in seconds (default: 1.0)"},
		},
		Required: []string{"action"},
	}
}

type audioArgs struct {
	Action     string `json:"action"`
	MusicID    string `json:"music_id"`
	FileURL    string `json:"file_url"`
	Lyrics     string `json:"lyrics"`
	Duration   string `json:"duration"`
	VocalStart string `json:"vocal_start"`
	Sections   string `json:"sections"`
	Interval   string `json:"interval"`
}

func (t *AudioTool) Execute(ctx context.Context, argsJSON string) (string, error) {
	var args audioArgs
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %v", err)
	}
	switch args.Action {
	case "analyze":
		return t.analyze(ctx, args)
	case "detect_beats":
		return t.detectBeats(ctx, args)
	case "get_energy_curve":
		return t.getEnergyCurve(ctx, args)
	case "generate_srt":
		return t.generateSRT(ctx, args)
	default:
		return "", fmt.Errorf("unknown action: %s. Use: analyze, detect_beats, get_energy_curve, generate_srt", args.Action)
	}
}

// resolveAudioPath gets a local file path from music_id or file_url
func (t *AudioTool) resolveAudioPath(args audioArgs, tmpDir string) (string, error) {
	if args.MusicID != "" {
		return ResolveMusicPath(t.db, args.MusicID, tmpDir)
	}
	if args.FileURL != "" {
		// Check if it's a local upload path
		if strings.HasPrefix(args.FileURL, "/app/uploads/") || strings.HasPrefix(args.FileURL, "/v1/uploads/") {
			localPath := args.FileURL
			if strings.HasPrefix(localPath, "/v1/uploads/") {
				localPath = "/app/uploads/" + strings.TrimPrefix(localPath, "/v1/uploads/")
			}
			if _, err := os.Stat(localPath); err == nil {
				return localPath, nil
			}
		}
		// Check if it's a local music path
		if strings.HasPrefix(args.FileURL, "/v1/music/") {
			filename := strings.TrimPrefix(args.FileURL, "/v1/music/")
			localPath := filepath.Join(MusicDir(), filename)
			if _, err := os.Stat(localPath); err == nil {
				return localPath, nil
			}
		}
		// Download remote URL
		if strings.HasPrefix(args.FileURL, "http://") || strings.HasPrefix(args.FileURL, "https://") {
			ext := ".wav"
			if strings.Contains(args.FileURL, ".mp3") {
				ext = ".mp3"
			} else if strings.Contains(args.FileURL, ".flac") {
				ext = ".flac"
			}
			dlPath := filepath.Join(tmpDir, "audio"+ext)
			if err := DownloadFile(args.FileURL, dlPath); err != nil {
				return "", fmt.Errorf("failed to download audio: %v", err)
			}
			return dlPath, nil
		}
		// Might be a direct local path
		if _, err := os.Stat(args.FileURL); err == nil {
			return args.FileURL, nil
		}
		return "", fmt.Errorf("cannot resolve file_url: %s", args.FileURL)
	}

	// Try to find the latest music in conversation
	convID := ""
	if cid, ok := context.Background().Value(CtxKeyConversationID).(string); ok {
		convID = cid
	}
	if convID != "" {
		var music model.MusicRecord
		if err := t.db.Where("conversation_id = ? AND status = ?", convID, "succeeded").
			Order("created_at DESC").First(&music).Error; err == nil {
			return ResolveMusicPath(t.db, music.ID, tmpDir)
		}
	}

	return "", fmt.Errorf("music_id or file_url is required")
}

// analyze extracts comprehensive audio metadata using ffprobe
func (t *AudioTool) analyze(ctx context.Context, args audioArgs) (string, error) {
	tmpDir, err := os.MkdirTemp("", "audio-analyze-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmpDir)

	audioPath, err := t.resolveAudioPath(args, tmpDir)
	if err != nil {
		return "", err
	}

	log.Printf("[AudioTool] analyzing: %s", audioPath)

	// ffprobe: duration, sample_rate, channels, codec, bit_rate
	probeCmd := exec.CommandContext(ctx, "ffprobe",
		"-v", "quiet",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		audioPath,
	)
	probeOut, err := probeCmd.Output()
	if err != nil {
		return "", fmt.Errorf("ffprobe failed: %v", err)
	}

	var probeData struct {
		Format struct {
			Duration string `json:"duration"`
			BitRate  string `json:"bit_rate"`
			Tags     struct {
				Title  string `json:"title"`
				Artist string `json:"artist"`
				Album  string `json:"album"`
			} `json:"tags"`
		} `json:"format"`
		Streams []struct {
			CodecName  string `json:"codec_name"`
			SampleRate string `json:"sample_rate"`
			Channels   int    `json:"channels"`
		} `json:"streams"`
	}
	if err := json.Unmarshal(probeOut, &probeData); err != nil {
		return "", fmt.Errorf("failed to parse ffprobe output: %v", err)
	}

	duration, _ := strconv.ParseFloat(probeData.Format.Duration, 64)

	// Get loudness/energy summary via EBU R128
	loudCmd := exec.CommandContext(ctx, "ffmpeg",
		"-i", audioPath,
		"-af", "ebur128=framelog=verbose",
		"-f", "null", "-",
	)
	loudOut, _ := loudCmd.CombinedOutput()
	loudStr := string(loudOut)

	// Parse integrated loudness
	integratedLoudness := ""
	for _, line := range strings.Split(loudStr, "\n") {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "I:") && strings.Contains(line, "LUFS") {
			integratedLoudness = line
			break
		}
	}

	// Get per-second energy using volumedetect
	energyCurve, err := t.computeEnergyCurve(ctx, audioPath, duration, 2.0)
	if err != nil {
		log.Printf("[AudioTool] energy curve failed: %v", err)
	}

	// Try BPM detection via aubio if available
	bpm := 0.0
	bpmCmd := exec.CommandContext(ctx, "aubio", "tempo", "-i", audioPath)
	if bpmOut, err := bpmCmd.Output(); err == nil {
		// aubio outputs one line per beat, last line is summary or we parse the tempo
		lines := strings.Split(strings.TrimSpace(string(bpmOut)), "\n")
		if len(lines) > 0 {
			// Try parsing last line as BPM
			for i := len(lines) - 1; i >= 0; i-- {
				if v, err := strconv.ParseFloat(strings.TrimSpace(lines[i]), 64); err == nil && v > 30 && v < 300 {
					bpm = v
					break
				}
			}
		}
	} else {
		log.Printf("[AudioTool] aubio not available, skipping BPM detection: %v", err)
	}

	// Estimate section count based on duration
	estimatedSections := int(duration / 30)
	if estimatedSections < 4 {
		estimatedSections = 4
	}
	estimatedClips := int(duration / 5)

	result := map[string]interface{}{
		"action":   "analyze",
		"status":   "success",
		"duration": math.Round(duration*100) / 100,
		"duration_formatted": fmt.Sprintf("%d:%02d",
			int(duration)/60, int(duration)%60),
		"sample_rate": "",
		"channels":    0,
		"codec":       "",
		"bit_rate":    probeData.Format.BitRate,
	}

	if len(probeData.Streams) > 0 {
		s := probeData.Streams[0]
		result["sample_rate"] = s.SampleRate
		result["channels"] = s.Channels
		result["codec"] = s.CodecName
	}

	if probeData.Format.Tags.Title != "" {
		result["title"] = probeData.Format.Tags.Title
	}
	if probeData.Format.Tags.Artist != "" {
		result["artist"] = probeData.Format.Tags.Artist
	}

	if bpm > 0 {
		result["bpm"] = math.Round(bpm*10) / 10
		beatInterval := 60.0 / bpm
		result["beat_interval"] = math.Round(beatInterval*1000) / 1000
	}

	if integratedLoudness != "" {
		result["loudness"] = integratedLoudness
	}

	if len(energyCurve) > 0 {
		result["energy_curve"] = energyCurve
		result["energy_summary"] = summarizeEnergy(energyCurve, duration)
	}

	result["estimated_sections"] = estimatedSections
	result["estimated_clips"] = estimatedClips
	result["mv_guidance"] = fmt.Sprintf(
		"这首歌时长 %s，建议分为 %d 个段落，约 %d 个视频片段（每个 3-8 秒）。"+
			"请结合歌词段落和能量曲线设计分镜脚本。",
		result["duration_formatted"], estimatedSections, estimatedClips)

	return toJSON(result), nil
}

// detectBeats returns beat timestamps using aubio or energy-based fallback
func (t *AudioTool) detectBeats(ctx context.Context, args audioArgs) (string, error) {
	tmpDir, err := os.MkdirTemp("", "audio-beats-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmpDir)

	audioPath, err := t.resolveAudioPath(args, tmpDir)
	if err != nil {
		return "", err
	}

	// Try aubio beat detection first
	beatCmd := exec.CommandContext(ctx, "aubio", "beat", "-i", audioPath)
	beatOut, err := beatCmd.Output()

	var beats []float64
	var bpm float64

	if err == nil {
		// aubio outputs one timestamp per line
		for _, line := range strings.Split(strings.TrimSpace(string(beatOut)), "\n") {
			if v, err := strconv.ParseFloat(strings.TrimSpace(line), 64); err == nil {
				beats = append(beats, math.Round(v*1000)/1000)
			}
		}
		if len(beats) >= 2 {
			// Estimate BPM from median inter-beat interval
			var intervals []float64
			for i := 1; i < len(beats); i++ {
				intervals = append(intervals, beats[i]-beats[i-1])
			}
			// Median
			if len(intervals) > 0 {
				median := intervals[len(intervals)/2]
				if median > 0 {
					bpm = 60.0 / median
				}
			}
		}
	} else {
		log.Printf("[AudioTool] aubio not available, using energy-based beat estimation")

		// Fallback: get duration and estimate beats from ffprobe
		probeCmd := exec.CommandContext(ctx, "ffprobe",
			"-v", "quiet", "-show_entries", "format=duration",
			"-of", "default=noprint_wrappers=1:nokey=1", audioPath,
		)
		durOut, err := probeCmd.Output()
		if err != nil {
			return "", fmt.Errorf("ffprobe failed: %v", err)
		}
		duration, _ := strconv.ParseFloat(strings.TrimSpace(string(durOut)), 64)

		// Detect onsets via ffmpeg silencedetect as rough beat markers
		onsetCmd := exec.CommandContext(ctx, "ffmpeg",
			"-i", audioPath,
			"-af", "silencedetect=noise=-30dB:d=0.3",
			"-f", "null", "-",
		)
		onsetOut, _ := onsetCmd.CombinedOutput()
		onsetStr := string(onsetOut)

		// Parse silence_end timestamps as onset markers
		for _, line := range strings.Split(onsetStr, "\n") {
			if strings.Contains(line, "silence_end:") {
				parts := strings.Split(line, "silence_end:")
				if len(parts) >= 2 {
					valStr := strings.TrimSpace(strings.Split(parts[1], "|")[0])
					if v, err := strconv.ParseFloat(valStr, 64); err == nil {
						beats = append(beats, math.Round(v*1000)/1000)
					}
				}
			}
		}

		// If no onsets detected, generate evenly spaced beats
		if len(beats) == 0 && duration > 0 {
			// Assume 120 BPM as default
			bpm = 120
			interval := 60.0 / bpm
			for t := interval; t < duration; t += interval {
				beats = append(beats, math.Round(t*1000)/1000)
			}
		}
	}

	// Limit output size: for long songs, sample every Nth beat
	maxBeats := 200
	sampledBeats := beats
	if len(beats) > maxBeats {
		step := len(beats) / maxBeats
		sampledBeats = nil
		for i := 0; i < len(beats); i += step {
			sampledBeats = append(sampledBeats, beats[i])
		}
	}

	result := map[string]interface{}{
		"action":      "detect_beats",
		"status":      "success",
		"total_beats": len(beats),
		"beats":       sampledBeats,
	}
	if bpm > 0 {
		result["bpm"] = math.Round(bpm*10) / 10
		result["beat_interval_sec"] = math.Round(60.0/bpm*1000) / 1000
	}
	if len(beats) > 0 {
		result["first_beat"] = beats[0]
		result["last_beat"] = beats[len(beats)-1]
	}

	return toJSON(result), nil
}

// getEnergyCurve returns per-interval energy levels
func (t *AudioTool) getEnergyCurve(ctx context.Context, args audioArgs) (string, error) {
	tmpDir, err := os.MkdirTemp("", "audio-energy-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmpDir)

	audioPath, err := t.resolveAudioPath(args, tmpDir)
	if err != nil {
		return "", err
	}

	// Get duration
	probeCmd := exec.CommandContext(ctx, "ffprobe",
		"-v", "quiet", "-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1", audioPath,
	)
	durOut, err := probeCmd.Output()
	if err != nil {
		return "", fmt.Errorf("ffprobe failed: %v", err)
	}
	duration, _ := strconv.ParseFloat(strings.TrimSpace(string(durOut)), 64)

	interval := 1.0
	if args.Interval != "" {
		if v, err := strconv.ParseFloat(args.Interval, 64); err == nil && v >= 0.5 {
			interval = v
		}
	}

	curve, err := t.computeEnergyCurve(ctx, audioPath, duration, interval)
	if err != nil {
		return "", err
	}

	return toJSON(map[string]interface{}{
		"action":       "get_energy_curve",
		"status":       "success",
		"duration":     math.Round(duration*100) / 100,
		"interval_sec": interval,
		"total_points": len(curve),
		"energy_curve": curve,
		"summary":      summarizeEnergy(curve, duration),
	}), nil
}

// computeEnergyCurve uses ffmpeg astats to measure RMS energy per interval
func (t *AudioTool) computeEnergyCurve(ctx context.Context, audioPath string, duration, interval float64) ([]float64, error) {
	if duration <= 0 {
		return nil, fmt.Errorf("invalid duration")
	}

	var curve []float64
	for start := 0.0; start < duration; start += interval {
		segDur := interval
		if start+segDur > duration {
			segDur = duration - start
		}
		if segDur < 0.1 {
			break
		}

		cmd := exec.CommandContext(ctx, "ffmpeg",
			"-ss", fmt.Sprintf("%.3f", start),
			"-t", fmt.Sprintf("%.3f", segDur),
			"-i", audioPath,
			"-af", "astats=metadata=1:reset=1,ametadata=print:key=lavfi.astats.Overall.RMS_level",
			"-f", "null", "-",
		)
		out, _ := cmd.CombinedOutput()

		// Parse RMS level from output
		rms := -60.0 // default silence
		for _, line := range strings.Split(string(out), "\n") {
			if strings.Contains(line, "lavfi.astats.Overall.RMS_level=") {
				parts := strings.Split(line, "=")
				if len(parts) >= 2 {
					if v, err := strconv.ParseFloat(strings.TrimSpace(parts[len(parts)-1]), 64); err == nil {
						if v > rms {
							rms = v
						}
					}
				}
			}
		}

		// Normalize: -60dB = 0.0, 0dB = 1.0
		normalized := (rms + 60.0) / 60.0
		if normalized < 0 {
			normalized = 0
		}
		if normalized > 1 {
			normalized = 1
		}
		curve = append(curve, math.Round(normalized*100)/100)
	}

	return curve, nil
}

// summarizeEnergy provides a text summary of the energy curve for LLM
func summarizeEnergy(curve []float64, duration float64) string {
	if len(curve) == 0 {
		return ""
	}

	// Find min, max, avg
	min, max, sum := 1.0, 0.0, 0.0
	for _, v := range curve {
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
		sum += v
	}
	avg := sum / float64(len(curve))

	// Find energy peaks and valleys
	threshold := avg + (max-avg)*0.5
	var peaks []string
	for i, v := range curve {
		if v >= threshold {
			timeSec := float64(i) * (duration / float64(len(curve)))
			peaks = append(peaks, fmt.Sprintf("%.0fs", timeSec))
			// Skip next few to avoid duplicate peaks
			for i+1 < len(curve) && curve[i+1] >= threshold {
				i++
			}
		}
	}

	peakStr := "无明显高峰"
	if len(peaks) > 0 {
		if len(peaks) > 8 {
			peaks = peaks[:8]
		}
		peakStr = strings.Join(peaks, ", ")
	}

	return fmt.Sprintf(
		"能量范围: %.0f%%-%.0f%%, 平均: %.0f%%, 高能量时段: %s",
		min*100, max*100, avg*100, peakStr)
}

// generateSRT creates an SRT subtitle file from lyrics + timing info
// isCreditLine checks if a line is a song credit/metadata (not singing)
func isCreditLine(line string) bool {
	creditKeywords := []string{"词曲", "演唱", "制作人", "作词", "作曲", "编曲", "混音", "录音", "母带", "出品", "监制", "Lyrics", "Composer", "Producer", "Singer", "Vocal"}
	for _, kw := range creditKeywords {
		if strings.Contains(line, kw) {
			return true
		}
	}
	return false
}

func (t *AudioTool) generateSRT(ctx context.Context, args audioArgs) (string, error) {
	if args.Lyrics == "" {
		return "", fmt.Errorf("lyrics text is required")
	}
	if args.Duration == "" {
		return "", fmt.Errorf("duration (in seconds) is required")
	}

	duration, err := strconv.ParseFloat(args.Duration, 64)
	if err != nil {
		return "", fmt.Errorf("invalid duration: %v", err)
	}

	// Parse vocal_start if provided
	vocalStart := 0.0
	if args.VocalStart != "" {
		if vs, err := strconv.ParseFloat(args.VocalStart, 64); err == nil {
			vocalStart = vs
		}
	}

	// Parse lyrics into lines, stripping section markers
	// Separate credit lines (词曲/演唱/制作人) from singing lyrics
	var credits []string
	var singLines []string
	for _, line := range strings.Split(args.Lyrics, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Detect section markers
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			continue
		}
		// Strip inline section markers
		if idx := strings.Index(line, "]"); idx != -1 && strings.HasPrefix(line, "[") {
			line = strings.TrimSpace(line[idx+1:])
			if line == "" {
				continue
			}
		}
		if isCreditLine(line) {
			credits = append(credits, line)
		} else {
			singLines = append(singLines, line)
		}
	}

	if len(singLines) == 0 {
		return "", fmt.Errorf("no lyrics lines found after parsing")
	}

	// Parse sections if provided
	type section struct {
		Type  string  `json:"type"`
		Start float64 `json:"start"`
		End   float64 `json:"end"`
	}
	var sections []section
	if args.Sections != "" {
		json.Unmarshal([]byte(args.Sections), &sections)
	}

	// Auto-detect vocal_start if not provided
	if vocalStart <= 0 {
		// Default: estimate intro based on song duration
		if duration > 180 {
			vocalStart = 12.0 // typical pop song intro ~12s
		} else if duration > 60 {
			vocalStart = 5.0
		} else {
			vocalStart = 2.0
		}
	}

	// Build SRT
	var srt strings.Builder
	seq := 1

	// Place credit lines in the intro period (0 to vocalStart)
	if len(credits) > 0 {
		creditInterval := vocalStart / float64(len(credits))
		if creditInterval < 2.0 {
			creditInterval = 2.0
		}
		for i, credit := range credits {
			start := float64(i) * creditInterval
			end := start + creditInterval - 0.1
			if end > vocalStart {
				end = vocalStart - 0.1
			}
			if start >= vocalStart {
				break
			}
			srt.WriteString(fmt.Sprintf("%d\n", seq))
			srt.WriteString(fmt.Sprintf("%s --> %s\n", formatSRTTime(start), formatSRTTime(end)))
			srt.WriteString(credit + "\n\n")
			seq++
		}
	}

	// Distribute singing lyrics from vocalStart to (duration - outro)
	outro := 3.0
	singDuration := duration - vocalStart - outro
	if singDuration < float64(len(singLines))*1.5 {
		singDuration = duration - vocalStart - 0.5
		outro = 0.5
	}

	lineInterval := singDuration / float64(len(singLines))
	if lineInterval < 1.5 {
		lineInterval = 1.5
	}

	for i, line := range singLines {
		start := vocalStart + float64(i)*lineInterval
		end := start + lineInterval - 0.1

		if end > duration {
			end = duration
		}
		if start >= duration {
			break
		}

		srt.WriteString(fmt.Sprintf("%d\n", seq))
		srt.WriteString(fmt.Sprintf("%s --> %s\n", formatSRTTime(start), formatSRTTime(end)))
		srt.WriteString(line + "\n\n")
		seq++
	}

	srtContent := srt.String()
	totalLines := len(credits) + len(singLines)

	return toJSON(map[string]interface{}{
		"action":       "generate_srt",
		"status":       "success",
		"credit_lines": len(credits),
		"sing_lines":   len(singLines),
		"total_lines":  totalLines,
		"vocal_start":  vocalStart,
		"duration":     duration,
		"srt":          srtContent,
		"message":      fmt.Sprintf("已生成 %d 行字幕（%d行署名+%d行歌词），人声起始 %.1fs，总时长 %s。可直接用于 mv_production 的 lyrics_srt 参数。", totalLines, len(credits), len(singLines), vocalStart, fmt.Sprintf("%d:%02d", int(duration)/60, int(duration)%60)),
	}), nil
}

// formatSRTTime formats seconds to SRT timestamp HH:MM:SS,mmm
func formatSRTTime(sec float64) string {
	h := int(sec) / 3600
	m := (int(sec) % 3600) / 60
	s := int(sec) % 60
	ms := int((sec - float64(int(sec))) * 1000)
	return fmt.Sprintf("%02d:%02d:%02d,%03d", h, m, s, ms)
}
