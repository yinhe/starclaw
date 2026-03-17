package starclaw

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// ChatStream is an iterator over streaming chat completion chunks.
type ChatStream struct {
	reader *bufio.Reader
	body   io.ReadCloser
	done   bool
}

// Next returns the next chunk. Returns io.EOF when the stream is done.
func (s *ChatStream) Next() (*ChatCompletionChunk, error) {
	if s.done {
		return nil, io.EOF
	}

	for {
		line, err := s.reader.ReadString('\n')
		if err != nil {
			s.done = true
			if err == io.EOF {
				return nil, io.EOF
			}
			return nil, fmt.Errorf("read stream: %w", err)
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			s.done = true
			return nil, io.EOF
		}

		var chunk ChatCompletionChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue // skip malformed chunks
		}
		return &chunk, nil
	}
}

// Close closes the underlying response body.
func (s *ChatStream) Close() error {
	s.done = true
	if s.body != nil {
		return s.body.Close()
	}
	return nil
}

// ChatStream creates a streaming chat completion.
// The caller must call Close() on the returned ChatStream when done.
//
// Usage:
//
//	stream, err := client.ChatStream(ctx, req)
//	if err != nil { ... }
//	defer stream.Close()
//
//	for {
//	    chunk, err := stream.Next()
//	    if err == io.EOF { break }
//	    if err != nil { ... }
//	    fmt.Print(chunk.Choices[0].Delta.Content)
//	}
func (c *Client) ChatStream(ctx context.Context, req ChatCompletionRequest) (*ChatStream, error) {
	req.Stream = true

	// Use a client with no timeout for streaming
	httpClient := &http.Client{}

	bodyData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.endpoint+"/v1/chat/completions", strings.NewReader(string(bodyData)))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	return &ChatStream{
		reader: bufio.NewReader(resp.Body),
		body:   resp.Body,
	}, nil
}
