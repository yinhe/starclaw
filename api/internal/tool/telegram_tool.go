package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/yinhe/starclaw/internal/model"
	"gorm.io/gorm"
)

const telegramAPIBase = "https://api.telegram.org/bot"

// TelegramTool provides Telegram messaging capabilities for AI agents.
type TelegramTool struct {
	db     *gorm.DB
	client *http.Client
}

func NewTelegramTool(db *gorm.DB) *TelegramTool {
	return &TelegramTool{
		db:     db,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

func (t *TelegramTool) Name() string { return "telegram" }

func (t *TelegramTool) Description() string {
	return "Telegram messaging tool: send text/markdown/HTML messages, send photos, get chat info. Configure Telegram Bot Token (from @BotFather) in Integration settings first."
}

func (t *TelegramTool) Parameters() interface{} {
	return JSONSchema{
		Type: "object",
		Properties: map[string]Property{
			"action": {
				Type:        "string",
				Description: "Action to perform",
				Enum:        []string{"send_message", "send_photo", "get_chat", "get_me"},
			},
			"integration_id": {
				Type:        "string",
				Description: "Integration ID (from settings). If omitted, uses the first enabled Telegram integration.",
			},
			"chat_id": {
				Type:        "string",
				Description: "Telegram chat ID (user, group, or channel). Uses default from config if omitted.",
			},
			"content": {
				Type:        "string",
				Description: "Message text content.",
			},
			"parse_mode": {
				Type:        "string",
				Description: "Text parsing mode: Markdown, MarkdownV2, or HTML. Default: Markdown.",
				Enum:        []string{"Markdown", "MarkdownV2", "HTML"},
			},
			"photo_url": {
				Type:        "string",
				Description: "For send_photo: URL of the photo to send.",
			},
			"caption": {
				Type:        "string",
				Description: "For send_photo: photo caption text.",
			},
			"disable_notification": {
				Type:        "string",
				Description: "Set to 'true' to send silently.",
			},
		},
		Required: []string{"action"},
	}
}

type telegramArgs struct {
	Action              string `json:"action"`
	IntegrationID       string `json:"integration_id"`
	ChatID              string `json:"chat_id"`
	Content             string `json:"content"`
	ParseMode           string `json:"parse_mode"`
	PhotoURL            string `json:"photo_url"`
	Caption             string `json:"caption"`
	DisableNotification string `json:"disable_notification"`
}

func (t *TelegramTool) Execute(ctx context.Context, args string) (string, error) {
	parsed, err := ParseArgs[telegramArgs](args)
	if err != nil {
		return "", err
	}

	_, cfg, err := t.resolveIntegration(ctx, parsed.IntegrationID)
	if err != nil {
		return "", err
	}

	// Use default chat_id from config if not specified
	if parsed.ChatID == "" {
		parsed.ChatID = cfg.ChatID
	}

	switch parsed.Action {
	case "send_message":
		return t.sendMessage(ctx, cfg, parsed)
	case "send_photo":
		return t.sendPhoto(ctx, cfg, parsed)
	case "get_chat":
		return t.getChat(ctx, cfg, parsed.ChatID)
	case "get_me":
		return t.getMe(ctx, cfg)
	default:
		return "", fmt.Errorf("unknown action: %s. Supported: send_message, send_photo, get_chat, get_me", parsed.Action)
	}
}

func (t *TelegramTool) resolveIntegration(ctx context.Context, integrationID string) (*model.Integration, *model.TelegramConfig, error) {
	var integration model.Integration
	if integrationID != "" {
		if err := t.db.WithContext(ctx).Where("id = ? AND type = ? AND enabled = ?", integrationID, model.IntegrationTelegram, true).First(&integration).Error; err != nil {
			return nil, nil, fmt.Errorf("Telegram integration not found or disabled (id=%s)", integrationID)
		}
	} else {
		userID, _ := ctx.Value(CtxKeyUserID).(string)
		query := t.db.WithContext(ctx).Where("type = ? AND enabled = ?", model.IntegrationTelegram, true)
		if userID != "" {
			query = query.Where("user_id = ?", userID)
		}
		if err := query.First(&integration).Error; err != nil {
			return nil, nil, fmt.Errorf("no Telegram integration configured. Add one in Settings → Integrations")
		}
	}
	var cfg model.TelegramConfig
	if err := json.Unmarshal([]byte(integration.Config), &cfg); err != nil {
		return nil, nil, fmt.Errorf("failed to parse Telegram config: %w", err)
	}
	return &integration, &cfg, nil
}

func (t *TelegramTool) apiURL(token, method string) string {
	return telegramAPIBase + token + "/" + method
}

func (t *TelegramTool) sendMessage(ctx context.Context, cfg *model.TelegramConfig, args telegramArgs) (string, error) {
	if cfg.BotToken == "" {
		return "", fmt.Errorf("Telegram Bot Token not configured")
	}
	if args.ChatID == "" {
		return "", fmt.Errorf("chat_id is required (specify in parameters or set default in integration config)")
	}

	parseMode := args.ParseMode
	if parseMode == "" {
		parseMode = "Markdown"
	}

	payload := map[string]interface{}{
		"chat_id":    args.ChatID,
		"text":       args.Content,
		"parse_mode": parseMode,
	}
	if args.DisableNotification == "true" {
		payload["disable_notification"] = true
	}

	return t.callAPI(ctx, cfg.BotToken, "sendMessage", payload)
}

func (t *TelegramTool) sendPhoto(ctx context.Context, cfg *model.TelegramConfig, args telegramArgs) (string, error) {
	if cfg.BotToken == "" {
		return "", fmt.Errorf("Telegram Bot Token not configured")
	}
	if args.ChatID == "" {
		return "", fmt.Errorf("chat_id is required")
	}
	if args.PhotoURL == "" {
		return "", fmt.Errorf("photo_url is required for send_photo")
	}

	payload := map[string]interface{}{
		"chat_id": args.ChatID,
		"photo":   args.PhotoURL,
	}
	if args.Caption != "" {
		payload["caption"] = args.Caption
		if args.ParseMode != "" {
			payload["parse_mode"] = args.ParseMode
		}
	}
	if args.DisableNotification == "true" {
		payload["disable_notification"] = true
	}

	return t.callAPI(ctx, cfg.BotToken, "sendPhoto", payload)
}

func (t *TelegramTool) getChat(ctx context.Context, cfg *model.TelegramConfig, chatID string) (string, error) {
	if cfg.BotToken == "" {
		return "", fmt.Errorf("Telegram Bot Token not configured")
	}
	if chatID == "" {
		return "", fmt.Errorf("chat_id is required for get_chat")
	}

	payload := map[string]interface{}{"chat_id": chatID}
	return t.callAPI(ctx, cfg.BotToken, "getChat", payload)
}

func (t *TelegramTool) getMe(ctx context.Context, cfg *model.TelegramConfig) (string, error) {
	if cfg.BotToken == "" {
		return "", fmt.Errorf("Telegram Bot Token not configured")
	}
	return t.callAPI(ctx, cfg.BotToken, "getMe", nil)
}

func (t *TelegramTool) callAPI(ctx context.Context, token, method string, payload map[string]interface{}) (string, error) {
	var reqBody io.Reader
	if payload != nil {
		b, _ := json.Marshal(payload)
		reqBody = strings.NewReader(string(b))
	}

	req, err := http.NewRequestWithContext(ctx, "POST", t.apiURL(token, method), reqBody)
	if err != nil {
		return "", err
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := t.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("Telegram API call failed: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 20*1024))

	var result struct {
		OK          bool            `json:"ok"`
		Description string          `json:"description"`
		Result      json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Sprintf("API responded but parse failed: %s", string(body)), nil
	}
	if !result.OK {
		return "", fmt.Errorf("Telegram API error: %s", result.Description)
	}

	// Format result nicely depending on method
	switch method {
	case "sendMessage":
		var msg struct {
			MessageID int `json:"message_id"`
			Chat      struct {
				ID    int64  `json:"id"`
				Title string `json:"title"`
			} `json:"chat"`
		}
		json.Unmarshal(result.Result, &msg)
		chatName := msg.Chat.Title
		if chatName == "" {
			chatName = fmt.Sprintf("%d", msg.Chat.ID)
		}
		return fmt.Sprintf("Message sent (id: %d, chat: %s)", msg.MessageID, chatName), nil

	case "sendPhoto":
		var msg struct {
			MessageID int `json:"message_id"`
		}
		json.Unmarshal(result.Result, &msg)
		return fmt.Sprintf("Photo sent (message_id: %d)", msg.MessageID), nil

	case "getMe":
		var bot struct {
			ID        int64  `json:"id"`
			FirstName string `json:"first_name"`
			Username  string `json:"username"`
		}
		json.Unmarshal(result.Result, &bot)
		return fmt.Sprintf("Bot: @%s (%s, id: %d)", bot.Username, bot.FirstName, bot.ID), nil

	case "getChat":
		var chat struct {
			ID          int64  `json:"id"`
			Type        string `json:"type"`
			Title       string `json:"title"`
			Description string `json:"description"`
		}
		json.Unmarshal(result.Result, &chat)
		return fmt.Sprintf("Chat: %s (id: %d, type: %s, desc: %s)", chat.Title, chat.ID, chat.Type, chat.Description), nil

	default:
		return string(result.Result), nil
	}
}
