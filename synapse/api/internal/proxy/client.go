package proxy

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client forwards requests to the Node.js overseas relay proxy
type Client struct {
	baseURL   string
	secretKey string
	http      *http.Client
}

func NewClient(baseURL, secretKey string) *Client {
	return &Client{
		baseURL:   baseURL,
		secretKey: secretKey,
		http: &http.Client{
			Timeout: 5 * time.Minute, // long timeout for video generation etc.
		},
	}
}

// Forward sends a request to the proxy and returns the raw response.
// The caller is responsible for closing resp.Body.
func (c *Client) Forward(method, path string, body io.Reader, headers http.Header) (*http.Response, error) {
	url := fmt.Sprintf("%s%s", c.baseURL, path)

	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, fmt.Errorf("proxy: failed to create request: %w", err)
	}

	// Copy relevant headers
	for k, vals := range headers {
		for _, v := range vals {
			req.Header.Add(k, v)
		}
	}

	// Internal auth
	req.Header.Set("X-API-KEY", c.secretKey)
	// Ensure content type is set
	if req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("proxy: request failed: %w", err)
	}

	return resp, nil
}

// ForwardStream sends a request and streams the response back via the provided writer.
func (c *Client) ForwardStream(w http.ResponseWriter, method, path string, body io.Reader, headers http.Header) error {
	_, err := c.ForwardStreamCapture(w, method, path, body, headers)
	return err
}

// ForwardStreamCapture streams the response and captures the full response body for usage extraction.
// Returns the accumulated response body bytes (for non-stream) or the last SSE data line (for stream).
func (c *Client) ForwardStreamCapture(w http.ResponseWriter, method, path string, body io.Reader, headers http.Header) ([]byte, error) {
	resp, err := c.Forward(method, path, body, headers)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Copy response headers
	for k, vals := range resp.Header {
		for _, v := range vals {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)

	isSSE := strings.Contains(resp.Header.Get("Content-Type"), "text/event-stream")

	if f, ok := w.(http.Flusher); ok {
		buf := make([]byte, 4096)
		var lastDataLine string
		var partial string
		var fullBody []byte

		for {
			n, readErr := resp.Body.Read(buf)
			if n > 0 {
				w.Write(buf[:n])
				f.Flush()

				if isSSE {
					// Track last SSE data line for usage extraction
					partial += string(buf[:n])
					for {
						idx := strings.Index(partial, "\n")
						if idx < 0 {
							break
						}
						line := strings.TrimSpace(partial[:idx])
						partial = partial[idx+1:]
						if strings.HasPrefix(line, "data: ") && line != "data: [DONE]" {
							lastDataLine = line[6:]
						}
					}
				} else {
					fullBody = append(fullBody, buf[:n]...)
				}
			}
			if readErr != nil {
				break
			}
		}

		if isSSE {
			return []byte(lastDataLine), nil
		}
		return fullBody, nil
	}

	// Non-flusher fallback
	data, _ := io.ReadAll(resp.Body)
	w.Write(data)
	return data, nil
}
