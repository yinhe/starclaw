package tool

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/yinhe/starclaw/internal/config"
)

// BountyTool allows AI agents to post bounty tasks for humans to complete
type BountyTool struct {
	cfg   config.SwarmConfig
	httpC *http.Client
}

func NewBountyTool(cfg config.SwarmConfig) *BountyTool {
	return &BountyTool{
		cfg:   cfg,
		httpC: &http.Client{Timeout: 15 * time.Second},
	}
}

func (t *BountyTool) Name() string { return "bounty" }

func (t *BountyTool) Description() string {
	return "赏金系统：AI 做不了的事，悬赏让人类来做。可以发布赏金任务、查询状态、确认交付、取消任务。"
}

func (t *BountyTool) Parameters() interface{} {
	return JSONSchema{
		Type: "object",
		Properties: map[string]Property{
			"action": {
				Type:        "string",
				Description: "Action: post_bounty | check_bounty | accept_delivery | cancel_bounty | list_bounties",
				Enum:        []string{"post_bounty", "check_bounty", "accept_delivery", "cancel_bounty", "list_bounties"},
			},
			"title":          {Type: "string", Description: "Bounty title (for post_bounty)"},
			"description":    {Type: "string", Description: "Detailed description of the task (for post_bounty)"},
			"category":       {Type: "string", Description: "Category: data_labeling, content_review, creative_design, real_world, expert_consult, code_review, other"},
			"requirements":   {Type: "string", Description: "What the deliverable must include (for post_bounty)"},
			"reward":         {Type: "string", Description: "Reward amount in CNY (for post_bounty)"},
			"deadline_hours": {Type: "string", Description: "Hours until deadline, 0 = no deadline (for post_bounty)"},
			"bounty_id":      {Type: "string", Description: "Bounty ID (for check/accept/cancel)"},
			"rating":         {Type: "string", Description: "Rating 1-5 for delivery (for accept_delivery)"},
			"feedback":       {Type: "string", Description: "Feedback on delivery (for accept_delivery)"},
		},
		Required: []string{"action"},
	}
}

func (t *BountyTool) Execute(ctx context.Context, args string) (string, error) {
	var params struct {
		Action        string `json:"action"`
		Title         string `json:"title"`
		Description   string `json:"description"`
		Category      string `json:"category"`
		Requirements  string `json:"requirements"`
		Reward        string `json:"reward"`
		DeadlineHours string `json:"deadline_hours"`
		BountyID      string `json:"bounty_id"`
		Rating        string `json:"rating"`
		Feedback      string `json:"feedback"`
	}
	if err := json.Unmarshal([]byte(args), &params); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}

	bountyURL := t.getBountyURL()
	if bountyURL == "" {
		return "赏金系统未配置（需要连接 Queen 服务）。请在 config.yaml 中设置 swarm.queen_url。", nil
	}

	// Extract user/node context
	userID := ""
	if uid, ok := ctx.Value(CtxKeyUserID).(string); ok {
		userID = uid
	}

	switch params.Action {
	case "post_bounty":
		return t.postBounty(bountyURL, userID, params.Title, params.Description, params.Category, params.Requirements, params.Reward, params.DeadlineHours)
	case "check_bounty":
		return t.checkBounty(bountyURL, params.BountyID)
	case "accept_delivery":
		return t.acceptDelivery(bountyURL, params.BountyID, params.Rating, params.Feedback)
	case "cancel_bounty":
		return t.cancelBounty(bountyURL, params.BountyID)
	case "list_bounties":
		return t.listBounties(bountyURL)
	default:
		return "未知操作。支持：post_bounty, check_bounty, accept_delivery, cancel_bounty, list_bounties", nil
	}
}

func (t *BountyTool) postBounty(baseURL, userID, title, desc, category, requirements, reward, deadlineHours string) (string, error) {
	if title == "" || reward == "" {
		return "发布赏金需要 title 和 reward 参数", nil
	}

	body := map[string]interface{}{
		"node_id":      "local",
		"user_id":      userID,
		"title":        title,
		"description":  desc,
		"category":     category,
		"requirements": requirements,
	}

	// Parse reward
	var rewardF float64
	fmt.Sscanf(reward, "%f", &rewardF)
	body["reward"] = rewardF

	if deadlineHours != "" {
		var dh int
		fmt.Sscanf(deadlineHours, "%d", &dh)
		body["deadline_hours"] = dh
	}

	result, err := t.doPost(baseURL+"/bounties", body)
	if err != nil {
		return fmt.Sprintf("发布赏金失败: %v", err), nil
	}

	bounty, _ := result["bounty"].(map[string]interface{})
	if bounty != nil {
		return fmt.Sprintf("赏金任务已发布！\nID: %v\n标题: %v\n奖金: ¥%.2f\n状态: open\n等待人类领取中...", bounty["id"], bounty["title"], rewardF), nil
	}
	return fmt.Sprintf("赏金已发布: %v", result), nil
}

func (t *BountyTool) checkBounty(baseURL, bountyID string) (string, error) {
	if bountyID == "" {
		return "需要 bounty_id 参数", nil
	}

	resp, err := t.httpC.Get(baseURL + "/bounties/" + bountyID)
	if err != nil {
		return fmt.Sprintf("查询失败: %v", err), nil
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	bounty, _ := result["bounty"].(map[string]interface{})
	if bounty == nil {
		return "赏金任务未找到", nil
	}

	status := bounty["status"]
	info := fmt.Sprintf("赏金任务 %s\n标题: %v\n状态: %v\n奖金: ¥%v", bountyID, bounty["title"], status, bounty["reward"])

	if status == "delivered" {
		info += fmt.Sprintf("\n交付说明: %v\n交付链接: %v", bounty["delivery_note"], bounty["delivery_url"])
	}
	if status == "completed" {
		info += fmt.Sprintf("\n评分: %v\n反馈: %v", bounty["rating"], bounty["feedback"])
	}

	return info, nil
}

func (t *BountyTool) acceptDelivery(baseURL, bountyID, rating, feedback string) (string, error) {
	if bountyID == "" {
		return "需要 bounty_id 参数", nil
	}

	body := map[string]interface{}{
		"node_id":  "local",
		"feedback": feedback,
	}
	if rating != "" {
		var r int
		fmt.Sscanf(rating, "%d", &r)
		body["rating"] = r
	}

	result, err := t.doPost(baseURL+"/bounties/"+bountyID+"/accept", body)
	if err != nil {
		return fmt.Sprintf("确认交付失败: %v", err), nil
	}
	msg, _ := result["message"].(string)
	return fmt.Sprintf("交付已确认: %s", msg), nil
}

func (t *BountyTool) cancelBounty(baseURL, bountyID string) (string, error) {
	if bountyID == "" {
		return "需要 bounty_id 参数", nil
	}

	result, err := t.doPost(baseURL+"/bounties/"+bountyID+"/cancel", map[string]interface{}{})
	if err != nil {
		return fmt.Sprintf("取消失败: %v", err), nil
	}
	msg, _ := result["message"].(string)
	return fmt.Sprintf("赏金已取消: %s", msg), nil
}

func (t *BountyTool) listBounties(baseURL string) (string, error) {
	resp, err := t.httpC.Get(baseURL + "/bounties?status=open")
	if err != nil {
		return fmt.Sprintf("查询失败: %v", err), nil
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	bounties, _ := result["bounties"].([]interface{})
	if len(bounties) == 0 {
		return "当前没有开放的赏金任务", nil
	}

	var sb bytes.Buffer
	sb.WriteString(fmt.Sprintf("当前开放赏金任务 (%d 个):\n\n", len(bounties)))
	for i, b := range bounties {
		if i >= 10 {
			sb.WriteString(fmt.Sprintf("... 还有 %d 个\n", len(bounties)-10))
			break
		}
		bm, _ := b.(map[string]interface{})
		sb.WriteString(fmt.Sprintf("%d. [%v] %v — ¥%v\n   %v\n\n", i+1, bm["category"], bm["title"], bm["reward"], bm["description"]))
	}

	return sb.String(), nil
}

func (t *BountyTool) getBountyURL() string {
	if t.cfg.QueenURL != "" {
		return t.cfg.QueenURL
	}
	return ""
}

func (t *BountyTool) doPost(url string, body map[string]interface{}) (map[string]interface{}, error) {
	data, _ := json.Marshal(body)
	resp, err := t.httpC.Post(url, "application/json", bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respData, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(respData, &result)

	if resp.StatusCode >= 400 {
		errMsg, _ := result["error"].(string)
		return nil, fmt.Errorf("%d: %s", resp.StatusCode, errMsg)
	}

	return result, nil
}
