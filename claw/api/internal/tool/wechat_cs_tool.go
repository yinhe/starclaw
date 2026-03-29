package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/yinhe/starclaw/internal/model"
	"gorm.io/gorm"
)

// WeChatCSTool provides customer-service oriented WeChat operations.
// It is designed for bot-labeled support workflows: capture latest context,
// suggest escalation, and send approved replies via desktop automation.
type WeChatCSTool struct {
	db      *gorm.DB
	desktop *DesktopTool
}

func NewWeChatCSTool(db *gorm.DB) *WeChatCSTool {
	return &WeChatCSTool{db: db, desktop: NewDesktopTool()}
}

func (t *WeChatCSTool) Name() string { return "wechat_cs" }

func (t *WeChatCSTool) Description() string {
	return "微信群客服机器人技能：用于客服场景的微信桌面辅助。支持抓取最新聊天界面、发送已确认回复、标记转人工，以及对消息做基础意图判断。默认按机器人客服身份工作，不应用于伪装真人长期社交互动。"
}

func (t *WeChatCSTool) Parameters() interface{} {
	return JSONSchema{
		Type: "object",
		Properties: map[string]Property{
			"action": {
				Type:        "string",
				Description: "Action: capture_latest, send_reply, handoff_to_human, classify_message, enable_watch, list_watches, disable_watch",
				Enum:        []string{"capture_latest", "send_reply", "handoff_to_human", "classify_message", "enable_watch", "list_watches", "disable_watch"},
			},
			"target": {
				Type:        "string",
				Description: "微信群名或联系人名，用于 send_reply。",
			},
			"content": {
				Type:        "string",
				Description: "回复内容或待分类消息内容。",
			},
			"window_title": {
				Type:        "string",
				Description: "微信窗口标题，默认使用“微信”。",
			},
			"reason": {
				Type:        "string",
				Description: "转人工原因，用于 handoff_to_human。",
			},
			"customer_name": {
				Type:        "string",
				Description: "客户名/群昵称，用于转人工记录。",
			},
			"agent_id": {
				Type:        "string",
				Description: "客服机器人 Agent ID，用于 enable_watch。",
			},
			"watch_id": {
				Type:        "string",
				Description: "监控项 ID，用于 disable_watch。",
			},
			"mode": {
				Type:        "string",
				Description: "watch 模式：suggest_only 或 auto_task。默认 suggest_only。",
				Enum:        []string{"suggest_only", "auto_task"},
			},
			"poll_interval_sec": {
				Type:        "string",
				Description: "轮询间隔秒数，默认 20。",
			},
		},
		Required: []string{"action"},
	}
}

type wechatCSArgs struct {
	Action      string `json:"action"`
	Target      string `json:"target"`
	Content     string `json:"content"`
	WindowTitle string `json:"window_title"`
	Reason      string `json:"reason"`
	Customer    string `json:"customer_name"`
	AgentID     string `json:"agent_id"`
	WatchID     string `json:"watch_id"`
	Mode        string `json:"mode"`
	PollSec     string `json:"poll_interval_sec"`
}

func (t *WeChatCSTool) Execute(ctx context.Context, args string) (string, error) {
	parsed, err := ParseArgs[wechatCSArgs](args)
	if err != nil {
		return "", err
	}
	if parsed.WindowTitle == "" {
		parsed.WindowTitle = "微信"
	}

	switch parsed.Action {
	case "capture_latest":
		return t.captureLatest(ctx, parsed)
	case "send_reply":
		return t.sendReply(ctx, parsed)
	case "handoff_to_human":
		return t.handoffToHuman(parsed), nil
	case "classify_message":
		return t.classifyMessage(parsed), nil
	case "enable_watch":
		return t.enableWatch(ctx, parsed)
	case "list_watches":
		return t.listWatches(ctx)
	case "disable_watch":
		return t.disableWatch(ctx, parsed)
	default:
		return "", fmt.Errorf("unknown action: %s", parsed.Action)
	}
}

func (t *WeChatCSTool) captureLatest(ctx context.Context, a wechatCSArgs) (string, error) {
	focusArgs, _ := json.Marshal(map[string]interface{}{
		"action": "focus_window",
		"title":  a.WindowTitle,
	})
	focusOut, focusErr := t.desktop.Execute(ctx, string(focusArgs))

	screenArgs, _ := json.Marshal(map[string]interface{}{
		"action": "screenshot",
		"region": "full",
	})
	screenOut, screenErr := t.desktop.Execute(ctx, string(screenArgs))
	if screenErr != nil {
		return "", screenErr
	}

	return toJSON(map[string]interface{}{
		"action":          "capture_latest",
		"status":          "success",
		"window_title":    a.WindowTitle,
		"focus_result":    jsonOrRaw(focusOut, focusErr),
		"screenshot":      jsonOrRaw(screenOut, nil),
		"message":         "已抓取当前微信聊天界面截图。请结合截图判断客户最新消息并生成客服回复。",
		"bot_identity":    "客服机器人",
		"requires_review": true,
	}), nil
}

func (t *WeChatCSTool) sendReply(ctx context.Context, a wechatCSArgs) (string, error) {
	if strings.TrimSpace(a.Target) == "" {
		return "", fmt.Errorf("send_reply requires target")
	}
	if strings.TrimSpace(a.Content) == "" {
		return "", fmt.Errorf("send_reply requires content")
	}

	desktopArgs, _ := json.Marshal(map[string]interface{}{
		"action": "wechat_send",
		"title":  a.Target,
		"text":   a.Content,
	})
	out, err := t.desktop.Execute(ctx, string(desktopArgs))
	if err != nil {
		return "", err
	}
	var delivery map[string]interface{}
	if err := json.Unmarshal([]byte(out), &delivery); err == nil {
		if status, _ := delivery["status"].(string); strings.EqualFold(status, "error") {
			msg, _ := delivery["message"].(string)
			if strings.TrimSpace(msg) == "" {
				msg = "wechat_send returned error"
			}
			return "", fmt.Errorf("%s", msg)
		}
	}

	return toJSON(map[string]interface{}{
		"action":          "send_reply",
		"status":          "success",
		"target":          a.Target,
		"bot_identity":    "客服机器人",
		"reply":           a.Content,
		"delivery_result": jsonOrRaw(out, nil),
		"message":         fmt.Sprintf("已按客服机器人身份向“%s”发送回复。", a.Target),
	}), nil
}

func (t *WeChatCSTool) handoffToHuman(a wechatCSArgs) string {
	reason := strings.TrimSpace(a.Reason)
	if reason == "" {
		reason = "需要人工进一步处理"
	}
	return toJSON(map[string]interface{}{
		"action":        "handoff_to_human",
		"status":        "success",
		"customer_name": a.Customer,
		"reason":        reason,
		"priority":      handoffPriority(reason),
		"message":       fmt.Sprintf("已标记转人工：%s", reason),
	})
}

func (t *WeChatCSTool) classifyMessage(a wechatCSArgs) string {
	msg := strings.TrimSpace(a.Content)
	intent := "general"
	risk := "low"
	suggest := "可正常客服回复"

	switch {
	case containsAny(msg, []string{"退款", "退钱", "投诉", "维权", "差评", "举报"}):
		intent = "complaint"
		risk = "high"
		suggest = "建议优先转人工，由人工客服处理投诉/退款问题"
	case containsAny(msg, []string{"价格", "多少钱", "报价", "费用", "优惠"}):
		intent = "pricing"
		risk = "medium"
		suggest = "可先提供标准价格/优惠说明，敏感报价建议人工确认"
	case containsAny(msg, []string{"下单", "购买", "怎么买", "链接", "付款"}):
		intent = "purchase"
		risk = "medium"
		suggest = "可引导下单流程，涉及支付异常时转人工"
	case containsAny(msg, []string{"多久", "什么时候", "发货", "进度", "安排"}):
		intent = "delivery_progress"
		risk = "low"
		suggest = "可回复标准进度说明或时效说明"
	case containsAny(msg, []string{"图片", "截图", "车牌", "识别", "语音"}):
		intent = "multimodal_request"
		risk = "medium"
		suggest = "需要先获取图片/语音内容，再决定回复或转人工"
	}

	return toJSON(map[string]interface{}{
		"action":          "classify_message",
		"status":          "success",
		"intent":          intent,
		"risk":            risk,
		"suggestion":      suggest,
		"requires_review": risk != "low",
		"message":         "已完成客服消息基础分类。",
	})
}

func (t *WeChatCSTool) enableWatch(ctx context.Context, a wechatCSArgs) (string, error) {
	if t.db == nil {
		return "", fmt.Errorf("wechat_cs enable_watch requires database")
	}
	userID, _ := ctx.Value(CtxKeyUserID).(string)
	if userID == "" {
		return "", fmt.Errorf("enable_watch requires user context")
	}
	if strings.TrimSpace(a.Target) == "" {
		return "", fmt.Errorf("enable_watch requires target")
	}
	if strings.TrimSpace(a.AgentID) == "" {
		return "", fmt.Errorf("enable_watch requires agent_id")
	}
	mode := strings.TrimSpace(a.Mode)
	if mode == "" {
		mode = "suggest_only"
	}
	pollSec := 20
	if strings.TrimSpace(a.PollSec) != "" {
		fmt.Sscanf(strings.TrimSpace(a.PollSec), "%d", &pollSec)
		if pollSec <= 0 {
			pollSec = 20
		}
	}

	var watch model.WeChatWatch
	err := t.db.WithContext(ctx).Where("user_id = ? AND target = ?", userID, a.Target).First(&watch).Error
	if err == nil {
		watch.AgentID = a.AgentID
		watch.WindowTitle = a.WindowTitle
		watch.Mode = mode
		watch.PollIntervalSec = pollSec
		watch.Enabled = true
		watch.LastImageHash = ""
		watch.LastImageURL = ""
		watch.LastTriggeredAt = nil
		if err := t.db.WithContext(ctx).Save(&watch).Error; err != nil {
			return "", fmt.Errorf("更新微信监控失败: %w", err)
		}
	} else {
		if err != gorm.ErrRecordNotFound {
			return "", fmt.Errorf("查询微信监控失败: %w", err)
		}
		watch = model.WeChatWatch{
			UserID:          userID,
			AgentID:         a.AgentID,
			Target:          a.Target,
			WindowTitle:     a.WindowTitle,
			Mode:            mode,
			PollIntervalSec: pollSec,
			Enabled:         true,
		}
		if err := t.db.WithContext(ctx).Create(&watch).Error; err != nil {
			return "", fmt.Errorf("创建微信监控失败: %w", err)
		}
	}

	return toJSON(map[string]interface{}{
		"action":            "enable_watch",
		"status":            "success",
		"watch_id":          watch.ID,
		"target":            watch.Target,
		"agent_id":          watch.AgentID,
		"mode":              watch.Mode,
		"poll_interval_sec": watch.PollIntervalSec,
		"message":           fmt.Sprintf("已启用微信群“%s”的客服监控。", watch.Target),
	}), nil
}

func (t *WeChatCSTool) listWatches(ctx context.Context) (string, error) {
	if t.db == nil {
		return "", fmt.Errorf("wechat_cs list_watches requires database")
	}
	userID, _ := ctx.Value(CtxKeyUserID).(string)
	if userID == "" {
		return "", fmt.Errorf("list_watches requires user context")
	}
	var watches []model.WeChatWatch
	if err := t.db.WithContext(ctx).Where("user_id = ?", userID).Order("created_at DESC").Find(&watches).Error; err != nil {
		return "", fmt.Errorf("查询微信监控列表失败: %w", err)
	}
	return toJSON(map[string]interface{}{
		"action":  "list_watches",
		"status":  "success",
		"watches": watches,
		"message": fmt.Sprintf("共找到 %d 个微信客服监控项。", len(watches)),
	}), nil
}

func (t *WeChatCSTool) disableWatch(ctx context.Context, a wechatCSArgs) (string, error) {
	if t.db == nil {
		return "", fmt.Errorf("wechat_cs disable_watch requires database")
	}
	userID, _ := ctx.Value(CtxKeyUserID).(string)
	if userID == "" {
		return "", fmt.Errorf("disable_watch requires user context")
	}
	watchID := strings.TrimSpace(a.WatchID)
	if watchID == "" {
		return "", fmt.Errorf("disable_watch requires watch_id")
	}
	updates := map[string]interface{}{
		"enabled":    false,
		"updated_at": time.Now(),
	}
	if err := t.db.WithContext(ctx).Model(&model.WeChatWatch{}).Where("id = ? AND user_id = ?", watchID, userID).Updates(updates).Error; err != nil {
		return "", fmt.Errorf("关闭微信监控失败: %w", err)
	}
	return toJSON(map[string]interface{}{
		"action":   "disable_watch",
		"status":   "success",
		"watch_id": watchID,
		"message":  "已关闭该微信客服监控项。",
	}), nil
}

func handoffPriority(reason string) string {
	s := strings.ToLower(reason)
	switch {
	case strings.Contains(reason, "投诉"), strings.Contains(reason, "退款"), strings.Contains(reason, "法律"), strings.Contains(s, "complaint"):
		return "high"
	case strings.Contains(reason, "价格"), strings.Contains(reason, "报价"), strings.Contains(reason, "支付"):
		return "medium"
	default:
		return "normal"
	}
}

func containsAny(s string, words []string) bool {
	for _, w := range words {
		if strings.Contains(s, w) {
			return true
		}
	}
	return false
}

func jsonOrRaw(s string, err error) interface{} {
	if err != nil {
		return map[string]interface{}{"error": err.Error(), "raw": s}
	}
	var v interface{}
	if e := json.Unmarshal([]byte(s), &v); e == nil {
		return v
	}
	return map[string]interface{}{"raw": s}
}
