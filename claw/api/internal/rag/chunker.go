package rag

import (
	"strings"
	"unicode/utf8"
)

// ChunkText splits text into overlapping chunks of approximately chunkSize characters
func ChunkText(text string, chunkSize, overlap int) []string {
	if chunkSize <= 0 {
		chunkSize = 500
	}
	if overlap < 0 {
		overlap = 0
	}
	if overlap >= chunkSize {
		overlap = chunkSize / 5
	}

	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}

	runes := []rune(text)
	totalLen := len(runes)

	if totalLen <= chunkSize {
		return []string{text}
	}

	var chunks []string
	start := 0

	for start < totalLen {
		end := start + chunkSize
		if end > totalLen {
			end = totalLen
		}

		// Try to break at sentence/paragraph boundary
		chunk := string(runes[start:end])
		if end < totalLen {
			// Look for a good break point near the end
			breakIdx := findBreakPoint(chunk)
			if breakIdx > chunkSize/2 {
				end = start + breakIdx + 1
				chunk = string(runes[start:end])
			}
		}

		chunk = strings.TrimSpace(chunk)
		if chunk != "" {
			chunks = append(chunks, chunk)
		}

		start = end - overlap
		if start <= 0 && len(chunks) > 0 {
			break
		}
	}

	return chunks
}

// findBreakPoint finds the best position to break text (sentence/paragraph boundary)
func findBreakPoint(text string) int {
	runes := []rune(text)
	best := -1

	// Priority: paragraph > sentence > clause > word
	for i := len(runes) - 1; i >= len(runes)/2; i-- {
		ch := runes[i]
		switch ch {
		case '\n':
			return i
		case '.', '。', '！', '!', '？', '?':
			if best == -1 {
				best = i
			}
		case '，', ',', '；', ';', '：', ':':
			if best == -1 {
				best = i
			}
		case ' ':
			if best == -1 {
				best = i
			}
		}
	}

	return best
}

// EstimateTokens gives a rough token count (1 token ≀4 chars for English, ≀1.5 chars for Chinese)
func EstimateTokens(text string) int {
	asciiCount := 0
	totalCount := utf8.RuneCountInString(text)
	for _, r := range text {
		if r < 128 {
			asciiCount++
		}
	}
	nonASCII := totalCount - asciiCount
	return asciiCount/4 + nonASCII*2/3 + 1
}
