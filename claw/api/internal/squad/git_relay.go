package squad

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/yinhe/starclaw/internal/node"
)

// GitRelayProxy proxies Git HTTP requests through the Nydus relay network.
// This enables Git collaboration between nodes behind NAT without direct connectivity.
//
// Flow for clone/push through relay:
//   1. Remote node calls GitRelayProxy.ProxyRequest(targetNodeID, gitHTTPRequest)
//   2. Proxy serializes the HTTP request as a RelayMessage
//   3. Relay forwards to the target node
//   4. Target node's GitHTTPHandler processes the request
//   5. Response flows back through the relay
type GitRelayProxy struct {
	nydus *node.NydusManager
	httpC *http.Client
}

// NewGitRelayProxy creates a new relay proxy for Git HTTP traffic.
func NewGitRelayProxy(nydus *node.NydusManager) *GitRelayProxy {
	return &GitRelayProxy{
		nydus: nydus,
		httpC: &http.Client{Timeout: 120 * time.Second},
	}
}

// GitProxyRequest wraps a Git HTTP request for relay transport.
type GitProxyRequest struct {
	Method      string            `json:"method"`
	Path        string            `json:"path"`       // e.g. "/v1/git/mission-xxx/info/refs?service=git-upload-pack"
	Headers     map[string]string `json:"headers"`
	Body        []byte            `json:"body,omitempty"`
	ContentType string            `json:"content_type"`
}

// GitProxyResponse wraps a Git HTTP response for relay transport.
type GitProxyResponse struct {
	StatusCode  int               `json:"status_code"`
	Headers     map[string]string `json:"headers"`
	Body        []byte            `json:"body"`
	ContentType string            `json:"content_type"`
}

// ProxyToNode sends a Git HTTP request to a remote node.
// If the node is directly reachable, uses direct HTTP.
// If only reachable via relay, uses the Nydus relay protocol.
func (p *GitRelayProxy) ProxyToNode(ctx context.Context, targetNodeID, targetAddress string, req *GitProxyRequest) (*GitProxyResponse, error) {
	// Strategy 1: Direct HTTP (if we have a resolved address)
	if targetAddress != "" {
		return p.directRequest(ctx, targetAddress, req)
	}

	// Strategy 2: Via Nydus relay
	if p.nydus != nil {
		conn := p.nydus.GetConnection(targetNodeID)
		if conn != nil && conn.RelayURL != "" {
			return p.relayRequest(ctx, targetNodeID, req)
		}
	}

	return nil, fmt.Errorf("no route to node %s", targetNodeID)
}

// directRequest sends a Git HTTP request directly to a node's address.
func (p *GitRelayProxy) directRequest(ctx context.Context, address string, req *GitProxyRequest) (*GitProxyResponse, error) {
	url := address + req.Path

	var body io.Reader
	if len(req.Body) > 0 {
		body = bytes.NewReader(req.Body)
	}

	httpReq, err := http.NewRequestWithContext(ctx, req.Method, url, body)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	if req.ContentType != "" {
		httpReq.Header.Set("Content-Type", req.ContentType)
	}
	for k, v := range req.Headers {
		httpReq.Header.Set(k, v)
	}

	resp, err := p.httpC.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("direct request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	headers := make(map[string]string)
	for k := range resp.Header {
		headers[k] = resp.Header.Get(k)
	}

	return &GitProxyResponse{
		StatusCode:  resp.StatusCode,
		Headers:     headers,
		Body:        respBody,
		ContentType: resp.Header.Get("Content-Type"),
	}, nil
}

// relayRequest sends a Git HTTP request via the Nydus relay network.
func (p *GitRelayProxy) relayRequest(ctx context.Context, targetNodeID string, req *GitProxyRequest) (*GitProxyResponse, error) {
	payload, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	relay := p.nydus.RelayClient()
	if relay == nil {
		return nil, fmt.Errorf("no relay client available")
	}

	respData, err := relay.Forward(ctx, targetNodeID, payload)
	if err != nil {
		return nil, fmt.Errorf("relay forward: %w", err)
	}

	var resp GitProxyResponse
	if err := json.Unmarshal(respData, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return &resp, nil
}

// HandleRelayedGitRequest processes an incoming relayed Git HTTP request on the target node.
// This is called when a Git relay message arrives.
func HandleRelayedGitRequest(handler *GitHTTPHandler, payload []byte) ([]byte, error) {
	var req GitProxyRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, fmt.Errorf("unmarshal git proxy request: %w", err)
	}

	log.Printf("[git-relay] handling relayed request: %s %s", req.Method, req.Path)

	// Create a fake HTTP request/response to feed into GitHTTPHandler
	var body io.Reader
	if len(req.Body) > 0 {
		body = bytes.NewReader(req.Body)
	}

	httpReq, err := http.NewRequest(req.Method, req.Path, body)
	if err != nil {
		return nil, fmt.Errorf("create http request: %w", err)
	}
	if req.ContentType != "" {
		httpReq.Header.Set("Content-Type", req.ContentType)
	}
	for k, v := range req.Headers {
		httpReq.Header.Set(k, v)
	}

	// Use a response recorder to capture the handler's output
	recorder := &responseRecorder{
		headers: make(http.Header),
		body:    &bytes.Buffer{},
	}

	handler.ServeHTTP(recorder, httpReq)

	headers := make(map[string]string)
	for k := range recorder.headers {
		headers[k] = recorder.headers.Get(k)
	}

	resp := &GitProxyResponse{
		StatusCode:  recorder.statusCode,
		Headers:     headers,
		Body:        recorder.body.Bytes(),
		ContentType: recorder.headers.Get("Content-Type"),
	}

	return json.Marshal(resp)
}

// responseRecorder captures HTTP handler output for relay transport.
type responseRecorder struct {
	headers    http.Header
	body       *bytes.Buffer
	statusCode int
}

func (r *responseRecorder) Header() http.Header {
	return r.headers
}

func (r *responseRecorder) Write(data []byte) (int, error) {
	if r.statusCode == 0 {
		r.statusCode = http.StatusOK
	}
	return r.body.Write(data)
}

func (r *responseRecorder) WriteHeader(code int) {
	r.statusCode = code
}
