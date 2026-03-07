package tool

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/yinhe/starclaw/internal/browser"
)

// BrowserTool allows agents to control a headless browser
type BrowserTool struct {
	manager *browser.Manager
}

// NewBrowserTool creates a new browser tool
func NewBrowserTool(mgr *browser.Manager) *BrowserTool {
	return &BrowserTool{manager: mgr}
}

func (t *BrowserTool) Name() string {
	return "browser"
}

func (t *BrowserTool) Description() string {
	return "控制无头浏览器，可以打开网页、点击元素、输入文字、截图、提取页面内容和滚动。用于代替用户操作网页。"
}

func (t *BrowserTool) Parameters() interface{} {
	return JSONSchema{
		Type: "object",
		Properties: map[string]Property{
			"action": {
				Type:        "string",
				Description: "The browser action to perform",
				Enum:        []string{"navigate", "click", "type", "screenshot", "extract_text", "scroll"},
			},
			"url": {
				Type:        "string",
				Description: "URL to navigate to (for 'navigate' action)",
			},
			"selector": {
				Type:        "string",
				Description: "CSS selector of the target element (for 'click', 'type', 'extract_text' actions)",
			},
			"text": {
				Type:        "string",
				Description: "Text to type into the element (for 'type' action)",
			},
			"direction": {
				Type:        "string",
				Description: "Scroll direction: 'up' or 'down' (for 'scroll' action)",
				Enum:        []string{"up", "down"},
			},
			"session_id": {
				Type:        "string",
				Description: "Browser session ID to reuse an existing tab. Leave empty for default session.",
			},
		},
		Required: []string{"action"},
	}
}

type browserArgs struct {
	Action    string `json:"action"`
	URL       string `json:"url,omitempty"`
	Selector  string `json:"selector,omitempty"`
	Text      string `json:"text,omitempty"`
	Direction string `json:"direction,omitempty"`
	SessionID string `json:"session_id,omitempty"`
}

func (t *BrowserTool) Execute(ctx context.Context, args string) (string, error) {
	var a browserArgs
	if err := json.Unmarshal([]byte(args), &a); err != nil {
		return "", fmt.Errorf("invalid browser args: %w", err)
	}

	if a.SessionID == "" {
		a.SessionID = "default"
	}

	var result *browser.ActionResult
	var err error

	switch a.Action {
	case "navigate":
		if a.URL == "" {
			return "", fmt.Errorf("url is required for navigate action")
		}
		result, err = t.manager.Navigate(ctx, a.SessionID, a.URL)

	case "click":
		if a.Selector == "" {
			return "", fmt.Errorf("selector is required for click action")
		}
		result, err = t.manager.Click(ctx, a.SessionID, a.Selector)

	case "type":
		if a.Selector == "" || a.Text == "" {
			return "", fmt.Errorf("selector and text are required for type action")
		}
		result, err = t.manager.Type(ctx, a.SessionID, a.Selector, a.Text)

	case "screenshot":
		result, err = t.manager.Screenshot(ctx, a.SessionID)

	case "extract_text":
		result, err = t.manager.ExtractText(ctx, a.SessionID, a.Selector)

	case "scroll":
		dir := a.Direction
		if dir == "" {
			dir = "down"
		}
		result, err = t.manager.Scroll(ctx, a.SessionID, dir, 500)

	default:
		return "", fmt.Errorf("unknown browser action: %s", a.Action)
	}

	if err != nil {
		return "", err
	}

	out, _ := json.Marshal(result)
	return string(out), nil
}
