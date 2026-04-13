package media

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"math"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

var multimodalWSUpgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

type sttWSClientMessage struct {
	Type              string `json:"type"`
	Format            string `json:"format,omitempty"`
	SampleRate        int    `json:"sample_rate,omitempty"`
	Channels          int    `json:"channels,omitempty"`
	EndpointMs        int    `json:"endpoint_ms,omitempty"`
	PartialIntervalMs int    `json:"partial_interval_ms,omitempty"`
}

type sttWSServerMessage struct {
	Type         string `json:"type"`
	SessionID    string `json:"session_id,omitempty"`
	Text         string `json:"text,omitempty"`
	Message      string `json:"message,omitempty"`
	DurationMs   int    `json:"duration_ms,omitempty"`
	ChunkCount   int    `json:"chunk_count,omitempty"`
	SampleRate   int    `json:"sample_rate,omitempty"`
	Speeching    bool   `json:"speeching,omitempty"`
	ProviderMode string `json:"provider_mode,omitempty"`
}

type liveSTTSession struct {
	handler *MultimodalHandler
	conn    *websocket.Conn
	writeMu sync.Mutex
	mu      sync.Mutex

	userID    string
	sessionID string

	sampleRate int
	channels   int

	pcm               bytes.Buffer
	chunkCount        int
	speechStarted     bool
	lastSpeechAt      time.Time
	partialText       string
	lastPartialAt     time.Time
	lastPartialBytes  int
	transcribing      bool
	finalSent         bool
	closed            bool
	endpointSilence   time.Duration
	partialInterval   time.Duration
	speechThreshold   float64
	minPartialBytes   int
	partialDeltaBytes int
	minFinalBytes     int
}

func newLiveSTTSession(handler *MultimodalHandler, conn *websocket.Conn, userID string) *liveSTTSession {
	return &liveSTTSession{
		handler:           handler,
		conn:              conn,
		userID:            userID,
		sessionID:         uuid.New().String(),
		sampleRate:        16000,
		channels:          1,
		endpointSilence:   1200 * time.Millisecond,
		partialInterval:   900 * time.Millisecond,
		speechThreshold:   700,
		minPartialBytes:   16000,
		partialDeltaBytes: 12000,
		minFinalBytes:     8000,
	}
}

func (s *liveSTTSession) writeJSON(msg sttWSServerMessage) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	return s.conn.WriteJSON(msg)
}

func (s *liveSTTSession) writeError(message string) {
	_ = s.writeJSON(sttWSServerMessage{Type: "error", Message: message})
}

func (s *liveSTTSession) applyConfig(msg sttWSClientMessage) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if msg.SampleRate > 0 {
		s.sampleRate = msg.SampleRate
	}
	if msg.Channels > 0 {
		s.channels = msg.Channels
	}
	if msg.EndpointMs >= 600 {
		s.endpointSilence = time.Duration(msg.EndpointMs) * time.Millisecond
	}
	if msg.PartialIntervalMs >= 500 {
		s.partialInterval = time.Duration(msg.PartialIntervalMs) * time.Millisecond
	}
	bytesPerSecond := s.sampleRate * s.channels * 2
	if bytesPerSecond > 0 {
		s.minPartialBytes = maxIntSTT(bytesPerSecond*8/10, 8000)
		s.partialDeltaBytes = maxIntSTT(bytesPerSecond/2, 8000)
		s.minFinalBytes = maxIntSTT(bytesPerSecond/4, 4000)
	}
}

func (s *liveSTTSession) appendPCM(chunk []byte) {
	now := time.Now()
	energy := pcmAverageAbs(chunk)

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.finalSent {
		return
	}
	s.chunkCount++
	s.pcm.Write(chunk)
	if energy >= s.speechThreshold {
		if !s.speechStarted {
			s.speechStarted = true
			go s.writeJSON(sttWSServerMessage{Type: "speech_start", SessionID: s.sessionID, Speeching: true})
		}
		s.lastSpeechAt = now
	}
}

func (s *liveSTTSession) shouldEmitPartial() ([]byte, int, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.finalSent || s.transcribing || !s.speechStarted {
		return nil, 0, false
	}
	length := s.pcm.Len()
	if length < s.minPartialBytes {
		return nil, 0, false
	}
	if time.Since(s.lastPartialAt) < s.partialInterval {
		return nil, 0, false
	}
	if length-s.lastPartialBytes < s.partialDeltaBytes {
		return nil, 0, false
	}
	copyBytes := append([]byte(nil), s.pcm.Bytes()...)
	s.transcribing = true
	return copyBytes, length, true
}

func (s *liveSTTSession) shouldEmitFinal() ([]byte, int, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.finalSent || s.transcribing || !s.speechStarted {
		return nil, 0, false
	}
	length := s.pcm.Len()
	if length < s.minFinalBytes || s.lastSpeechAt.IsZero() {
		return nil, 0, false
	}
	if time.Since(s.lastSpeechAt) < s.endpointSilence {
		return nil, 0, false
	}
	copyBytes := append([]byte(nil), s.pcm.Bytes()...)
	s.transcribing = true
	return copyBytes, length, true
}

func (s *liveSTTSession) emitPartial(pcm []byte, pcmLen int) {
	wav := pcm16ToWav(pcm, s.sampleRate, s.channels)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	text, err := s.handler.transcribeAudio(ctx, s.userID, "stream_partial.wav", wav)
	if err != nil {
		s.mu.Lock()
		s.transcribing = false
		s.mu.Unlock()
		s.writeError(err.Error())
		return
	}
	text = strings.TrimSpace(text)

	s.mu.Lock()
	defer s.mu.Unlock()
	s.transcribing = false
	if s.closed || s.finalSent || text == "" || text == s.partialText {
		return
	}
	s.partialText = text
	s.lastPartialAt = time.Now()
	s.lastPartialBytes = pcmLen
	go s.writeJSON(sttWSServerMessage{
		Type:         "partial",
		SessionID:    s.sessionID,
		Text:         text,
		DurationMs:   pcmDurationMs(pcmLen, s.sampleRate, s.channels),
		ChunkCount:   s.chunkCount,
		SampleRate:   s.sampleRate,
		ProviderMode: "server_vad_buffered_realtime",
	})
}

func (s *liveSTTSession) emitFinal(pcm []byte, pcmLen int) {
	wav := pcm16ToWav(pcm, s.sampleRate, s.channels)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	text, err := s.handler.transcribeAudio(ctx, s.userID, "stream_final.wav", wav)
	if err != nil {
		s.mu.Lock()
		s.transcribing = false
		s.mu.Unlock()
		s.writeError(err.Error())
		return
	}
	text = strings.TrimSpace(text)

	s.mu.Lock()
	defer s.mu.Unlock()
	s.transcribing = false
	if s.closed || s.finalSent {
		return
	}
	s.finalSent = true
	go s.writeJSON(sttWSServerMessage{Type: "speech_end", SessionID: s.sessionID, Speeching: false})
	go s.writeJSON(sttWSServerMessage{
		Type:         "final",
		SessionID:    s.sessionID,
		Text:         text,
		DurationMs:   pcmDurationMs(pcmLen, s.sampleRate, s.channels),
		ChunkCount:   s.chunkCount,
		SampleRate:   s.sampleRate,
		ProviderMode: "server_vad_buffered_realtime",
	})
}

func (s *liveSTTSession) forceFinalize() {
	s.mu.Lock()
	if s.closed || s.transcribing || s.finalSent {
		s.mu.Unlock()
		return
	}
	pcm := append([]byte(nil), s.pcm.Bytes()...)
	pcmLen := len(pcm)
	if pcmLen == 0 {
		s.closed = true
		s.mu.Unlock()
		return
	}
	s.transcribing = true
	s.mu.Unlock()
	s.emitFinal(pcm, pcmLen)
}

func (s *liveSTTSession) close() {
	s.mu.Lock()
	s.closed = true
	s.mu.Unlock()
}

func (s *liveSTTSession) runTicker(done <-chan struct{}) {
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			if pcm, pcmLen, ok := s.shouldEmitFinal(); ok {
				go s.emitFinal(pcm, pcmLen)
				continue
			}
			if pcm, pcmLen, ok := s.shouldEmitPartial(); ok {
				go s.emitPartial(pcm, pcmLen)
			}
		}
	}
}

func (h *MultimodalHandler) SpeechToTextWebSocket(c *gin.Context) {
	userID := c.GetString("user_id")
	if len(h.findSTTAttempts(userID)) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "未配置支持语音识别的模型提供商（需要 Qwen 或 OpenAI）"})
		return
	}

	conn, err := multimodalWSUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	session := newLiveSTTSession(h, conn, userID)
	_ = session.writeJSON(sttWSServerMessage{
		Type:         "ready",
		SessionID:    session.sessionID,
		SampleRate:   session.sampleRate,
		ProviderMode: "server_vad_buffered_realtime",
	})

	done := make(chan struct{})
	defer close(done)
	go session.runTicker(done)

	for {
		messageType, data, err := conn.ReadMessage()
		if err != nil {
			session.close()
			return
		}

		switch messageType {
		case websocket.TextMessage:
			var msg sttWSClientMessage
			if err := json.Unmarshal(data, &msg); err != nil {
				session.writeError("invalid control message")
				continue
			}
			switch msg.Type {
			case "start":
				session.applyConfig(msg)
				_ = session.writeJSON(sttWSServerMessage{
					Type:         "listening",
					SessionID:    session.sessionID,
					SampleRate:   session.sampleRate,
					ProviderMode: "server_vad_buffered_realtime",
				})
			case "stop":
				session.forceFinalize()
				time.Sleep(150 * time.Millisecond)
				session.close()
				_ = session.writeJSON(sttWSServerMessage{Type: "closed", SessionID: session.sessionID})
				return
			case "cancel":
				session.close()
				_ = session.writeJSON(sttWSServerMessage{Type: "cancelled", SessionID: session.sessionID})
				return
			default:
				session.writeError("unsupported control type")
			}
		case websocket.BinaryMessage:
			session.appendPCM(data)
		default:
			session.writeError("unsupported websocket frame")
		}
	}
}

func pcmAverageAbs(chunk []byte) float64 {
	if len(chunk) < 2 {
		return 0
	}
	var sum float64
	count := 0
	for i := 0; i+1 < len(chunk); i += 2 {
		sample := int16(binary.LittleEndian.Uint16(chunk[i : i+2]))
		sum += math.Abs(float64(sample))
		count++
	}
	if count == 0 {
		return 0
	}
	return sum / float64(count)
}

func pcm16ToWav(pcm []byte, sampleRate, channels int) []byte {
	if sampleRate <= 0 {
		sampleRate = 16000
	}
	if channels <= 0 {
		channels = 1
	}
	dataLen := len(pcm)
	byteRate := sampleRate * channels * 2
	blockAlign := channels * 2
	buf := &bytes.Buffer{}
	buf.WriteString("RIFF")
	_ = binary.Write(buf, binary.LittleEndian, uint32(36+dataLen))
	buf.WriteString("WAVE")
	buf.WriteString("fmt ")
	_ = binary.Write(buf, binary.LittleEndian, uint32(16))
	_ = binary.Write(buf, binary.LittleEndian, uint16(1))
	_ = binary.Write(buf, binary.LittleEndian, uint16(channels))
	_ = binary.Write(buf, binary.LittleEndian, uint32(sampleRate))
	_ = binary.Write(buf, binary.LittleEndian, uint32(byteRate))
	_ = binary.Write(buf, binary.LittleEndian, uint16(blockAlign))
	_ = binary.Write(buf, binary.LittleEndian, uint16(16))
	buf.WriteString("data")
	_ = binary.Write(buf, binary.LittleEndian, uint32(dataLen))
	buf.Write(pcm)
	return buf.Bytes()
}

func pcmDurationMs(pcmLen, sampleRate, channels int) int {
	denominator := sampleRate * maxIntSTT(channels, 1) * 2
	if denominator <= 0 {
		return 0
	}
	return int((float64(pcmLen) / float64(denominator)) * 1000)
}

func maxIntSTT(a, b int) int {
	if a > b {
		return a
	}
	return b
}
