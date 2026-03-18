package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"
)

// ════════════════════════════════════════════════════════════════
//  Modality Types
// ════════════════════════════════════════════════════════════════

// Modality represents a type of input or output.
type Modality string

const (
	ModalityText  Modality = "text"
	ModalityImage Modality = "image"
	ModalityAudio Modality = "audio"
	ModalityVideo Modality = "video"
	ModalityFile  Modality = "file"
)

// MultimodalInput represents a single input item with its modality.
type MultimodalInput struct {
	Modality Modality               `json:"modality"`
	Content  string                 `json:"content"`           // text content or base64/URL for media
	MimeType string                 `json:"mime_type,omitempty"` // e.g. image/png, audio/wav
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// MultimodalOutput represents a generated output with its modality.
type MultimodalOutput struct {
	Modality Modality               `json:"modality"`
	Content  string                 `json:"content"`
	MimeType string                 `json:"mime_type,omitempty"`
	URL      string                 `json:"url,omitempty"`      // for generated media
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// MultimodalRequest is a request that can contain multiple modalities.
type MultimodalRequest struct {
	AgentID        string            `json:"agent_id"`
	ConversationID string            `json:"conversation_id"`
	Inputs         []MultimodalInput `json:"inputs"`
	OutputModality []Modality        `json:"output_modality,omitempty"` // desired output types
	Model          string            `json:"model,omitempty"`
	Stream         bool              `json:"stream"`
}

// MultimodalResponse contains the agent's multimodal outputs.
type MultimodalResponse struct {
	Outputs       []MultimodalOutput `json:"outputs"`
	InputAnalysis []InputAnalysis    `json:"input_analysis,omitempty"` // what the agent understood from each input
	Usage         TokenUsage         `json:"usage"`
	DurationMs    int64              `json:"duration_ms"`
}

// InputAnalysis is the agent's understanding of a non-text input.
type InputAnalysis struct {
	Modality    Modality `json:"modality"`
	Description string   `json:"description"` // natural language description
	Labels      []string `json:"labels,omitempty"`
	Confidence  float64  `json:"confidence"`
}

// TokenUsage tracks token consumption.
type TokenUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// ════════════════════════════════════════════════════════════════
//  Modality Processor Interface
// ════════════════════════════════════════════════════════════════

// ModalityProcessor handles a specific modality's input/output.
type ModalityProcessor interface {
	// Modality returns which modality this processor handles.
	Modality() Modality

	// ProcessInput converts a modality-specific input into text for LLM context.
	ProcessInput(ctx context.Context, input MultimodalInput) (string, *InputAnalysis, error)

	// GenerateOutput produces modality-specific output from text.
	GenerateOutput(ctx context.Context, textPrompt string, params map[string]interface{}) (*MultimodalOutput, error)

	// SupportedMimeTypes returns the MIME types this processor can handle.
	SupportedMimeTypes() []string
}

// ════════════════════════════════════════════════════════════════
//  Built-in Processors
// ════════════════════════════════════════════════════════════════

// TextProcessor handles plain text modality.
type TextProcessor struct{}

func (p *TextProcessor) Modality() Modality           { return ModalityText }
func (p *TextProcessor) SupportedMimeTypes() []string  { return []string{"text/plain"} }

func (p *TextProcessor) ProcessInput(_ context.Context, input MultimodalInput) (string, *InputAnalysis, error) {
	return input.Content, nil, nil // text passes through directly
}

func (p *TextProcessor) GenerateOutput(_ context.Context, textPrompt string, _ map[string]interface{}) (*MultimodalOutput, error) {
	return &MultimodalOutput{Modality: ModalityText, Content: textPrompt, MimeType: "text/plain"}, nil
}

// ImageProcessor handles image understanding and generation.
type ImageProcessor struct {
	// VisionModel is the model endpoint for image understanding (e.g. gpt-4o, claude-3)
	VisionModel string
	// GenModel is the model endpoint for image generation (e.g. dall-e-3, stable-diffusion)
	GenModel string
}

func (p *ImageProcessor) Modality() Modality { return ModalityImage }
func (p *ImageProcessor) SupportedMimeTypes() []string {
	return []string{"image/png", "image/jpeg", "image/gif", "image/webp", "image/svg+xml"}
}

func (p *ImageProcessor) ProcessInput(_ context.Context, input MultimodalInput) (string, *InputAnalysis, error) {
	// Build a vision prompt for the LLM
	description := fmt.Sprintf("[Image input: %s]", input.MimeType)
	if input.Content != "" {
		// Content is either a URL or base64
		if strings.HasPrefix(input.Content, "http") {
			description = fmt.Sprintf("[Image from URL: %s — describe this image in detail]", input.Content)
		} else {
			description = "[Image (base64 encoded) — describe this image in detail]"
		}
	}

	analysis := &InputAnalysis{
		Modality:    ModalityImage,
		Description: description,
		Confidence:  0.9,
	}

	return description, analysis, nil
}

func (p *ImageProcessor) GenerateOutput(_ context.Context, textPrompt string, params map[string]interface{}) (*MultimodalOutput, error) {
	return &MultimodalOutput{
		Modality: ModalityImage,
		Content:  textPrompt,
		MimeType: "image/png",
		Metadata: map[string]interface{}{"generator": p.GenModel, "prompt": textPrompt},
	}, nil
}

// AudioProcessor handles audio understanding (STT) and generation (TTS).
type AudioProcessor struct {
	STTModel string
	TTSModel string
}

func (p *AudioProcessor) Modality() Modality { return ModalityAudio }
func (p *AudioProcessor) SupportedMimeTypes() []string {
	return []string{"audio/wav", "audio/mp3", "audio/mpeg", "audio/ogg", "audio/flac", "audio/webm"}
}

func (p *AudioProcessor) ProcessInput(_ context.Context, input MultimodalInput) (string, *InputAnalysis, error) {
	description := fmt.Sprintf("[Audio input: %s — transcribe and analyze]", input.MimeType)
	analysis := &InputAnalysis{
		Modality:    ModalityAudio,
		Description: "Audio content awaiting transcription",
		Confidence:  0.85,
	}
	return description, analysis, nil
}

func (p *AudioProcessor) GenerateOutput(_ context.Context, textPrompt string, _ map[string]interface{}) (*MultimodalOutput, error) {
	return &MultimodalOutput{
		Modality: ModalityAudio,
		Content:  textPrompt,
		MimeType: "audio/mp3",
		Metadata: map[string]interface{}{"tts_input": textPrompt},
	}, nil
}

// VideoProcessor handles video understanding.
type VideoProcessor struct {
	VisionModel string
}

func (p *VideoProcessor) Modality() Modality { return ModalityVideo }
func (p *VideoProcessor) SupportedMimeTypes() []string {
	return []string{"video/mp4", "video/webm", "video/mpeg"}
}

func (p *VideoProcessor) ProcessInput(_ context.Context, input MultimodalInput) (string, *InputAnalysis, error) {
	description := fmt.Sprintf("[Video input: %s — extract key frames and describe content]", input.MimeType)
	analysis := &InputAnalysis{
		Modality:    ModalityVideo,
		Description: "Video content awaiting frame extraction and analysis",
		Confidence:  0.8,
	}
	return description, analysis, nil
}

func (p *VideoProcessor) GenerateOutput(_ context.Context, textPrompt string, _ map[string]interface{}) (*MultimodalOutput, error) {
	return &MultimodalOutput{
		Modality: ModalityVideo,
		Content:  textPrompt,
		MimeType: "video/mp4",
		Metadata: map[string]interface{}{"video_prompt": textPrompt},
	}, nil
}

// ════════════════════════════════════════════════════════════════
//  Multimodal Router
// ════════════════════════════════════════════════════════════════

// MultimodalRouter routes inputs to appropriate processors and assembles outputs.
type MultimodalRouter struct {
	mu         sync.RWMutex
	processors map[Modality]ModalityProcessor
}

// NewMultimodalRouter creates a router with default processors.
func NewMultimodalRouter() *MultimodalRouter {
	r := &MultimodalRouter{
		processors: make(map[Modality]ModalityProcessor),
	}

	// Register default processors
	r.Register(&TextProcessor{})
	r.Register(&ImageProcessor{VisionModel: "gpt-4o", GenModel: "dall-e-3"})
	r.Register(&AudioProcessor{STTModel: "whisper-1", TTSModel: "tts-1"})
	r.Register(&VideoProcessor{VisionModel: "gpt-4o"})

	return r
}

// Register adds or replaces a modality processor.
func (r *MultimodalRouter) Register(p ModalityProcessor) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.processors[p.Modality()] = p
	log.Printf("[Multimodal] Registered processor for %s modality", p.Modality())
}

// GetProcessor returns the processor for a given modality.
func (r *MultimodalRouter) GetProcessor(m Modality) (ModalityProcessor, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.processors[m]
	return p, ok
}

// ProcessInputs converts all multimodal inputs into a unified text context for the LLM.
func (r *MultimodalRouter) ProcessInputs(ctx context.Context, inputs []MultimodalInput) (string, []InputAnalysis, error) {
	var parts []string
	var analyses []InputAnalysis

	for _, input := range inputs {
		proc, ok := r.GetProcessor(input.Modality)
		if !ok {
			return "", nil, fmt.Errorf("unsupported modality: %s", input.Modality)
		}

		text, analysis, err := proc.ProcessInput(ctx, input)
		if err != nil {
			return "", nil, fmt.Errorf("processing %s input: %w", input.Modality, err)
		}

		parts = append(parts, text)
		if analysis != nil {
			analyses = append(analyses, *analysis)
		}
	}

	return strings.Join(parts, "\n\n"), analyses, nil
}

// GenerateOutputs produces outputs for the requested modalities.
func (r *MultimodalRouter) GenerateOutputs(ctx context.Context, textResponse string, modalities []Modality) ([]MultimodalOutput, error) {
	if len(modalities) == 0 {
		modalities = []Modality{ModalityText}
	}

	var outputs []MultimodalOutput

	for _, m := range modalities {
		proc, ok := r.GetProcessor(m)
		if !ok {
			continue
		}

		output, err := proc.GenerateOutput(ctx, textResponse, nil)
		if err != nil {
			log.Printf("[Multimodal] Error generating %s output: %v", m, err)
			continue
		}
		outputs = append(outputs, *output)
	}

	return outputs, nil
}

// DetectModality determines the modality from MIME type.
func DetectModality(mimeType string) Modality {
	switch {
	case strings.HasPrefix(mimeType, "image/"):
		return ModalityImage
	case strings.HasPrefix(mimeType, "audio/"):
		return ModalityAudio
	case strings.HasPrefix(mimeType, "video/"):
		return ModalityVideo
	case strings.HasPrefix(mimeType, "text/"):
		return ModalityText
	default:
		return ModalityFile
	}
}

// SupportedModalities returns all registered modalities.
func (r *MultimodalRouter) SupportedModalities() []map[string]interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []map[string]interface{}
	for m, p := range r.processors {
		result = append(result, map[string]interface{}{
			"modality":   m,
			"mime_types": p.SupportedMimeTypes(),
		})
	}
	return result
}

// ════════════════════════════════════════════════════════════════
//  Multimodal Conversation Message
// ════════════════════════════════════════════════════════════════

// MultimodalMessage represents a conversation message with multiple modalities.
type MultimodalMessage struct {
	Role    string            `json:"role"` // user, assistant, system
	Parts   []MultimodalInput `json:"parts"`
	Created time.Time         `json:"created"`
}

// ToLLMContent converts multimodal parts to LLM-compatible content format.
func (m *MultimodalMessage) ToLLMContent() []map[string]interface{} {
	var content []map[string]interface{}

	for _, part := range m.Parts {
		switch part.Modality {
		case ModalityText:
			content = append(content, map[string]interface{}{
				"type": "text",
				"text": part.Content,
			})
		case ModalityImage:
			if strings.HasPrefix(part.Content, "http") {
				content = append(content, map[string]interface{}{
					"type": "image_url",
					"image_url": map[string]string{
						"url": part.Content,
					},
				})
			} else {
				content = append(content, map[string]interface{}{
					"type": "image_url",
					"image_url": map[string]string{
						"url": "data:" + part.MimeType + ";base64," + part.Content,
					},
				})
			}
		default:
			// Encode non-standard modalities as text description
			content = append(content, map[string]interface{}{
				"type": "text",
				"text": fmt.Sprintf("[%s content: %s]", part.Modality, part.MimeType),
			})
		}
	}

	return content
}

// ToJSON serializes for storage.
func (m *MultimodalMessage) ToJSON() string {
	b, _ := json.Marshal(m)
	return string(b)
}
