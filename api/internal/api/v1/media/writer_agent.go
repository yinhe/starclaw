package media

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yinhe/starclaw/internal/model"
	"github.com/yinhe/starclaw/internal/provider"
	"gorm.io/gorm"
)

// DramaWriterHandler 提供「编剧 AI Agent」能力：
//   - POST /v1/drama/writer/review  → 多维度评价一集的剧本 md + 提示词 md，
//     输出结构化 JSON（scores/issues/suggestions），供前端 ScriptTab 展示。
//
// 设计理由：
//
//	用户反馈 EP04 播放量断崖（6000→1000），需要系统化审视每集剧情细节。
//	此 Agent 以 EP01-EP04 实战复盘 + 抖音短剧冷启动算法为先验，
//	在生产前给出可执行的修改建议，不替代人工判断，仅做"第二双眼睛"。
type DramaWriterHandler struct {
	db        *gorm.DB
	providers *provider.Registry
}

func NewDramaWriterHandler(db *gorm.DB, providers *provider.Registry) *DramaWriterHandler {
	return &DramaWriterHandler{db: db, providers: providers}
}

// ── Request / Response ──────────────────────────────────────────────

type writerReviewReq struct {
	EpisodeLabel string   `json:"episode_label"` // "EP05 夜袭"
	EpisodeMeta  string   `json:"episode_meta"`  // 简短元数据 JSON/文本
	BibleURL     string   `json:"bible_url"`     // 可选：由后端 fetch（相对 /v1/... 或绝对）
	ScriptURL    string   `json:"script_url"`    // 可选：同上
	PromptsURL   string   `json:"prompts_url"`   // 可选：同上
	BibleMD      string   `json:"bible_md"`      // 直传文本（优先于 URL）
	ScriptMD     string   `json:"script_md"`
	PromptsMD    string   `json:"prompts_md"`
	FocusDims    []string `json:"focus_dims,omitempty"` // 指定只审这些维度（空=全部）
}

// writerReviewResp 是严格 JSON schema，系统 prompt 里强制模型按此输出。
type writerReviewResp struct {
	EpisodeLabel string                  `json:"episode_label"`
	OverallScore float64                 `json:"overall_score"` // 0-100
	Verdict      string                  `json:"verdict"`       // 一句话总评
	Dimensions   []writerDimensionResult `json:"dimensions"`    // 9 个维度
	TopIssues    []writerIssue           `json:"top_issues"`    // 最关键的 3-5 个问题
	Suggestions  []writerSuggestion      `json:"suggestions"`   // 具体修改建议
	RewriteHints []writerRewriteHint     `json:"rewrite_hints"` // 镜级别可直接粘贴的改写
	Model        string                  `json:"model"`
	Provider     string                  `json:"provider"`
	GeneratedAt  string                  `json:"generated_at"`
}

type writerDimensionResult struct {
	Key     string   `json:"key"` // 见 writerDimensions
	Label   string   `json:"label"`
	Score   float64  `json:"score"`   // 0-100
	Comment string   `json:"comment"` // 1-3 句评语
	Good    []string `json:"good"`    // 做得好的点
	Bad     []string `json:"bad"`     // 问题点
}

type writerIssue struct {
	Dimension string `json:"dimension"` // writerDimensions.key
	Severity  string `json:"severity"`  // "high" | "medium" | "low"
	Where     string `json:"where"`     // "S2 第 18-28s" / "全剧结尾"
	Problem   string `json:"problem"`   // 具体问题
	Why       string `json:"why"`       // 为何影响播放
}

type writerSuggestion struct {
	Where    string `json:"where"`
	Action   string `json:"action"` // "增加" | "删除" | "替换" | "调顺序" | "改台词"
	Original string `json:"original,omitempty"`
	Revised  string `json:"revised"`
	Reason   string `json:"reason"`
}

type writerRewriteHint struct {
	SceneID   string `json:"scene_id"` // "S1" ... "S6"
	Field     string `json:"field"`    // "prompt" | "dialogue" | "beat"
	Before    string `json:"before"`
	After     string `json:"after"`
	Rationale string `json:"rationale"`
}

// ── Dimensions (9 维) ───────────────────────────────────────────────

type writerDimensionDef struct { //nolint:unused
	Key    string
	Label  string
	Rubric string // 评分标准
}

var writerDimensions = []writerDimensionDef{ //nolint:unused
	{Key: "hook_3s", Label: "开头 3 秒钩子", Rubric: "抖音冷启动核心。0-3s 是否有视觉冲击/悬念/身份反差/金句勾留，决定完播率上限"},
	{Key: "conflict", Label: "冲突强度", Rubric: "每 8-12s 是否有一次明确冲突升级。平淡段超 8s 直接掉粉"},
	{Key: "pacing", Label: "镜头节奏", Rubric: "场景数/时长比是否合理，切换是否太慢或太碎，单镜 ≤ 12s，最短 ≥ 3s"},
	{Key: "dialogue", Label: "对白口语化/市井感", Rubric: "是否像真人说话，有无书面语/AI 腔，中文对白 ≤ 15 字/句，爆梗 ≥ 1 个"},
	{Key: "emotion_arc", Label: "情感弧线", Rubric: "主角心理有无明确起伏（崩溃→希望→反转），观众共情点是否清晰"},
	{Key: "visual_payoff", Label: "视觉爽点密度", Rubric: "每 10s 至少一个可截图的高光画面（特效/姿态/表情/道具闪现）"},
	{Key: "character_consistency", Label: "角色一致性", Rubric: "人设/语气/服饰是否跨镜一致，[图N] 参考是否覆盖所有出场"},
	{Key: "platform_friendly", Label: "平台友好度", Rubric: "抖音审核：脏话/血腥/暧昧擦边是否处理；画面是否足够干净无文字/水印"},
	{Key: "hook_ending", Label: "结尾钩子", Rubric: "最后 3-5s 是否留悬念/反转/情绪定格，是否促进连刷下一集"},
}

// ── System prompt ───────────────────────────────────────────────────

const writerSystemPrompt = `你是《虫群宇宙》短剧项目的资深编剧顾问，你的工作是**在视频生成前**审阅一集的剧本+提示词，按抖音/TikTok 短剧冷启动算法的先验给出**可执行**的修改意见。

你的决策先验（EP01-EP04 实战复盘）：
1. EP03《闺蜜》6000+ 播 · EP04《新世界》1000 播 → 差距全在"冲突密度"和"开头钩子"
2. 纯温情无冲突 = 必扑（EP04 的教训）
3. 竖屏 720x1280，字幕会遮挡画面底部 1/5，建议不烧对白字幕；Seedance 原生中文 TTS + 唇形同步够用
4. 每 8-12s 必须有一次冲突升级；单镜不超过 12s
5. 开头 3 秒必须有视觉冲击或身份反差或金句，否则观众划走
6. 结尾必须留悬念/反转/情绪定格促进刷下一集
7. 中文对白短句 ≤ 15 字，口语化，至少一个爆梗或金句
8. 市井词不等于粗口；"老天爷啊""大爷的""我的妈呀"等替代"卧槽"更过审
9. 角色一致性靠 [图N] 占位符 + 统一 reference sheet，跨镜服饰要一致
10. 视觉爽点每 10s 至少一个可截图的高光画面

你的任务：针对用户给的一集（bible + 剧本 md + 提示词 md），按 9 个维度逐一打分 0-100，指出最关键的 3-5 个问题，给出具体到镜号/台词级别的修改建议。

**严格输出 JSON（不要任何解释，不要 Markdown，不要代码块，直接 JSON）**，schema 如下：
{
  "episode_label": "<传入的 episode_label>",
  "overall_score": 0-100 的数字,
  "verdict": "一句话总评（30 字内）",
  "dimensions": [
    {"key": "hook_3s", "label": "开头 3 秒钩子", "score": 0-100, "comment": "...", "good": ["..."], "bad": ["..."]},
    ... 共 9 项
  ],
  "top_issues": [
    {"dimension": "<key>", "severity": "high|medium|low", "where": "S2 第 18-28s", "problem": "...", "why": "..."},
    ... 3-5 项
  ],
  "suggestions": [
    {"where": "S1", "action": "增加|删除|替换|调顺序|改台词", "original": "...", "revised": "...", "reason": "..."},
    ... 可 5-10 项
  ],
  "rewrite_hints": [
    {"scene_id": "S1", "field": "prompt|dialogue|beat", "before": "...", "after": "...", "rationale": "..."},
    ... 可 3-8 项
  ]
}

9 个维度必须全部出现，按以下顺序：hook_3s, conflict, pacing, dialogue, emotion_arc, visual_payoff, character_consistency, platform_friendly, hook_ending。`

// ── Handler ──────────────────────────────────────────────────────────

// Review POST /v1/drama/writer/review
func (h *DramaWriterHandler) Review(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	var req writerReviewReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body", "detail": err.Error()})
		return
	}

	// 1) 补齐缺失的文本：若 MD 为空但给了 URL，则服务端 fetch
	if strings.TrimSpace(req.BibleMD) == "" && strings.TrimSpace(req.BibleURL) != "" {
		if t, err := fetchRelativeText(c, req.BibleURL); err == nil {
			req.BibleMD = t
		}
	}
	if strings.TrimSpace(req.ScriptMD) == "" && strings.TrimSpace(req.ScriptURL) != "" {
		if t, err := fetchRelativeText(c, req.ScriptURL); err == nil {
			req.ScriptMD = t
		}
	}
	if strings.TrimSpace(req.PromptsMD) == "" && strings.TrimSpace(req.PromptsURL) != "" {
		if t, err := fetchRelativeText(c, req.PromptsURL); err == nil {
			req.PromptsMD = t
		}
	}
	// 2) 至少要有 script_md 或 prompts_md 才能评
	if strings.TrimSpace(req.ScriptMD) == "" && strings.TrimSpace(req.PromptsMD) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "need at least one of script_md/prompts_md (or *_url)"})
		return
	}

	// 3) 选模型
	cfg, err := h.resolveWriterModel(userID)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return
	}
	p := provider.CreateFromConfig(h.providers, cfg)
	if p == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "LLM provider not available"})
		return
	}

	// 4) 拼 user message（截断到上下文安全长度，各段 ≤ 8000 chars）
	trim := func(s string, n int) string {
		if len(s) <= n {
			return s
		}
		return s[:n] + "\n\n...[truncated]"
	}
	var userBuf strings.Builder
	fmt.Fprintf(&userBuf, "【集名】%s\n", req.EpisodeLabel)
	if strings.TrimSpace(req.EpisodeMeta) != "" {
		fmt.Fprintf(&userBuf, "【元数据】%s\n", trim(req.EpisodeMeta, 1000))
	}
	if strings.TrimSpace(req.BibleMD) != "" {
		fmt.Fprintf(&userBuf, "\n【项目 Bible 节选】\n%s\n", trim(req.BibleMD, 6000))
	}
	if strings.TrimSpace(req.ScriptMD) != "" {
		fmt.Fprintf(&userBuf, "\n【本集剧本 md】\n%s\n", trim(req.ScriptMD, 8000))
	}
	if strings.TrimSpace(req.PromptsMD) != "" {
		fmt.Fprintf(&userBuf, "\n【本集提示词 md】\n%s\n", trim(req.PromptsMD, 6000))
	}
	if len(req.FocusDims) > 0 {
		fmt.Fprintf(&userBuf, "\n【只评这些维度】%s\n", strings.Join(req.FocusDims, ", "))
	}
	fmt.Fprintf(&userBuf, "\n请严格按 JSON schema 输出。")

	// 5) 请求 LLM（大容量、低温度，force JSON）
	ctx, cancel := context.WithTimeout(c.Request.Context(), 90*time.Second)
	defer cancel()
	chunk, err := p.ChatSync(ctx, &provider.ChatRequest{
		Model: cfg.ModelName,
		Messages: []provider.ChatMessage{
			{Role: "system", Content: writerSystemPrompt},
			{Role: "user", Content: userBuf.String()},
		},
	})
	if err != nil {
		log.Printf("[drama_writer] llm err: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	rawText := strings.TrimSpace(chunk.Content)
	rawText = stripCodeFence(rawText)

	// 6) 解析 JSON；若失败则回降级：把原文作为 verdict 返回
	var resp writerReviewResp
	if err := json.Unmarshal([]byte(rawText), &resp); err != nil {
		log.Printf("[drama_writer] json parse failed, raw snippet=%q", rawText[:min(len(rawText), 400)])
		// fallback：把模型原文塞进 verdict
		resp = writerReviewResp{
			EpisodeLabel: req.EpisodeLabel,
			OverallScore: 0,
			Verdict:      "⚠️ 模型未严格按 JSON 返回，下面是原文：\n" + trim(rawText, 800),
		}
	}
	if resp.EpisodeLabel == "" {
		resp.EpisodeLabel = req.EpisodeLabel
	}
	resp.Model = cfg.ModelName
	resp.Provider = cfg.Provider
	resp.GeneratedAt = time.Now().UTC().Format(time.RFC3339)

	c.JSON(http.StatusOK, resp)
}

// ── 推广文案生成 · Douyin + 朋友圈 ─────────────────────────────────────

type promoGenReq struct {
	EpisodeLabel  string   `json:"episode_label"`
	EpisodeMeta   string   `json:"episode_meta"`
	BibleURL      string   `json:"bible_url"`
	ScriptURL     string   `json:"script_url"`
	PromptsURL    string   `json:"prompts_url"`
	BibleMD       string   `json:"bible_md"`
	ScriptMD      string   `json:"script_md"`
	PromptsMD     string   `json:"prompts_md"`
	CoverURL      string   `json:"cover_url,omitempty"`       // 片头封面
	FinalVideoURL string   `json:"final_video_url,omitempty"` // 已合成的正片 URL（可选）
	PickedClips   []string `json:"picked_clips,omitempty"`    // ["S1.t2", ...]
	Platforms     []string `json:"platforms,omitempty"`       // 默认 ["douyin","wechat_moments"]
}

type promoDouyin struct {
	Titles            []string `json:"titles"`              // 3-5 个候选标题
	Body              string   `json:"body"`                // 正文（发布时配文，≤ 200 字）
	Hashtags          []string `json:"hashtags"`            // #话题，按热度降序，6-12 个
	FirstFrameCaption string   `json:"first_frame_caption"` // 前 3s 锁屏钩子文字（6-12 字）
	SeriesTag         string   `json:"series_tag"`          // 系列标签 e.g. "#虫群宇宙第一季"
	PinnedComment     string   `json:"pinned_comment"`      // 作者置顶评论（引导互动）
}

type promoWechatMoments struct {
	CopyShort     string `json:"copy_short"`      // 短版（≤ 30 字，含 emoji）
	CopyMedium    string `json:"copy_medium"`     // 中版（80-120 字）
	CopyLong      string `json:"copy_long"`       // 长版（150-200 字）
	WithFriendTag string `json:"with_friend_tag"` // 朋友/社群带 @ 版本
	ShareHint     string `json:"share_hint"`      // 转发时追加的互动钩子
}

type promoResp struct {
	EpisodeLabel  string             `json:"episode_label"`
	Douyin        promoDouyin        `json:"douyin"`
	WechatMoments promoWechatMoments `json:"wechat_moments"`
	XiaoHongShu   string             `json:"xiaohongshu,omitempty"` // 小红书版（单条 300-500 字，优化 SEO）
	CoreHook      flexString         `json:"core_hook"`             // 本集最核心的抓人点（一句话）
	AudienceVibe  flexString         `json:"audience_vibe"`         // 目标受众情绪定位（"治愈/爽感/好奇"）
	Model         string             `json:"model"`
	Provider      string             `json:"provider"`
	GeneratedAt   string             `json:"generated_at"`
}

// flexString 同时接受 JSON string 与 []string —— 用户反馈：promp system prompt 写的
// "选 1-2 个"，模型经常理解成数组 ["治愈","好奇"]，导致 promoResp.audience_vibe
// Unmarshal 报 "cannot unmarshal array into Go struct field of type string"。
// 这里数组会被 " / " 拼接，单 string 直接落入。
type flexString string

func (f *flexString) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if len(b) == 0 || string(b) == "null" {
		*f = ""
		return nil
	}
	if b[0] == '"' {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		*f = flexString(s)
		return nil
	}
	if b[0] == '[' {
		var arr []string
		if err := json.Unmarshal(b, &arr); err != nil {
			// 尝试 []any，模型可能给 [123, "x"] 之类的混合
			var raw []any
			if err2 := json.Unmarshal(b, &raw); err2 != nil {
				return err
			}
			out := make([]string, 0, len(raw))
			for _, v := range raw {
				out = append(out, fmt.Sprint(v))
			}
			*f = flexString(strings.Join(out, " / "))
			return nil
		}
		*f = flexString(strings.Join(arr, " / "))
		return nil
	}
	// 数字 / 布尔等：直接 fmt.Sprint
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	*f = flexString(fmt.Sprint(v))
	return nil
}

const promoSystemPrompt = `你是《虫群宇宙》短剧项目的抖音冷启动运营操盘手，负责为一集短剧写**全套发布文案**。

核心先验（EP01-EP04 实战复盘）：
1. 抖音冷启动核心 = 前 3 秒钩子 + 标题钩子 + 话题标签三位一体
2. EP03《闺蜜》6000+ 播放靠"穿越者第一次有家的感觉"这种身份反差钩子
3. EP04《新世界》1000 播放因为标题+钩子都太温情，缺爆点
4. 抖音标题最佳字数 10-18 字，带数字/冲突词/反差词效果最好
5. 话题要选 1-2 个大盘（>10亿）+ 3-5 个垂类（>千万）+ 1-2 个 IP 专属
6. 朋友圈是社交破冰，必须"我" 为主语说自己的故事/感受，不能像广告
7. 小红书走 SEO 长尾词，开头要 hook，中段讲故事，结尾 CTA
8. 抖音置顶评论永远是"互动钩子"——问观众选择/立场，驱动评论区热度

**严格输出 JSON（不要任何解释、不要 markdown、不要代码块）**，schema：
{
  "episode_label": "<传入的 episode_label>",
  "douyin": {
    "titles": ["候选标题1 10-18字带钩子", "候选标题2", "候选标题3", "候选标题4"],
    "body": "发布时配文 ≤ 200 字，开头就 hook，中段留悬念，结尾 CTA（追更/关注/连刷）",
    "hashtags": ["#虫群宇宙", "#AI短剧", ... 6-12 个，大盘 > 垂类 > IP 专属],
    "first_frame_caption": "前 3 秒锁屏文字 6-12 字",
    "series_tag": "#虫群宇宙第X季",
    "pinned_comment": "作者置顶评论，驱动互动（问选择/问立场/金句+@朋友）"
  },
  "wechat_moments": {
    "copy_short": "≤ 30 字含 emoji，本集最强金句式钩子",
    "copy_medium": "80-120 字，第一人称，讲自己为啥做这集+本集观感",
    "copy_long": "150-200 字，完整故事感，适合私域老粉",
    "with_friend_tag": "带 @ 朋友/社群的版本，关系感更强",
    "share_hint": "别人转发时追加的一句互动钩子，≤ 20 字"
  },
  "xiaohongshu": "300-500 字的小红书正文，开头必须 3s hook，中段故事/情绪 + 排版用 emoji 分段，结尾 CTA + 话题塞在文末",
  "core_hook": "这一集最抓人的点（一句话 ≤ 30 字）",
  "audience_vibe": "目标受众情绪（治愈 / 爽感 / 好奇 / 热血 / 反转 中选 1-2 个）"
}`

// GeneratePromo POST /v1/drama/writer/promo
func (h *DramaWriterHandler) GeneratePromo(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	var req promoGenReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body", "detail": err.Error()})
		return
	}

	if strings.TrimSpace(req.BibleMD) == "" && strings.TrimSpace(req.BibleURL) != "" {
		if t, err := fetchRelativeText(c, req.BibleURL); err == nil {
			req.BibleMD = t
		}
	}
	if strings.TrimSpace(req.ScriptMD) == "" && strings.TrimSpace(req.ScriptURL) != "" {
		if t, err := fetchRelativeText(c, req.ScriptURL); err == nil {
			req.ScriptMD = t
		}
	}
	if strings.TrimSpace(req.PromptsMD) == "" && strings.TrimSpace(req.PromptsURL) != "" {
		if t, err := fetchRelativeText(c, req.PromptsURL); err == nil {
			req.PromptsMD = t
		}
	}
	if strings.TrimSpace(req.ScriptMD) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "need script_md or script_url"})
		return
	}

	cfg, err := h.resolveWriterModel(userID)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return
	}
	p := provider.CreateFromConfig(h.providers, cfg)
	if p == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "LLM provider not available"})
		return
	}

	trim := func(s string, n int) string {
		if len(s) <= n {
			return s
		}
		return s[:n] + "\n\n...[truncated]"
	}
	var userBuf strings.Builder
	fmt.Fprintf(&userBuf, "【集名】%s\n", req.EpisodeLabel)
	if strings.TrimSpace(req.EpisodeMeta) != "" {
		fmt.Fprintf(&userBuf, "【元数据】%s\n", trim(req.EpisodeMeta, 800))
	}
	if strings.TrimSpace(req.BibleMD) != "" {
		fmt.Fprintf(&userBuf, "\n【项目 Bible 节选】\n%s\n", trim(req.BibleMD, 3000))
	}
	fmt.Fprintf(&userBuf, "\n【本集剧本】\n%s\n", trim(req.ScriptMD, 6000))
	if strings.TrimSpace(req.PromptsMD) != "" {
		fmt.Fprintf(&userBuf, "\n【本集提示词 · 镜头氛围参考】\n%s\n", trim(req.PromptsMD, 3000))
	}
	if req.FinalVideoURL != "" {
		fmt.Fprintf(&userBuf, "\n【正片 URL】%s\n", req.FinalVideoURL)
	}
	if req.CoverURL != "" {
		fmt.Fprintf(&userBuf, "【封面 URL】%s\n", req.CoverURL)
	}
	if len(req.PickedClips) > 0 {
		fmt.Fprintf(&userBuf, "【Picked Clips】%s\n", strings.Join(req.PickedClips, ", "))
	}
	if len(req.Platforms) > 0 {
		fmt.Fprintf(&userBuf, "【只生成这些平台】%s\n", strings.Join(req.Platforms, ", "))
	}
	fmt.Fprintf(&userBuf, "\n请按 JSON schema 严格输出全套发布文案。")

	ctx, cancel := context.WithTimeout(c.Request.Context(), 90*time.Second)
	defer cancel()
	chunk, err := p.ChatSync(ctx, &provider.ChatRequest{
		Model: cfg.ModelName,
		Messages: []provider.ChatMessage{
			{Role: "system", Content: promoSystemPrompt},
			{Role: "user", Content: userBuf.String()},
		},
	})
	if err != nil {
		log.Printf("[drama_promo] llm err: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	rawText := extractJSONObject(strings.TrimSpace(chunk.Content))

	var resp promoResp
	if err := json.Unmarshal([]byte(rawText), &resp); err != nil {
		log.Printf("[drama_promo] json parse failed, raw=%q", rawText[:min(len(rawText), 400)])
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":     "model did not return valid JSON",
			"raw":       trim(rawText, 4000),
			"parse_err": err.Error(),
		})
		return
	}
	if resp.EpisodeLabel == "" {
		resp.EpisodeLabel = req.EpisodeLabel
	}
	resp.Model = cfg.ModelName
	resp.Provider = cfg.Provider
	resp.GeneratedAt = time.Now().UTC().Format(time.RFC3339)

	c.JSON(http.StatusOK, resp)
}

// ── helpers ──────────────────────────────────────────────────────────

// resolveWriterModel 复用 character_studio 的模型选择策略
func (h *DramaWriterHandler) resolveWriterModel(userID string) (model.ModelConfig, error) {
	// import cycle 避免：直接构造同样的查询
	// 实际是复用 character_studio 里同样的 model 表结构
	var cfg model.ModelConfig
	if err := h.db.Where("user_id = ? AND provider = ? AND is_enabled = ?", userID, "star-ai", true).First(&cfg).Error; err == nil {
		return cfg, nil
	}
	if err := h.db.Where("user_id = ? AND is_enabled = ? AND api_key != ''", userID, true).Order("created_at ASC").First(&cfg).Error; err == nil {
		return cfg, nil
	}
	if err := h.db.Where("user_id = ? AND is_enabled = ?", userID, true).Order("created_at ASC").First(&cfg).Error; err == nil {
		return cfg, nil
	}
	if err := h.db.Where("user_id = 'platform' AND is_enabled = ?", true).Order("created_at ASC").First(&cfg).Error; err == nil {
		return cfg, nil
	}
	return cfg, errors.New("请先在「模型管理」中启用至少一个模型")
}

// resolveLocalProjectPath 将 /v1/projects/:project/*filepath 映射到本地
// /app/docs/:project/*filepath（或 DOCS_DIR 环境变量覆盖）；返回失败则回退 HTTP。
// 配合 docker-compose: ../starclaw/docs:/app/docs:ro
func resolveLocalProjectPath(rawURL string) (string, bool) {
	// 兼容前端可能拼出的各种形式：
	//   /v1/projects/swarm-universe/drama/EP05.md
	//   http(s)://host/v1/projects/...
	lower := rawURL
	idx := strings.Index(lower, "/v1/projects/")
	if idx < 0 {
		return "", false
	}
	rest := lower[idx+len("/v1/projects/"):]
	// 去查询串
	if q := strings.IndexByte(rest, '?'); q >= 0 {
		rest = rest[:q]
	}
	if strings.Contains(rest, "..") {
		return "", false
	}
	parts := strings.SplitN(rest, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", false
	}
	docsDir := os.Getenv("DOCS_DIR")
	if docsDir == "" {
		docsDir = "/app/docs"
	}
	full := docsDir + "/" + parts[0] + "/" + parts[1]
	if _, err := os.Stat(full); err != nil {
		return "", false
	}
	return full, true
}

// fetchRelativeText 服务端拉取 /v1/... 相对路径的 md 文本（避免浏览器 CORS/双重请求）
// 关键优化：/v1/projects/:project/*filepath 直接从本地 /app/docs/ 读盘，跳过 HTTP 回环
// （避免容器内 c.Request.Host 解析失败、鉴权丢失、TLS 自签等问题）。
func fetchRelativeText(c *gin.Context, rawURL string) (string, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return "", errors.New("empty url")
	}
	// 优先本地短路：/v1/projects/:project/*filepath → /app/docs/:project/*filepath
	if local, ok := resolveLocalProjectPath(rawURL); ok {
		b, err := os.ReadFile(local)
		if err != nil {
			return "", fmt.Errorf("read local project file %s: %w", local, err)
		}
		if len(b) > (1 << 20) { // 1MB cap
			b = b[:1<<20]
		}
		return string(b), nil
	}
	var full string
	switch {
	case strings.HasPrefix(rawURL, "http://"), strings.HasPrefix(rawURL, "https://"):
		full = rawURL
	case strings.HasPrefix(rawURL, "/"):
		scheme := "http"
		if c.Request.TLS != nil {
			scheme = "https"
		}
		host := c.Request.Host
		full = fmt.Sprintf("%s://%s%s", scheme, host, rawURL)
	default:
		return "", fmt.Errorf("unsupported url scheme: %s", rawURL)
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	httpReq, _ := http.NewRequestWithContext(ctx, "GET", full, nil)
	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("http %d", resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1MB cap
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// stripCodeFence 剥除模型可能套的 ```json ... ``` 外壳
func stripCodeFence(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```") {
		// drop first fence line
		if idx := strings.Index(s, "\n"); idx > 0 {
			s = s[idx+1:]
		}
		s = strings.TrimSpace(s)
		if idx := strings.LastIndex(s, "```"); idx > 0 {
			s = s[:idx]
		}
	}
	return strings.TrimSpace(s)
}

// extractJSONObject 从模型输出里抠出第一个完整、括号平衡的 {…} JSON 对象。
// 用户反馈：promo 文案生成偶发 "model did not return valid JSON"，
// 抓到的 raw 文本要么前后多一段解释性中文，要么 ```json``` 围栏没合上，
// 要么尾部多一句"以上文案仅供参考"——单纯 stripCodeFence 不够。
// 这里走 brace-depth 扫描（忽略字符串里的 { }），保证只取第一对完整的对象字面量。
// 输入若不含 { 直接原样返回，让上层 Unmarshal 自己报错。
func extractJSONObject(s string) string {
	s = stripCodeFence(s)
	start := strings.Index(s, "{")
	if start < 0 {
		return s
	}
	depth := 0
	inStr := false
	escaped := false
	for i := start; i < len(s); i++ {
		ch := s[i]
		if inStr {
			if escaped {
				escaped = false
				continue
			}
			switch ch {
			case '\\':
				escaped = true
			case '"':
				inStr = false
			}
			continue
		}
		switch ch {
		case '"':
			inStr = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return s[start : i+1]
			}
		}
	}
	// 没合上的话回退到原始 stripCodeFence 结果
	return s
}
