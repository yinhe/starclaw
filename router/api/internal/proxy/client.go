package proxy

import (
	"fmt"
	"io"
	"net/http"
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
	resp, err := c.Forward(method, path, body, headers)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Copy response headers
	for k, vals := range resp.Header {
		for _, v := range vals {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)

	// Stream body
	if f, ok := w.(http.Flusher); ok {
		buf := make([]byte, 4096)
		for {
			n, err := resp.Body.Read(buf)
			if n > 0 {
				w.Write(buf[:n])
				f.Flush()
			}
			if err != nil {
				break
			}
		}
	} else {
		io.Copy(w, resp.Body)
	}

	return nil
}
