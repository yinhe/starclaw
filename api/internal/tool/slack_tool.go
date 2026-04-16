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

const (
	slackPostMessageURL  = "https://slack.com/api/chat.postMessage"
	slackChannelsURL     = "https://slack.com/api/conversations.list"
	slackChannelInfoURL  = "https://slack.com/api/conversations.info"
)

// SlackTool provides Slack messaging capabilities for AI agents.
type SlackTool struct {
	db     *gorm.DB
	client *http.Client
}

func NewSlackTool(db *gorm.DB) *SlackTool {
	return &SlackTool{
		db:     db,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

func (t *SlackTool) Name() string { return "slack" }

func (t *SlackTool) Description() string {
	return "Slack messaging tool: send messages, send webhooks, list channels. Configure Slack Bot Token in Integration settings first."
}

func (t *SlackTool) Parameters() interface{} {
	return JSONSchema{
		Type: "object",
		Properties: map[string]Property{
			"action": {
				Type:        "string",
				Description: "Action to perform",
				Enum:        []string{"send_message", "send_webhook", "list_channels"},
			},
			"integration_id": {
				Type:        "string",
				Description: "Integration ID (from settings). If omitted, uses the first enabled Slack integration.",
			},
			"channel": {
				Type:        "string",
				Description: "For send_message: channel ID or name (e.g. #general or C01234ABCDE).",
			},
			"content": {
				Type:        "string",
				Description: "Message text. Supports Slack mrkdwn formatting.",
			},
			"blocks": {
				Type:        "string",
				Description: "For send_message: JSON array of Slack Block Kit blocks (optional, for rich layouts).",
			},
			"webhook_url": {
				Type:        "string",
				Description: "For send_webhook: override the configured webhook URL.",
			},
		},
		Required: []string{"action"},
	}
}

type slackArgs struct {
	Action        string `json:"action"`
	IntegrationID string `json:"integration_id"`
	Channel       string `json:"channel"`
	Content       string `json:"content"`
	Blocks        string `json:"blocks"`
	WebhookURL    string `json:"webhook_url"`
}

func (t *SlackTool) Execute(ctx context.Context, args string) (string, error) {
	parsed, err := ParseArgs[slackArgs](args)
	if err != nil {
		return "", err
	}

	_, cfg, err := t.resolveIntegration(ctx, parsed.IntegrationID)
	if err != nil {
		return "", err
	}

	switch parsed.Action {
	case "send_message":
		return t.sendMessage(ctx, cfg, parsed)
	case "send_webhook":
		return t.sendWebhook(ctx, cfg, parsed)
	case "list_channels":
		return t.listChannels(ctx, cfg)
	default:
		return "", fmt.Errorf("unknown action: %s. Supported: send_message, send_webhook, list_channels", parsed.Action)
	}
}

func (t *SlackTool) resolveIntegration(ctx context.Context, integrationID string) (*model.Integration, *model.SlackConfig, error) {
	var integration model.Integration
	if integrationID != "" {
		if err := t.db.WithContext(ctx).Where("id = ? AND type = ? AND enabled = ?", integrationID, model.IntegrationSlack, true).First(&integration).Error; err != nil {
			return nil, nil, fmt.Errorf("Slack integration not found or disabled (id=%s)", integrationID)
		}
	} else {
		userID, _ := ctx.Value(CtxKeyUserID).(string)
		query := t.db.WithContext(ctx).Where("type = ? AND enabled = ?", model.IntegrationSlack, true)
		if userID != "" {
			query = query.Where("user_id = ?", userID)
		}
		if err := query.First(&integration).Error; err != nil {
			return nil, nil, fmt.Errorf("no Slack integration configured. Add one in Settings → Integrations")
		}
	}
	var cfg model.SlackConfig
	if err := json.Unmarshal([]byte(integration.Config), &cfg); err != nil {
		return nil, nil, fmt.Errorf("failed to parse Slack config: %w", err)
	}
	return &integration, &cfg, nil
}

func (t *SlackTool) sendMessage(ctx context.Context, cfg *model.SlackConfig, args slackArgs) (string, error) {
	if cfg.BotToken == "" {
		return "", fmt.Errorf("Slack Bot Token not configured")
	}
	if args.Channel == "" {
		return "", fmt.Errorf("channel is required for send_message")
	}

	payload := map[string]interface{}{
		"channel": args.Channel,
		"text":    args.Content,
	}

	// Parse optional blocks
	if args.Blocks != "" {
		var blocks interface{}
		if err := json.Unmarshal([]byte(args.Blocks), &blocks); err != nil {
			return "", fmt.Errorf("invalid blocks JSON: %w", err)
		}
		payload["blocks"] = blocks
	}

	payloadBytes, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, "POST", slackPostMessageURL, strings.NewReader(string(payloadBytes)))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Authorization", "Bearer "+cfg.BotToken)

	resp, err := t.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send Slack message: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 10*1024))

	var result struct {
		OK      bool   `json:"ok"`
		Error   string `json:"error"`
		Channel string `json:"channel"`
		TS      string `json:"ts"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Sprintf("Message sent but failed to parse response: %s", string(body)), nil
	}
	if !result.OK {
		return "", fmt.Errorf("Slack API error: %s", result.Error)
	}
	return fmt.Sprintf("Message sent to %s (ts: %s)", result.Channel, result.TS), nil
}

func (t *SlackTool) sendWebhook(ctx context.Context, cfg *model.SlackConfig, args slackArgs) (string, error) {
	webhookURL := args.WebhookURL
	if webhookURL == "" {
		webhookURL = cfg.WebhookURL
	}
	if webhookURL == "" {
		return "", fmt.Errorf("no Webhook URL configured")
	}

	payload := map[string]interface{}{"text": args.Content}
	if args.Blocks != "" {
		var blocks interface{}
		if err := json.Unmarshal([]byte(args.Blocks), &blocks); err == nil {
			payload["blocks"] = blocks
		}
	}

	payloadBytes, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, "POST", webhookURL, strings.NewReader(string(payloadBytes)))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := t.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("Slack webhook failed: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("Slack webhook returned %d: %s", resp.StatusCode, string(body))
	}
	return "Slack webhook message sent successfully", nil
}

func (t *SlackTool) listChannels(ctx context.Context, cfg *model.SlackConfig) (string, error) {
	if cfg.BotToken == "" {
		return "", fmt.Errorf("Slack Bot Token not configured")
	}

	req, err := http.NewRequestWithContext(ctx, "GET", slackChannelsURL+"?types=public_channel,private_channel&limit=100", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+cfg.BotToken)

	resp, err := t.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to list channels: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 50*1024))

	var result struct {
		OK       bool   `json:"ok"`
		Error    string `json:"error"`
		Channels []struct {
			ID         string `json:"id"`
			Name       string `json:"name"`
			IsPrivate  bool   `json:"is_private"`
			NumMembers int    `json:"num_members"`
			Topic      struct {
				Value string `json:"value"`
			} `json:"topic"`
		} `json:"channels"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Sprintf("Failed to parse response: %s", string(body)), nil
	}
	if !result.OK {
		return "", fmt.Errorf("Slack API error: %s", result.Error)
	}

	if len(result.Channels) == 0 {
		return "No channels found (bot may not be invited to any channels)", nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%d channels:\n", len(result.Channels)))
	for i, ch := range result.Channels {
		privacy := "public"
		if ch.IsPrivate {
			privacy = "private"
		}
		sb.WriteString(fmt.Sprintf("%d. #%s (id: %s, %s, %d members)\n", i+1, ch.Name, ch.ID, privacy, ch.NumMembers))
	}
	return sb.String(), nil
}
