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
	discordAPIBase = "https://discord.com/api/v10"
)

// DiscordTool provides Discord messaging capabilities for AI agents.
type DiscordTool struct {
	db     *gorm.DB
	client *http.Client
}

func NewDiscordTool(db *gorm.DB) *DiscordTool {
	return &DiscordTool{
		db:     db,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

func (t *DiscordTool) Name() string { return "discord" }

func (t *DiscordTool) Description() string {
	return "Discord messaging tool: send messages to channels, send webhook messages, list guild channels. Configure Discord Bot Token in Integration settings first."
}

func (t *DiscordTool) Parameters() interface{} {
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
				Description: "Integration ID (from settings). If omitted, uses the first enabled Discord integration.",
			},
			"channel_id": {
				Type:        "string",
				Description: "For send_message: Discord channel ID to send to.",
			},
			"guild_id": {
				Type:        "string",
				Description: "For list_channels: Discord guild (server) ID.",
			},
			"content": {
				Type:        "string",
				Description: "Message content. Supports Discord markdown.",
			},
			"embed_title": {
				Type:        "string",
				Description: "Optional embed title for richer messages.",
			},
			"embed_description": {
				Type:        "string",
				Description: "Optional embed description.",
			},
			"embed_color": {
				Type:        "string",
				Description: "Optional embed color as decimal integer (e.g. 3447003 for blue).",
			},
			"webhook_url": {
				Type:        "string",
				Description: "For send_webhook: override the configured webhook URL.",
			},
		},
		Required: []string{"action"},
	}
}

type discordArgs struct {
	Action           string `json:"action"`
	IntegrationID    string `json:"integration_id"`
	ChannelID        string `json:"channel_id"`
	GuildID          string `json:"guild_id"`
	Content          string `json:"content"`
	EmbedTitle       string `json:"embed_title"`
	EmbedDescription string `json:"embed_description"`
	EmbedColor       string `json:"embed_color"`
	WebhookURL       string `json:"webhook_url"`
}

func (t *DiscordTool) Execute(ctx context.Context, args string) (string, error) {
	parsed, err := ParseArgs[discordArgs](args)
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
		return t.listChannels(ctx, cfg, parsed.GuildID)
	default:
		return "", fmt.Errorf("unknown action: %s. Supported: send_message, send_webhook, list_channels", parsed.Action)
	}
}

func (t *DiscordTool) resolveIntegration(ctx context.Context, integrationID string) (*model.Integration, *model.DiscordConfig, error) {
	var integration model.Integration
	if integrationID != "" {
		if err := t.db.WithContext(ctx).Where("id = ? AND type = ? AND enabled = ?", integrationID, model.IntegrationDiscord, true).First(&integration).Error; err != nil {
			return nil, nil, fmt.Errorf("Discord integration not found or disabled (id=%s)", integrationID)
		}
	} else {
		userID, _ := ctx.Value(CtxKeyUserID).(string)
		query := t.db.WithContext(ctx).Where("type = ? AND enabled = ?", model.IntegrationDiscord, true)
		if userID != "" {
			query = query.Where("user_id = ?", userID)
		}
		if err := query.First(&integration).Error; err != nil {
			return nil, nil, fmt.Errorf("no Discord integration configured. Add one in Settings → Integrations")
		}
	}
	var cfg model.DiscordConfig
	if err := json.Unmarshal([]byte(integration.Config), &cfg); err != nil {
		return nil, nil, fmt.Errorf("failed to parse Discord config: %w", err)
	}
	return &integration, &cfg, nil
}

func (t *DiscordTool) sendMessage(ctx context.Context, cfg *model.DiscordConfig, args discordArgs) (string, error) {
	if cfg.BotToken == "" {
		return "", fmt.Errorf("Discord Bot Token not configured")
	}
	if args.ChannelID == "" {
		return "", fmt.Errorf("channel_id is required for send_message")
	}

	payload := map[string]interface{}{
		"content": args.Content,
	}

	// Add embed if title or description provided
	if args.EmbedTitle != "" || args.EmbedDescription != "" {
		embed := map[string]interface{}{}
		if args.EmbedTitle != "" {
			embed["title"] = args.EmbedTitle
		}
		if args.EmbedDescription != "" {
			embed["description"] = args.EmbedDescription
		}
		if args.EmbedColor != "" {
			var color int
			fmt.Sscanf(args.EmbedColor, "%d", &color)
			if color > 0 {
				embed["color"] = color
			}
		}
		payload["embeds"] = []interface{}{embed}
	}

	payloadBytes, _ := json.Marshal(payload)
	url := fmt.Sprintf("%s/channels/%s/messages", discordAPIBase, args.ChannelID)
	req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(payloadBytes)))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bot "+cfg.BotToken)

	resp, err := t.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send Discord message: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 10*1024))

	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("Discord API error (%d): %s", resp.StatusCode, string(body))
	}

	var result struct {
		ID        string `json:"id"`
		ChannelID string `json:"channel_id"`
	}
	json.Unmarshal(body, &result)
	return fmt.Sprintf("Message sent (id: %s, channel: %s)", result.ID, result.ChannelID), nil
}

func (t *DiscordTool) sendWebhook(ctx context.Context, cfg *model.DiscordConfig, args discordArgs) (string, error) {
	webhookURL := args.WebhookURL
	if webhookURL == "" {
		webhookURL = cfg.WebhookURL
	}
	if webhookURL == "" {
		return "", fmt.Errorf("no Webhook URL configured")
	}

	payload := map[string]interface{}{"content": args.Content}

	if args.EmbedTitle != "" || args.EmbedDescription != "" {
		embed := map[string]interface{}{}
		if args.EmbedTitle != "" {
			embed["title"] = args.EmbedTitle
		}
		if args.EmbedDescription != "" {
			embed["description"] = args.EmbedDescription
		}
		payload["embeds"] = []interface{}{embed}
	}

	payloadBytes, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, "POST", webhookURL, strings.NewReader(string(payloadBytes)))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := t.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("Discord webhook failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 10*1024))
		return "", fmt.Errorf("Discord webhook error (%d): %s", resp.StatusCode, string(body))
	}
	return "Discord webhook message sent successfully", nil
}

func (t *DiscordTool) listChannels(ctx context.Context, cfg *model.DiscordConfig, guildID string) (string, error) {
	if cfg.BotToken == "" {
		return "", fmt.Errorf("Discord Bot Token not configured")
	}
	if guildID == "" {
		return "", fmt.Errorf("guild_id is required for list_channels")
	}

	url := fmt.Sprintf("%s/guilds/%s/channels", discordAPIBase, guildID)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bot "+cfg.BotToken)

	resp, err := t.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to list channels: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 50*1024))

	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("Discord API error (%d): %s", resp.StatusCode, string(body))
	}

	var channels []struct {
		ID       string `json:"id"`
		Name     string `json:"name"`
		Type     int    `json:"type"`
		Position int    `json:"position"`
		Topic    string `json:"topic"`
	}
	if err := json.Unmarshal(body, &channels); err != nil {
		return fmt.Sprintf("Failed to parse: %s", string(body)), nil
	}

	// Filter to text channels (type 0) and voice channels (type 2)
	typeNames := map[int]string{0: "text", 2: "voice", 4: "category", 5: "announcement", 13: "stage", 15: "forum"}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%d channels:\n", len(channels)))
	for i, ch := range channels {
		typeName := typeNames[ch.Type]
		if typeName == "" {
			typeName = fmt.Sprintf("type-%d", ch.Type)
		}
		sb.WriteString(fmt.Sprintf("%d. #%s (id: %s, %s)\n", i+1, ch.Name, ch.ID, typeName))
	}
	return sb.String(), nil
}
