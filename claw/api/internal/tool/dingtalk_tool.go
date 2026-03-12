package tool

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/yinhe/starclaw/internal/model"
	"gorm.io/gorm"
)

const (
	dingtalkTokenURL   = "https://oapi.dingtalk.com/gettoken"
	dingtalkMessageURL = "https://oapi.dingtalk.com/topapi/message/corpconversation/asyncsend_v2"
	dingtalkChatSendURL = "https://oapi.dingtalk.com/chat/send"
	dingtalkDeptListURL = "https://oapi.dingtalk.com/topapi/v2/department/listsub"
)

// DingtalkTool provides DingTalk (钉钉) messaging capabilities for AI agents.
type DingtalkTool struct {
	db     *gorm.DB
	client *http.Client
}

func NewDingtalkTool(db *gorm.DB) *DingtalkTool {
	return &DingtalkTool{
		db:     db,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

func (t *DingtalkTool) Name() string { return "dingtalk" }

func (t *DingtalkTool) Description() string {
	return "钉钉通讯工具：发送工作通知、群 Webhook 推送、列出部门。需要先在「集成设置」中配置钉钉应用凭证。"
}

func (t *DingtalkTool) Parameters() interface{} {
	return JSONSchema{
		Type: "object",
		Properties: map[string]Property{
			"action": {
				Type:        "string",
				Description: "Action to perform",
				Enum:        []string{"send_message", "send_webhook", "list_departments"},
			},
			"integration_id": {
				Type:        "string",
				Description: "Integration ID (from settings). If omitted, uses the first enabled DingTalk integration.",
			},
			"user_ids": {
				Type:        "string",
				Description: "For send_message: comma-separated DingTalk user IDs to send work notification to.",
			},
			"dept_ids": {
				Type:        "string",
				Description: "For send_message: comma-separated department IDs. Send to all members in these departments.",
			},
			"to_all_user": {
				Type:        "string",
				Description: "For send_message: set to 'true' to send to all employees.",
			},
			"content": {
				Type:        "string",
				Description: "Message content. Plain text for text messages, or JSON for other types.",
			},
			"msg_type": {
				Type:        "string",
				Description: "Message type: text, markdown, action_card. Default: text.",
				Enum:        []string{"text", "markdown", "action_card"},
			},
			"title": {
				Type:        "string",
				Description: "For markdown/action_card: message title.",
			},
			"webhook_url": {
				Type:        "string",
				Description: "For send_webhook: override the configured webhook URL.",
			},
		},
		Required: []string{"action"},
	}
}

type dingtalkArgs struct {
	Action        string `json:"action"`
	IntegrationID string `json:"integration_id"`
	UserIDs       string `json:"user_ids"`
	DeptIDs       string `json:"dept_ids"`
	ToAllUser     string `json:"to_all_user"`
	Content       string `json:"content"`
	MsgType       string `json:"msg_type"`
	Title         string `json:"title"`
	WebhookURL    string `json:"webhook_url"`
}

func (t *DingtalkTool) Execute(ctx context.Context, args string) (string, error) {
	parsed, err := ParseArgs[dingtalkArgs](args)
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
	case "list_departments":
		return t.listDepartments(ctx, integration, cfg)
	default:
		return "", fmt.Errorf("unknown action: %s. Supported: send_message, send_webhook, list_departments", parsed.Action)
	}
}

func (t *DingtalkTool) resolveIntegration(ctx context.Context, integrationID string) (*model.Integration, *model.DingtalkConfig, error) {
	var integration model.Integration
	if integrationID != "" {
		if err := t.db.WithContext(ctx).Where("id = ? AND type = ? AND enabled = ?", integrationID, model.IntegrationDingtalk, true).First(&integration).Error; err != nil {
			return nil, nil, fmt.Errorf("钉钉集成未找到或未启用 (id=%s)", integrationID)
		}
	} else {
		userID, _ := ctx.Value(CtxKeyUserID).(string)
		query := t.db.WithContext(ctx).Where("type = ? AND enabled = ?", model.IntegrationDingtalk, true)
		if userID != "" {
			query = query.Where("user_id = ?", userID)
		}
		if err := query.First(&integration).Error; err != nil {
			return nil, nil, fmt.Errorf("未配置钉钉集成，请先在「设置 → 集成」中添加钉钉应用凭证")
		}
	}
	var cfg model.DingtalkConfig
	if err := json.Unmarshal([]byte(integration.Config), &cfg); err != nil {
		return nil, nil, fmt.Errorf("钉钉配置解析失败: %w", err)
	}
	return &integration, &cfg, nil
}

func (t *DingtalkTool) getAccessToken(ctx context.Context, cfg *model.DingtalkConfig) (string, error) {
	if cfg.AppKey == "" || cfg.AppSecret == "" {
		return "", fmt.Errorf("钉钉 AppKey 和 AppSecret 未配置")
	}
	u := fmt.Sprintf("%s?appkey=%s&appsecret=%s", dingtalkTokenURL, url.QueryEscape(cfg.AppKey), url.QueryEscape(cfg.AppSecret))
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return "", err
	}
	resp, err := t.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("获取钉钉 Token 失败: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 10*1024))
	var result struct {
		ErrCode     int    `json:"errcode"`
		ErrMsg      string `json:"errmsg"`
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("解析钉钉响应失败: %s", string(body))
	}
	if result.ErrCode != 0 {
		return "", fmt.Errorf("钉钉认证失败 (code=%d): %s", result.ErrCode, result.ErrMsg)
	}
	return result.AccessToken, nil
}

func (t *DingtalkTool) sendMessage(ctx context.Context, integration *model.Integration, cfg *model.DingtalkConfig, args dingtalkArgs) (string, error) {
	token, err := t.getAccessToken(ctx, cfg)
	if err != nil {
		return "", err
	}
	if args.UserIDs == "" && args.DeptIDs == "" && args.ToAllUser != "true" {
		return "", fmt.Errorf("至少指定 user_ids、dept_ids 或 to_all_user=true")
	}

	msgType := args.MsgType
	if msgType == "" {
		msgType = "text"
	}

	var msg map[string]interface{}
	switch msgType {
	case "text":
		msg = map[string]interface{}{"msgtype": "text", "text": map[string]string{"content": args.Content}}
	case "markdown":
		title := args.Title
		if title == "" {
			title = "通知"
		}
		msg = map[string]interface{}{"msgtype": "markdown", "markdown": map[string]string{"title": title, "text": args.Content}}
	case "action_card":
		title := args.Title
		if title == "" {
			title = "通知"
		}
		msg = map[string]interface{}{"msgtype": "action_card", "action_card": map[string]string{"title": title, "markdown": args.Content, "single_title": "查看详情", "single_url": ""}}
	default:
		return "", fmt.Errorf("不支持的消息类型: %s", msgType)
	}

	payload := map[string]interface{}{"agent_id": cfg.AppKey, "msg": msg}
	if args.UserIDs != "" {
		payload["userid_list"] = args.UserIDs
	}
	if args.DeptIDs != "" {
		payload["dept_id_list"] = args.DeptIDs
	}
	if args.ToAllUser == "true" {
		payload["to_all_user"] = true
	}

	payloadBytes, _ := json.Marshal(payload)
	u := fmt.Sprintf("%s?access_token=%s", dingtalkMessageURL, token)
	req, err := http.NewRequestWithContext(ctx, "POST", u, strings.NewReader(string(payloadBytes)))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := t.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("发送钉钉消息失败: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 10*1024))
	var result struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
		TaskID  int64  `json:"task_id"`
	}
	json.Unmarshal(body, &result)
	if result.ErrCode != 0 {
		return "", fmt.Errorf("钉钉发送失败 (code=%d): %s", result.ErrCode, result.ErrMsg)
	}
	return fmt.Sprintf("钉钉工作通知发送成功，task_id: %d", result.TaskID), nil
}

func (t *DingtalkTool) sendWebhook(ctx context.Context, cfg *model.DingtalkConfig, args dingtalkArgs) (string, error) {
	webhookURL := args.WebhookURL
	if webhookURL == "" {
		webhookURL = cfg.WebhookURL
	}
	if webhookURL == "" {
		return "", fmt.Errorf("未配置 Webhook URL")
	}

	// If signing secret is set, add signature
	if cfg.SignSecret != "" {
		ts := fmt.Sprintf("%d", time.Now().UnixMilli())
		stringToSign := ts + "\n" + cfg.SignSecret
		mac := hmac.New(sha256.New, []byte(cfg.SignSecret))
		mac.Write([]byte(stringToSign))
		sign := url.QueryEscape(base64.StdEncoding.EncodeToString(mac.Sum(nil)))
		webhookURL += fmt.Sprintf("&timestamp=%s&sign=%s", ts, sign)
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
		title := args.Title
		if title == "" {
			title = "通知"
		}
		payload = map[string]interface{}{"msgtype": "markdown", "markdown": map[string]string{"title": title, "text": args.Content}}
	case "action_card":
		title := args.Title
		if title == "" {
			title = "通知"
		}
		payload = map[string]interface{}{"msgtype": "actionCard", "actionCard": map[string]string{"title": title, "text": args.Content, "singleTitle": "查看详情", "singleURL": ""}}
	default:
		return "", fmt.Errorf("不支持的消息类型: %s", msgType)
	}

	payloadBytes, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, "POST", webhookURL, strings.NewReader(string(payloadBytes)))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := t.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("钉钉 Webhook 发送失败: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 10*1024))
	var result struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	json.Unmarshal(body, &result)
	if result.ErrCode != 0 {
		return "", fmt.Errorf("钉钉 Webhook 失败 (code=%d): %s", result.ErrCode, result.ErrMsg)
	}
	return "钉钉 Webhook 消息发送成功", nil
}

func (t *DingtalkTool) listDepartments(ctx context.Context, integration *model.Integration, cfg *model.DingtalkConfig) (string, error) {
	token, err := t.getAccessToken(ctx, cfg)
	if err != nil {
		return "", err
	}

	payload := `{"dept_id":1}`
	u := fmt.Sprintf("%s?access_token=%s", dingtalkDeptListURL, token)
	req, err := http.NewRequestWithContext(ctx, "POST", u, strings.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := t.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("获取部门列表失败: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 50*1024))

	var result struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
		Result  []struct {
			DeptID   int64  `json:"dept_id"`
			Name     string `json:"name"`
			ParentID int64  `json:"parent_id"`
		} `json:"result"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Sprintf("获取部门列表但解析失败: %s", string(body)), nil
	}
	if result.ErrCode != 0 {
		return "", fmt.Errorf("获取部门列表失败 (code=%d): %s", result.ErrCode, result.ErrMsg)
	}

	if len(result.Result) == 0 {
		return "未找到子部门", nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("共 %d 个子部门:\n", len(result.Result)))
	for i, dept := range result.Result {
		sb.WriteString(fmt.Sprintf("%d. %s (dept_id: %d, parent_id: %d)\n", i+1, dept.Name, dept.DeptID, dept.ParentID))
	}
	return sb.String(), nil
}
