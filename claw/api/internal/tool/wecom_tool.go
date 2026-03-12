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
	wecomTokenURL   = "https://qyapi.weixin.qq.com/cgi-bin/gettoken"
	wecomMessageURL = "https://qyapi.weixin.qq.com/cgi-bin/message/send"
	wecomDeptURL    = "https://qyapi.weixin.qq.com/cgi-bin/department/simplelist"
)

// WeComTool provides WeCom (企业微信) messaging capabilities for AI agents.
type WeComTool struct {
	db     *gorm.DB
	client *http.Client
}

func NewWeComTool(db *gorm.DB) *WeComTool {
	return &WeComTool{
		db:     db,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

func (t *WeComTool) Name() string { return "wecom" }

func (t *WeComTool) Description() string {
	return "企业微信通讯工具：发送应用消息、群机器人 Webhook 推送。需要先在「集成设置」中配置企业微信应用凭证。"
}

func (t *WeComTool) Parameters() interface{} {
	return JSONSchema{
		Type: "object",
		Properties: map[string]Property{
			"action": {
				Type:        "string",
				Description: "Action to perform",
				Enum:        []string{"send_message", "send_webhook"},
			},
			"integration_id": {
				Type:        "string",
				Description: "Integration ID (from settings). If omitted, uses the first enabled WeCom integration.",
			},
			"to_user": {
				Type:        "string",
				Description: "For send_message: user IDs separated by '|'. Use '@all' for everyone.",
			},
			"to_party": {
				Type:        "string",
				Description: "For send_message: department IDs separated by '|'.",
			},
			"to_tag": {
				Type:        "string",
				Description: "For send_message: tag IDs separated by '|'.",
			},
			"msg_type": {
				Type:        "string",
				Description: "Message type: text, markdown, textcard, news. Default: text.",
				Enum:        []string{"text", "markdown", "textcard", "news"},
			},
			"content": {
				Type:        "string",
				Description: "Message content. Plain text for text/markdown, JSON for textcard/news.",
			},
			"title": {
				Type:        "string",
				Description: "For textcard: card title.",
			},
			"url": {
				Type:        "string",
				Description: "For textcard: click URL.",
			},
			"webhook_url": {
				Type:        "string",
				Description: "For send_webhook: override the configured webhook URL.",
			},
		},
		Required: []string{"action"},
	}
}

type wecomArgs struct {
	Action        string `json:"action"`
	IntegrationID string `json:"integration_id"`
	ToUser        string `json:"to_user"`
	ToParty       string `json:"to_party"`
	ToTag         string `json:"to_tag"`
	MsgType       string `json:"msg_type"`
	Content       string `json:"content"`
	Title         string `json:"title"`
	URL           string `json:"url"`
	WebhookURL    string `json:"webhook_url"`
}

func (t *WeComTool) Execute(ctx context.Context, args string) (string, error) {
	parsed, err := ParseArgs[wecomArgs](args)
	if err != nil {
		return "", err
	}

	integration, cfg, err := t.resolveIntegration(ctx, parsed.IntegrationID)
	if err != nil {
		return "", err
	}

	switch parsed.Action {
	case "send_message":
		return t.sendMessage(ctx, integration, cfg, parsed)
	case "send_webhook":
		return t.sendWebhook(ctx, cfg, parsed)
	default:
		return "", fmt.Errorf("unknown action: %s. Supported: send_message, send_webhook", parsed.Action)
	}
}

func (t *WeComTool) resolveIntegration(ctx context.Context, integrationID string) (*model.Integration, *model.WeComConfig, error) {
	var integration model.Integration
	if integrationID != "" {
		if err := t.db.WithContext(ctx).Where("id = ? AND type = ? AND enabled = ?", integrationID, model.IntegrationWeCom, true).First(&integration).Error; err != nil {
			return nil, nil, fmt.Errorf("企业微信集成未找到或未启用 (id=%s)", integrationID)
		}
	} else {
		userID, _ := ctx.Value(CtxKeyUserID).(string)
		query := t.db.WithContext(ctx).Where("type = ? AND enabled = ?", model.IntegrationWeCom, true)
		if userID != "" {
			query = query.Where("user_id = ?", userID)
		}
		if err := query.First(&integration).Error; err != nil {
			return nil, nil, fmt.Errorf("未配置企业微信集成，请先在「设置 → 集成」中添加企业微信应用凭证")
		}
	}
	var cfg model.WeComConfig
	if err := json.Unmarshal([]byte(integration.Config), &cfg); err != nil {
		return nil, nil, fmt.Errorf("企业微信配置解析失败: %w", err)
	}
	return &integration, &cfg, nil
}

func (t *WeComTool) getAccessToken(ctx context.Context, cfg *model.WeComConfig) (string, error) {
	if cfg.CorpID == "" || cfg.Secret == "" {
		return "", fmt.Errorf("企业微信 CorpID 和 Secret 未配置")
	}
	u := fmt.Sprintf("%s?corpid=%s&corpsecret=%s", wecomTokenURL, cfg.CorpID, cfg.Secret)
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return "", err
	}
	resp, err := t.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("获取企业微信 Token 失败: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 10*1024))
	var result struct {
		ErrCode     int    `json:"errcode"`
		ErrMsg      string `json:"errmsg"`
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("解析企业微信响应失败: %s", string(body))
	}
	if result.ErrCode != 0 {
		return "", fmt.Errorf("企业微信认证失败 (code=%d): %s", result.ErrCode, result.ErrMsg)
	}
	return result.AccessToken, nil
}

func (t *WeComTool) sendMessage(ctx context.Context, integration *model.Integration, cfg *model.WeComConfig, args wecomArgs) (string, error) {
	token, err := t.getAccessToken(ctx, cfg)
	if err != nil {
		return "", err
	}

	if args.ToUser == "" && args.ToParty == "" && args.ToTag == "" {
		args.ToUser = "@all"
	}

	msgType := args.MsgType
	if msgType == "" {
		msgType = "text"
	}

	payload := map[string]interface{}{
		"agentid":  cfg.AgentID,
		"msgtype":  msgType,
	}
	if args.ToUser != "" {
		payload["touser"] = args.ToUser
	}
	if args.ToParty != "" {
		payload["toparty"] = args.ToParty
	}
	if args.ToTag != "" {
		payload["totag"] = args.ToTag
	}

	switch msgType {
	case "text":
		payload["text"] = map[string]string{"content": args.Content}
	case "markdown":
		payload["markdown"] = map[string]string{"content": args.Content}
	case "textcard":
		title := args.Title
		if title == "" {
			title = "通知"
		}
		card := map[string]string{"title": title, "description": args.Content, "url": args.URL}
		payload["textcard"] = card
	default:
		return "", fmt.Errorf("不支持的消息类型: %s", msgType)
	}

	payloadBytes, _ := json.Marshal(payload)
	u := fmt.Sprintf("%s?access_token=%s", wecomMessageURL, token)
	req, err := http.NewRequestWithContext(ctx, "POST", u, strings.NewReader(string(payloadBytes)))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := t.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("发送企业微信消息失败: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 10*1024))
	var result struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	json.Unmarshal(body, &result)
	if result.ErrCode != 0 {
		return "", fmt.Errorf("企业微信发送失败 (code=%d): %s", result.ErrCode, result.ErrMsg)
	}
	return "企业微信应用消息发送成功", nil
}

func (t *WeComTool) sendWebhook(ctx context.Context, cfg *model.WeComConfig, args wecomArgs) (string, error) {
	webhookURL := args.WebhookURL
	if webhookURL == "" {
		webhookURL = cfg.WebhookURL
	}
	if webhookURL == "" {
		return "", fmt.Errorf("未配置 Webhook URL")
	}

	msgType := args.MsgType
	if msgType == "" {
		msgType = "text"
	}

	var payload map[string]interface{}
	switch msgType {
	case "text":
		payload = map[string]interface{}{"msgtype": "text", "text": map[string]string{"content": args.Content}}
	case "markdown":
		payload = map[string]interface{}{"msgtype": "markdown", "markdown": map[string]string{"content": args.Content}}
	default:
		return "", fmt.Errorf("Webhook 仅支持 text 和 markdown 类型")
	}

	payloadBytes, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, "POST", webhookURL, strings.NewReader(string(payloadBytes)))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := t.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("企业微信 Webhook 发送失败: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 10*1024))
	var result struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	json.Unmarshal(body, &result)
	if result.ErrCode != 0 {
		return "", fmt.Errorf("企业微信 Webhook 失败 (code=%d): %s", result.ErrCode, result.ErrMsg)
	}
	return "企业微信 Webhook 消息发送成功", nil
}
