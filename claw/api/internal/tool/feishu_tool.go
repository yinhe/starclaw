package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/yinhe/starclaw/internal/model"
	"gorm.io/gorm"
)

const (
	feishuTokenURL   = "https://open.feishu.cn/open-apis/auth/v3/tenant_access_token/internal"
	feishuMessageURL = "https://open.feishu.cn/open-apis/im/v1/messages"
	feishuChatsURL   = "https://open.feishu.cn/open-apis/im/v1/chats"
)

// FeishuTool provides Feishu (Lark) messaging capabilities for AI agents.
// Supports: send_message, send_webhook, send_card, list_chats, get_chat_members.
type FeishuTool struct {
	db     *gorm.DB
	client *http.Client

	mu          sync.RWMutex
	tokenCache  map[string]*feishuTokenEntry // integrationID -> cached token
}

type feishuTokenEntry struct {
	Token     string
	ExpiresAt time.Time
}

func NewFeishuTool(db *gorm.DB) *FeishuTool {
	return &FeishuTool{
		db:         db,
		client:     &http.Client{Timeout: 30 * time.Second},
		tokenCache: make(map[string]*feishuTokenEntry),
	}
}

func (t *FeishuTool) Name() string { return "feishu" }

func (t *FeishuTool) Description() string {
	return "飞书通讯工具：发送消息、发送卡片、Webhook 推送、列出群组。需要先在「集成设置」中配置飞书应用凭证。"
}

func (t *FeishuTool) Parameters() interface{} {
	return JSONSchema{
		Type: "object",
		Properties: map[string]Property{
			"action": {
				Type:        "string",
				Description: "Action to perform",
				Enum:        []string{"send_message", "send_webhook", "send_card", "list_chats", "get_chat_members"},
			},
			"integration_id": {
				Type:        "string",
				Description: "Integration ID (from settings). If omitted, uses the first enabled Feishu integration.",
			},
			"receive_id": {
				Type:        "string",
				Description: "For send_message/send_card: recipient ID. Can be open_id, user_id, email, or chat_id.",
			},
			"receive_id_type": {
				Type:        "string",
				Description: "Type of receive_id: open_id, user_id, email, or chat_id. Default: chat_id.",
				Enum:        []string{"open_id", "user_id", "email", "chat_id"},
			},
			"msg_type": {
				Type:        "string",
				Description: "Message type for send_message: text, post, image, interactive. Default: text.",
				Enum:        []string{"text", "post", "image", "interactive"},
			},
			"content": {
				Type:        "string",
				Description: "Message content. For text: plain text string. For post/interactive: JSON string of rich content.",
			},
			"chat_id": {
				Type:        "string",
				Description: "For get_chat_members: the chat ID to query members.",
			},
			"webhook_url": {
				Type:        "string",
				Description: "For send_webhook: override the configured webhook URL.",
			},
		},
		Required: []string{"action"},
	}
}

type feishuArgs struct {
	Action        string `json:"action"`
	IntegrationID string `json:"integration_id"`
	ReceiveID     string `json:"receive_id"`
	ReceiveIDType string `json:"receive_id_type"`
	MsgType       string `json:"msg_type"`
	Content       string `json:"content"`
	ChatID        string `json:"chat_id"`
	WebhookURL    string `json:"webhook_url"`
}

func (t *FeishuTool) Execute(ctx context.Context, args string) (string, error) {
	parsed, err := ParseArgs[feishuArgs](args)
	if err != nil {
		return "", err
	}

	// Resolve integration
	integration, cfg, err := t.resolveIntegration(ctx, parsed.IntegrationID)
	if err != nil {
		return "", err
	}

	switch parsed.Action {
	case "send_message":
		return t.sendMessage(ctx, integration, cfg, parsed)
	case "send_webhook":
		return t.sendWebhook(ctx, cfg, parsed)
	case "send_card":
		return t.sendCard(ctx, integration, cfg, parsed)
	case "list_chats":
		return t.listChats(ctx, integration, cfg)
	case "get_chat_members":
		return t.getChatMembers(ctx, integration, cfg, parsed.ChatID)
	default:
		return "", fmt.Errorf("unknown action: %s. Supported: send_message, send_webhook, send_card, list_chats, get_chat_members", parsed.Action)
	}
}

// resolveIntegration finds the Feishu integration and parses its config
func (t *FeishuTool) resolveIntegration(ctx context.Context, integrationID string) (*model.Integration, *model.FeishuConfig, error) {
	var integration model.Integration

	if integrationID != "" {
		if err := t.db.WithContext(ctx).Where("id = ? AND type = ? AND enabled = ?", integrationID, model.IntegrationFeishu, true).First(&integration).Error; err != nil {
			return nil, nil, fmt.Errorf("飞书集成未找到或未启用 (id=%s)", integrationID)
		}
	} else {
		// Use the first enabled Feishu integration
		userID, _ := ctx.Value(CtxKeyUserID).(string)
		query := t.db.WithContext(ctx).Where("type = ? AND enabled = ?", model.IntegrationFeishu, true)
		if userID != "" {
			query = query.Where("user_id = ?", userID)
		}
		if err := query.First(&integration).Error; err != nil {
			return nil, nil, fmt.Errorf("未配置飞书集成，请先在「设置 → 集成」中添加飞书应用凭证")
		}
	}

	var cfg model.FeishuConfig
	if err := json.Unmarshal([]byte(integration.Config), &cfg); err != nil {
		return nil, nil, fmt.Errorf("飞书配置解析失败: %w", err)
	}
	return &integration, &cfg, nil
}

// getTenantToken obtains or refreshes the tenant_access_token
func (t *FeishuTool) getTenantToken(ctx context.Context, integration *model.Integration, cfg *model.FeishuConfig) (string, error) {
	if cfg.AppID == "" || cfg.AppSecret == "" {
		return "", fmt.Errorf("飞书 App ID 和 App Secret 未配置，无法获取 Token")
	}

	// Check cache
	t.mu.RLock()
	if entry, ok := t.tokenCache[integration.ID]; ok && time.Now().Before(entry.ExpiresAt) {
		t.mu.RUnlock()
		return entry.Token, nil
	}
	t.mu.RUnlock()

	// Fetch new token
	body := fmt.Sprintf(`{"app_id":"%s","app_secret":"%s"}`, cfg.AppID, cfg.AppSecret)
	req, err := http.NewRequestWithContext(ctx, "POST", feishuTokenURL, strings.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	resp, err := t.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("获取飞书 Token 失败: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 10*1024))

	var result struct {
		Code              int    `json:"code"`
		Msg               string `json:"msg"`
		TenantAccessToken string `json:"tenant_access_token"`
		Expire            int    `json:"expire"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("解析飞书响应失败: %s", string(respBody))
	}
	if result.Code != 0 {
		return "", fmt.Errorf("飞书认证失败 (code=%d): %s", result.Code, result.Msg)
	}

	// Cache token (expire 10 minutes early for safety)
	t.mu.Lock()
	t.tokenCache[integration.ID] = &feishuTokenEntry{
		Token:     result.TenantAccessToken,
		ExpiresAt: time.Now().Add(time.Duration(result.Expire-600) * time.Second),
	}
	t.mu.Unlock()

	log.Printf("[Feishu] Token refreshed for integration %s, expires in %ds", integration.ID, result.Expire)
	return result.TenantAccessToken, nil
}

// sendMessage sends a message via Feishu Open API
func (t *FeishuTool) sendMessage(ctx context.Context, integration *model.Integration, cfg *model.FeishuConfig, args feishuArgs) (string, error) {
	token, err := t.getTenantToken(ctx, integration, cfg)
	if err != nil {
		return "", err
	}

	if args.ReceiveID == "" {
		return "", fmt.Errorf("receive_id 不能为空，请指定接收者 ID（open_id, user_id, email 或 chat_id）")
	}

	receiveIDType := args.ReceiveIDType
	if receiveIDType == "" {
		receiveIDType = "chat_id"
	}

	msgType := args.MsgType
	if msgType == "" {
		msgType = "text"
	}

	// Build content JSON
	var contentJSON string
	if msgType == "text" {
		contentObj := map[string]string{"text": args.Content}
		b, _ := json.Marshal(contentObj)
		contentJSON = string(b)
	} else {
		// For post/interactive/image, content should already be valid JSON
		contentJSON = args.Content
	}

	payload := map[string]string{
		"receive_id": args.ReceiveID,
		"msg_type":   msgType,
		"content":    contentJSON,
	}
	payloadBytes, _ := json.Marshal(payload)

	url := fmt.Sprintf("%s?receive_id_type=%s", feishuMessageURL, receiveIDType)
	req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(payloadBytes)))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := t.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("发送消息失败: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 10*1024))
	var result struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			MessageID string `json:"message_id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return fmt.Sprintf("发送成功但解析响应失败: %s", string(respBody)), nil
	}
	if result.Code != 0 {
		return "", fmt.Errorf("飞书发送失败 (code=%d): %s", result.Code, result.Msg)
	}

	return fmt.Sprintf("消息发送成功，message_id: %s", result.Data.MessageID), nil
}

// sendWebhook sends a message via Feishu Custom Bot Webhook (no auth needed)
func (t *FeishuTool) sendWebhook(ctx context.Context, cfg *model.FeishuConfig, args feishuArgs) (string, error) {
	webhookURL := args.WebhookURL
	if webhookURL == "" {
		webhookURL = cfg.WebhookURL
	}
	if webhookURL == "" {
		return "", fmt.Errorf("未配置 Webhook URL，请在集成设置中配置或在参数中指定 webhook_url")
	}

	// Build webhook payload
	msgType := args.MsgType
	if msgType == "" {
		msgType = "text"
	}

	var payload map[string]interface{}
	if msgType == "text" {
		payload = map[string]interface{}{
			"msg_type": "text",
			"content": map[string]string{
				"text": args.Content,
			},
		}
	} else if msgType == "interactive" {
		var card interface{}
		if err := json.Unmarshal([]byte(args.Content), &card); err != nil {
			return "", fmt.Errorf("卡片 JSON 格式错误: %w", err)
		}
		payload = map[string]interface{}{
			"msg_type": "interactive",
			"card":     card,
		}
	} else if msgType == "post" {
		var post interface{}
		if err := json.Unmarshal([]byte(args.Content), &post); err != nil {
			return "", fmt.Errorf("富文本 JSON 格式错误: %w", err)
		}
		payload = map[string]interface{}{
			"msg_type": "post",
			"content": map[string]interface{}{
				"post": post,
			},
		}
	} else {
		return "", fmt.Errorf("Webhook 不支持消息类型: %s (支持 text, post, interactive)", msgType)
	}

	payloadBytes, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, "POST", webhookURL, strings.NewReader(string(payloadBytes)))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	resp, err := t.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("Webhook 发送失败: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 10*1024))

	var result struct {
		Code int         `json:"code"`
		Msg  string      `json:"msg"`
		Data interface{} `json:"data"`
	}
	json.Unmarshal(respBody, &result)

	if result.Code != 0 {
		return "", fmt.Errorf("Webhook 发送失败 (code=%d): %s", result.Code, result.Msg)
	}

	return "Webhook 消息发送成功", nil
}

// sendCard sends an interactive card message
func (t *FeishuTool) sendCard(ctx context.Context, integration *model.Integration, cfg *model.FeishuConfig, args feishuArgs) (string, error) {
	// Card is just a send_message with msg_type=interactive
	args.MsgType = "interactive"
	return t.sendMessage(ctx, integration, cfg, args)
}

// listChats lists all chats the bot is in
func (t *FeishuTool) listChats(ctx context.Context, integration *model.Integration, cfg *model.FeishuConfig) (string, error) {
	token, err := t.getTenantToken(ctx, integration, cfg)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, "GET", feishuChatsURL+"?page_size=50", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := t.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("获取群组列表失败: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 50*1024))

	var result struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			Items []struct {
				ChatID      string `json:"chat_id"`
				Name        string `json:"name"`
				Description string `json:"description"`
				OwnerID     string `json:"owner_id"`
				ChatMode    string `json:"chat_mode"`
				MemberCount int    `json:"member_count,omitempty"`
			} `json:"items"`
			HasMore   bool   `json:"has_more"`
			PageToken string `json:"page_token"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return fmt.Sprintf("获取群组列表但解析失败: %s", string(respBody)), nil
	}
	if result.Code != 0 {
		return "", fmt.Errorf("获取群组列表失败 (code=%d): %s", result.Code, result.Msg)
	}

	if len(result.Data.Items) == 0 {
		return "Bot 当前未加入任何群组", nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("共 %d 个群组:\n", len(result.Data.Items)))
	for i, chat := range result.Data.Items {
		sb.WriteString(fmt.Sprintf("%d. %s (chat_id: %s, 成员: %d)\n", i+1, chat.Name, chat.ChatID, chat.MemberCount))
	}
	if result.Data.HasMore {
		sb.WriteString("（还有更多群组未显示）")
	}
	return sb.String(), nil
}

// getChatMembers lists members of a specific chat
func (t *FeishuTool) getChatMembers(ctx context.Context, integration *model.Integration, cfg *model.FeishuConfig, chatID string) (string, error) {
	if chatID == "" {
		return "", fmt.Errorf("chat_id 不能为空")
	}

	token, err := t.getTenantToken(ctx, integration, cfg)
	if err != nil {
		return "", err
	}

	url := fmt.Sprintf("%s/%s/members?page_size=50", feishuChatsURL, chatID)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := t.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("获取群成员失败: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 50*1024))

	var result struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			Items []struct {
				MemberID   string `json:"member_id"`
				MemberType string `json:"member_id_type"`
				Name       string `json:"name"`
			} `json:"items"`
			MemberTotal int  `json:"member_total"`
			HasMore     bool `json:"has_more"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return fmt.Sprintf("获取群成员但解析失败: %s", string(respBody)), nil
	}
	if result.Code != 0 {
		return "", fmt.Errorf("获取群成员失败 (code=%d): %s", result.Code, result.Msg)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("群成员共 %d 人:\n", result.Data.MemberTotal))
	for i, m := range result.Data.Items {
		sb.WriteString(fmt.Sprintf("%d. %s (id: %s, type: %s)\n", i+1, m.Name, m.MemberID, m.MemberType))
	}
	if result.Data.HasMore {
		sb.WriteString("（还有更多成员未显示）")
	}
	return sb.String(), nil
}
