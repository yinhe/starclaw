package main

import (
	"fmt"
	"log"
	"time"

	"net/http"

	"github.com/yinhe/starclaw/internal/node"
	"github.com/yinhe/starclaw/internal/provider"
	"github.com/yinhe/starclaw/internal/tool"
)

func main() {
	// Load local Identity (same as the running Claw API)
	identity := node.LoadOrCreateIdentity()
	log.Printf("Identity: %s", identity.NodeID)

	// Init StarAI proxy with the identity
	client := &http.Client{
		Transport: &provider.SignedTransport{Identity: identity},
	}
	_ = client

	baseURL := "https://api.star-ai.net/v1"
	tool.InitStarAIProxy(identity, baseURL, nil)

	// Test: submit a simple image generation to fal.ai via StarAI proxy
	apiKey := "starai://fal"
	endpoint := "fal-ai/flux/schnell"
	payload := map[string]interface{}{
		"prompt":     "a cute red cat sitting on a windowsill, watercolor style",
		"image_size": "square",
	}

	log.Println("Submitting image generation via StarAI proxy...")
	requestID, statusEndpoint, err := tool.SubmitToFal(apiKey, endpoint, payload)
	if err != nil {
		log.Fatalf("Submit failed: %v", err)
	}
	log.Printf("Submitted! request_id=%s statusEndpoint=%s", requestID, statusEndpoint)

	log.Println("Polling for result (timeout 2min)...")
	result, err := tool.PollFalStatus(apiKey, statusEndpoint, requestID, 2*time.Minute)
	if err != nil {
		log.Fatalf("Poll failed: %v", err)
	}

	// Extract image URL
	if images, ok := result["images"].([]interface{}); ok && len(images) > 0 {
		if img, ok := images[0].(map[string]interface{}); ok {
			fmt.Printf("\n✅ Image generated successfully!\n")
			fmt.Printf("   URL: %s\n", img["url"])
			fmt.Printf("   Size: %v x %v\n", img["width"], img["height"])
		}
	} else {
		fmt.Printf("\n⚠️ Result: %v\n", result)
	}
}
