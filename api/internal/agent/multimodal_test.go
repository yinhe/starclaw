package agent

import (
	"context"
	"strings"
	"testing"
)

func TestMultimodalRouter_DefaultProcessors(t *testing.T) {
	r := NewMultimodalRouter()

	for _, m := range []Modality{ModalityText, ModalityImage, ModalityAudio, ModalityVideo} {
		if _, ok := r.GetProcessor(m); !ok {
			t.Errorf("expected default processor for %s", m)
		}
	}

	// File modality should NOT have a default processor
	if _, ok := r.GetProcessor(ModalityFile); ok {
		t.Error("file modality should not have a default processor")
	}
}

func TestMultimodalRouter_RegisterCustomProcessor(t *testing.T) {
	r := NewMultimodalRouter()

	custom := &TextProcessor{} // reuse text processor as a custom one
	r.Register(custom)

	p, ok := r.GetProcessor(ModalityText)
	if !ok {
		t.Fatal("expected text processor after re-register")
	}
	if p != custom {
		t.Error("expected custom processor to replace default")
	}
}

func TestMultimodalRouter_SupportedModalities(t *testing.T) {
	r := NewMultimodalRouter()
	modalities := r.SupportedModalities()

	if len(modalities) < 4 {
		t.Errorf("expected at least 4 modalities, got %d", len(modalities))
	}

	found := map[Modality]bool{}
	for _, m := range modalities {
		if mod, ok := m["modality"].(Modality); ok {
			found[mod] = true
		}
	}
	for _, m := range []Modality{ModalityText, ModalityImage, ModalityAudio, ModalityVideo} {
		if !found[m] {
			t.Errorf("expected %s in supported modalities", m)
		}
	}
}

func TestTextProcessor_ProcessInput(t *testing.T) {
	p := &TextProcessor{}
	ctx := context.Background()

	text, analysis, err := p.ProcessInput(ctx, MultimodalInput{
		Modality: ModalityText,
		Content:  "Hello world",
	})

	if err != nil {
		t.Fatalf("ProcessInput: %v", err)
	}
	if text != "Hello world" {
		t.Errorf("expected passthrough, got %q", text)
	}
	if analysis != nil {
		t.Error("text processor should return nil analysis")
	}
}

func TestTextProcessor_GenerateOutput(t *testing.T) {
	p := &TextProcessor{}
	ctx := context.Background()

	out, err := p.GenerateOutput(ctx, "response text", nil)
	if err != nil {
		t.Fatalf("GenerateOutput: %v", err)
	}
	if out.Modality != ModalityText {
		t.Errorf("expected text modality, got %s", out.Modality)
	}
	if out.Content != "response text" {
		t.Errorf("expected 'response text', got %q", out.Content)
	}
	if out.MimeType != "text/plain" {
		t.Errorf("expected text/plain, got %s", out.MimeType)
	}
}

func TestImageProcessor_ProcessInput_URL(t *testing.T) {
	p := &ImageProcessor{VisionModel: "gpt-4o", GenModel: "dall-e-3"}
	ctx := context.Background()

	text, analysis, err := p.ProcessInput(ctx, MultimodalInput{
		Modality: ModalityImage,
		Content:  "https://example.com/image.png",
		MimeType: "image/png",
	})

	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(text, "https://example.com/image.png") {
		t.Errorf("expected URL in text description, got %q", text)
	}
	if analysis == nil {
		t.Fatal("expected analysis for image input")
	}
	if analysis.Modality != ModalityImage {
		t.Errorf("expected image modality in analysis, got %s", analysis.Modality)
	}
	if analysis.Confidence <= 0 {
		t.Error("expected positive confidence")
	}
}

func TestImageProcessor_ProcessInput_Base64(t *testing.T) {
	p := &ImageProcessor{VisionModel: "gpt-4o", GenModel: "dall-e-3"}
	ctx := context.Background()

	text, analysis, err := p.ProcessInput(ctx, MultimodalInput{
		Modality: ModalityImage,
		Content:  "iVBORw0KGgoAAAANS", // base64 prefix
		MimeType: "image/png",
	})

	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(text, "base64") {
		t.Errorf("expected base64 mention in text, got %q", text)
	}
	if analysis == nil {
		t.Fatal("expected analysis")
	}
}

func TestAudioProcessor_ProcessAndGenerate(t *testing.T) {
	p := &AudioProcessor{STTModel: "whisper-1", TTSModel: "tts-1"}
	ctx := context.Background()

	text, analysis, err := p.ProcessInput(ctx, MultimodalInput{
		Modality: ModalityAudio,
		Content:  "base64-audio-data",
		MimeType: "audio/wav",
	})

	if err != nil {
		t.Fatal(err)
	}
	if text == "" {
		t.Error("expected non-empty text description")
	}
	if analysis == nil || analysis.Modality != ModalityAudio {
		t.Error("expected audio analysis")
	}

	out, err := p.GenerateOutput(ctx, "speech text", nil)
	if err != nil {
		t.Fatal(err)
	}
	if out.Modality != ModalityAudio {
		t.Errorf("expected audio, got %s", out.Modality)
	}
	if out.MimeType != "audio/mp3" {
		t.Errorf("expected audio/mp3, got %s", out.MimeType)
	}
}

func TestVideoProcessor_ProcessAndGenerate(t *testing.T) {
	p := &VideoProcessor{VisionModel: "gpt-4o"}
	ctx := context.Background()

	text, analysis, err := p.ProcessInput(ctx, MultimodalInput{
		Modality: ModalityVideo,
		Content:  "base64-video",
		MimeType: "video/mp4",
	})

	if err != nil {
		t.Fatal(err)
	}
	if text == "" {
		t.Error("expected non-empty description")
	}
	if analysis == nil || analysis.Modality != ModalityVideo {
		t.Error("expected video analysis")
	}

	out, err := p.GenerateOutput(ctx, "video prompt", nil)
	if err != nil {
		t.Fatal(err)
	}
	if out.Modality != ModalityVideo {
		t.Errorf("expected video, got %s", out.Modality)
	}
}

func TestMultimodalRouter_ProcessInputs(t *testing.T) {
	r := NewMultimodalRouter()
	ctx := context.Background()

	inputs := []MultimodalInput{
		{Modality: ModalityText, Content: "describe this image"},
		{Modality: ModalityImage, Content: "https://example.com/cat.jpg", MimeType: "image/jpeg"},
	}

	text, analyses, err := r.ProcessInputs(ctx, inputs)
	if err != nil {
		t.Fatalf("ProcessInputs: %v", err)
	}

	if !strings.Contains(text, "describe this image") {
		t.Error("expected text content in combined output")
	}
	if !strings.Contains(text, "https://example.com/cat.jpg") {
		t.Error("expected image URL in combined output")
	}

	// Only image should have analysis (text returns nil)
	if len(analyses) != 1 {
		t.Fatalf("expected 1 analysis, got %d", len(analyses))
	}
	if analyses[0].Modality != ModalityImage {
		t.Errorf("expected image analysis, got %s", analyses[0].Modality)
	}
}

func TestMultimodalRouter_ProcessInputs_UnsupportedModality(t *testing.T) {
	r := NewMultimodalRouter()
	ctx := context.Background()

	_, _, err := r.ProcessInputs(ctx, []MultimodalInput{
		{Modality: ModalityFile, Content: "data"},
	})

	if err == nil {
		t.Error("expected error for unsupported file modality")
	}
	if !strings.Contains(err.Error(), "unsupported modality") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestMultimodalRouter_GenerateOutputs(t *testing.T) {
	r := NewMultimodalRouter()
	ctx := context.Background()

	outputs, err := r.GenerateOutputs(ctx, "Hello!", []Modality{ModalityText, ModalityImage})
	if err != nil {
		t.Fatal(err)
	}

	if len(outputs) != 2 {
		t.Fatalf("expected 2 outputs, got %d", len(outputs))
	}
	if outputs[0].Modality != ModalityText {
		t.Errorf("first output should be text, got %s", outputs[0].Modality)
	}
	if outputs[1].Modality != ModalityImage {
		t.Errorf("second output should be image, got %s", outputs[1].Modality)
	}
}

func TestMultimodalRouter_GenerateOutputs_DefaultText(t *testing.T) {
	r := NewMultimodalRouter()
	ctx := context.Background()

	outputs, err := r.GenerateOutputs(ctx, "just text", nil) // nil modalities → default to text
	if err != nil {
		t.Fatal(err)
	}

	if len(outputs) != 1 {
		t.Fatalf("expected 1 output, got %d", len(outputs))
	}
	if outputs[0].Modality != ModalityText {
		t.Errorf("expected text, got %s", outputs[0].Modality)
	}
}

func TestDetectModality(t *testing.T) {
	cases := []struct {
		mime     string
		expected Modality
	}{
		{"image/png", ModalityImage},
		{"image/jpeg", ModalityImage},
		{"audio/wav", ModalityAudio},
		{"audio/mp3", ModalityAudio},
		{"video/mp4", ModalityVideo},
		{"text/plain", ModalityText},
		{"application/pdf", ModalityFile},
	}

	for _, tc := range cases {
		got := DetectModality(tc.mime)
		if got != tc.expected {
			t.Errorf("DetectModality(%q) = %s, want %s", tc.mime, got, tc.expected)
		}
	}
}

func TestMultimodalMessage_ToLLMContent(t *testing.T) {
	msg := MultimodalMessage{
		Role: "user",
		Parts: []MultimodalInput{
			{Modality: ModalityText, Content: "What is this?"},
			{Modality: ModalityImage, Content: "https://example.com/img.png", MimeType: "image/png"},
		},
	}

	llm := msg.ToLLMContent()
	if len(llm) != 2 {
		t.Fatalf("expected 2 content items, got %d", len(llm))
	}

	// First item should be text
	if llm[0]["type"] != "text" {
		t.Errorf("first item should be text, got %v", llm[0]["type"])
	}

	// Second item should be image_url
	if llm[1]["type"] != "image_url" {
		t.Errorf("second item should be image_url, got %v", llm[1]["type"])
	}
}

func TestImageProcessor_SupportedMimeTypes(t *testing.T) {
	p := &ImageProcessor{}
	mimes := p.SupportedMimeTypes()
	if len(mimes) == 0 {
		t.Error("expected non-empty mime types")
	}
	found := false
	for _, m := range mimes {
		if m == "image/png" {
			found = true
		}
	}
	if !found {
		t.Error("expected image/png in supported types")
	}
}
