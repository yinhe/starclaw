// Basic chat example using the StarClaw API (OpenAI-compatible).
//
// Usage:
//   export STARCLAW_ENDPOINT=http://localhost:8080
//   export STARCLAW_API_KEY=your-jwt-token
//   go run main.go "What is StarClaw?"
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

func main() {
	endpoint := os.Getenv("STARCLAW_ENDPOINT")
	if endpoint == "" {
		endpoint = "http://localhost:8080"
	}
	apiKey := os.Getenv("STARCLAW_API_KEY")

	prompt := "Hello, what can you do?"
	if len(os.Args) > 1 {
		prompt = os.Args[1]
	}

	body, _ := json.Marshal(map[string]interface{}{
		"model": "deepseek-chat",
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
	})

	req, _ := http.NewRequest("POST", endpoint+"/v1/chat/completions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		fmt.Fprintf(os.Stderr, "HTTP %d: %s\n", resp.StatusCode, data)
		os.Exit(1)
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	json.Unmarshal(data, &result)

	if len(result.Choices) > 0 {
		fmt.Println(result.Choices[0].Message.Content)
	}
}
