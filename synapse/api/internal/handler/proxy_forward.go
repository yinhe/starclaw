package handler

import (
	"io"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yinhe/starclaw-router/internal/proxy"
)

// ProxyHandler forwards requests to the Node.js overseas relay proxy as-is
type ProxyHandler struct {
	proxy *proxy.Client
}

func NewProxyHandler(proxyClient *proxy.Client) *ProxyHandler {
	return &ProxyHandler{proxy: proxyClient}
}

// Forward sends the request to the proxy, streaming the response back
func (h *ProxyHandler) Forward(c *gin.Context) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	headers := http.Header{}
	headers.Set("Content-Type", c.GetHeader("Content-Type"))
	if ct := c.GetHeader("Content-Type"); ct != "" {
		headers.Set("Content-Type", ct)
	}

	path := c.Request.URL.Path // e.g. /v1/images/generations
	err = h.proxy.ForwardStream(c.Writer, c.Request.Method, path, bytesReader(body), headers)
	if err != nil {
		log.Printf("[star-ai] proxy forward error on %s: %v", path, err)
		c.JSON(http.StatusBadGateway, gin.H{
			"error": gin.H{
				"message": "proxy service unreachable",
				"type":    "server_error",
			},
		})
	}
}

type bytesReaderWrapper struct {
	*io.SectionReader
}

func bytesReader(b []byte) io.Reader {
	return io.NopCloser(ioReader(b))
}

func ioReader(b []byte) io.Reader {
	return &byteBuf{data: b, pos: 0}
}

type byteBuf struct {
	data []byte
	pos  int
}

func (b *byteBuf) Read(p []byte) (n int, err error) {
	if b.pos >= len(b.data) {
		return 0, io.EOF
	}
	n = copy(p, b.data[b.pos:])
	b.pos += n
	return
}
