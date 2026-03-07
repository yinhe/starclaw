package browser

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
	"github.com/ysmood/gson"
)

// Manager manages headless browser sessions
type Manager struct {
	mu       sync.Mutex
	browser  *rod.Browser
	sessions map[string]*Session
}

// Session represents an active browser session with a page
type Session struct {
	ID      string
	Page    *rod.Page
	Created time.Time
}

// ActionResult is the result of a browser action
type ActionResult struct {
	Action     string `json:"action"`
	URL        string `json:"url,omitempty"`
	Title      string `json:"title,omitempty"`
	Content    string `json:"content,omitempty"`
	Screenshot string `json:"screenshot,omitempty"` // base64 PNG
	Error      string `json:"error,omitempty"`
}

// NewManager creates a new browser manager
func NewManager() *Manager {
	return &Manager{
		sessions: make(map[string]*Session),
	}
}

func (m *Manager) ensureBrowser() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.browser != nil {
		return nil
	}

	// Try to find system chromium first, then auto-download
	path, _ := launcher.LookPath()
	var l *launcher.Launcher
	if path != "" {
		l = launcher.New().Bin(path)
	} else {
		l = launcher.New()
	}

	u := l.
		Headless(true).
		NoSandbox(true).
		Set("disable-gpu").
		Set("disable-dev-shm-usage").
		Set("no-first-run").
		Set("disable-default-apps").
		MustLaunch()

	browser := rod.New().ControlURL(u)
	if err := browser.Connect(); err != nil {
		return fmt.Errorf("failed to connect to browser: %w", err)
	}

	m.browser = browser
	log.Println("[Browser] Headless Chrome started")
	return nil
}

// GetOrCreateSession gets or creates a browser session
func (m *Manager) GetOrCreateSession(sessionID string) (*Session, error) {
	if err := m.ensureBrowser(); err != nil {
		return nil, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if s, ok := m.sessions[sessionID]; ok {
		return s, nil
	}

	page, err := m.browser.Page(proto.TargetCreateTarget{URL: "about:blank"})
	if err != nil {
		return nil, fmt.Errorf("failed to create page: %w", err)
	}

	// Set viewport
	page.MustSetViewport(1280, 720, 1, false)

	session := &Session{
		ID:      sessionID,
		Page:    page,
		Created: time.Now(),
	}
	m.sessions[sessionID] = session
	return session, nil
}

// Navigate goes to a URL
func (m *Manager) Navigate(ctx context.Context, sessionID, url string) (*ActionResult, error) {
	session, err := m.GetOrCreateSession(sessionID)
	if err != nil {
		return nil, err
	}

	err = session.Page.Timeout(30 * time.Second).Navigate(url)
	if err != nil {
		return &ActionResult{Action: "navigate", Error: err.Error()}, nil
	}

	// Wait for page to be ready (with timeout)
	_ = session.Page.Timeout(10 * time.Second).WaitStable(300 * time.Millisecond)

	info, _ := session.Page.Info()
	title := ""
	currentURL := url
	if info != nil {
		title = info.Title
		currentURL = info.URL
	}

	screenshot := m.takeScreenshot(session)

	return &ActionResult{
		Action:     "navigate",
		URL:        currentURL,
		Title:      title,
		Screenshot: screenshot,
	}, nil
}

// Click clicks an element by CSS selector
func (m *Manager) Click(ctx context.Context, sessionID, selector string) (*ActionResult, error) {
	session, err := m.GetOrCreateSession(sessionID)
	if err != nil {
		return nil, err
	}

	el, err := session.Page.Context(ctx).Timeout(5 * time.Second).Element(selector)
	if err != nil {
		return &ActionResult{Action: "click", Error: fmt.Sprintf("element not found: %s", selector)}, nil
	}

	err = el.Click(proto.InputMouseButtonLeft, 1)
	if err != nil {
		return &ActionResult{Action: "click", Error: err.Error()}, nil
	}

	time.Sleep(500 * time.Millisecond)
	_ = session.Page.Timeout(10 * time.Second).WaitStable(300 * time.Millisecond)

	info, _ := session.Page.Info()
	title := ""
	currentURL := ""
	if info != nil {
		title = info.Title
		currentURL = info.URL
	}

	screenshot := m.takeScreenshot(session)

	return &ActionResult{
		Action:     "click",
		URL:        currentURL,
		Title:      title,
		Screenshot: screenshot,
	}, nil
}

// Type types text into an element
func (m *Manager) Type(ctx context.Context, sessionID, selector, text string) (*ActionResult, error) {
	session, err := m.GetOrCreateSession(sessionID)
	if err != nil {
		return nil, err
	}

	el, err := session.Page.Context(ctx).Timeout(5 * time.Second).Element(selector)
	if err != nil {
		return &ActionResult{Action: "type", Error: fmt.Sprintf("element not found: %s", selector)}, nil
	}

	el.MustSelectAllText().MustInput(text)

	screenshot := m.takeScreenshot(session)

	return &ActionResult{
		Action:     "type",
		Content:    fmt.Sprintf("Typed '%s' into %s", text, selector),
		Screenshot: screenshot,
	}, nil
}

// Screenshot takes a screenshot of the current page
func (m *Manager) Screenshot(ctx context.Context, sessionID string) (*ActionResult, error) {
	session, err := m.GetOrCreateSession(sessionID)
	if err != nil {
		return nil, err
	}

	info, _ := session.Page.Info()
	title := ""
	currentURL := ""
	if info != nil {
		title = info.Title
		currentURL = info.URL
	}

	screenshot := m.takeScreenshot(session)

	return &ActionResult{
		Action:     "screenshot",
		URL:        currentURL,
		Title:      title,
		Screenshot: screenshot,
	}, nil
}

// ExtractText extracts visible text from the page or a specific element
func (m *Manager) ExtractText(ctx context.Context, sessionID, selector string) (*ActionResult, error) {
	session, err := m.GetOrCreateSession(sessionID)
	if err != nil {
		return nil, err
	}

	var text string
	if selector != "" {
		el, err := session.Page.Context(ctx).Timeout(5 * time.Second).Element(selector)
		if err != nil {
			return &ActionResult{Action: "extract_text", Error: fmt.Sprintf("element not found: %s", selector)}, nil
		}
		text, _ = el.Text()
	} else {
		text, _ = session.Page.MustElement("body").Text()
	}

	// Truncate
	if len(text) > 5000 {
		text = text[:5000] + "...(truncated)"
	}

	return &ActionResult{
		Action:  "extract_text",
		Content: text,
	}, nil
}

// Scroll scrolls the page
func (m *Manager) Scroll(ctx context.Context, sessionID, direction string, amount int) (*ActionResult, error) {
	session, err := m.GetOrCreateSession(sessionID)
	if err != nil {
		return nil, err
	}

	if amount == 0 {
		amount = 500
	}

	switch direction {
	case "down":
		session.Page.Mouse.MustScroll(0, float64(amount))
	case "up":
		session.Page.Mouse.MustScroll(0, -float64(amount))
	default:
		session.Page.Mouse.MustScroll(0, float64(amount))
	}

	time.Sleep(300 * time.Millisecond)
	screenshot := m.takeScreenshot(session)

	return &ActionResult{
		Action:     "scroll",
		Content:    fmt.Sprintf("Scrolled %s by %d pixels", direction, amount),
		Screenshot: screenshot,
	}, nil
}

// CloseSession closes a browser session
func (m *Manager) CloseSession(sessionID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if s, ok := m.sessions[sessionID]; ok {
		s.Page.MustClose()
		delete(m.sessions, sessionID)
	}
}

// Close shuts down the browser
func (m *Manager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for id, s := range m.sessions {
		s.Page.MustClose()
		delete(m.sessions, id)
	}

	if m.browser != nil {
		m.browser.MustClose()
		m.browser = nil
	}
}

func (m *Manager) takeScreenshot(session *Session) string {
	data, err := session.Page.Screenshot(true, &proto.PageCaptureScreenshot{
		Format:  proto.PageCaptureScreenshotFormatJpeg,
		Quality: gson.Int(50),
	})
	if err != nil {
		return ""
	}
	// Store in cache and return URL path
	id := GetCache().Store(data, "image/jpeg")
	return "/v1/screenshots/" + id
}
