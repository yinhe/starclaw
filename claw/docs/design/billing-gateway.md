# Billing Gateway 计费中间层 — 技术设计

## 1. 设计目标

| 目标 | 说明 |
|------|------|
| **开发者零感知** | 第三方 agent/plugin 开发者不写任何计费代码，计费自动发生 |
| **双端同步** | Claw 和 Router 的费用记录一致，支持对账 |
| **分润引擎** | 上游成本 +10% 加价，城市合伙人/团队合伙人/平台/投资人 四方自动分润 |
| **全资源覆盖** | tokens、video、image、music、search、TTS、STT、plugin API 全覆盖 |
| **双模式兼容** | starai:// 走 Router proxy 的和直连 API 的都能计费 |

## 1.1 分润模型

```
用户支付价 = 上游成本 × 1.10  （加价 10%）
利润 = 用户支付价 - 上游成本 = 上游成本 × 0.10

四方分润（投资人始终拿 10%，其余按场景分配）:
```

| 场景 | 城市合伙人 | 团队合伙人 | 平台 | 投资人 |
|------|-----------|-----------|------|--------|
| **Claw 直连** (无合伙人) | - | - | **90%** | **10%** |
| **只有城市合伙人** | **30%** | - | **60%** | **10%** |
| **只有团队合伙人** | - | **70%** | **20%** | **10%** |
| **城市+团队都有** | **30%** | **30%** | **30%** | **10%** |

**投资人池 (Investor Pool)**:
- 初期释放 ¥1000万 空投（类 ICO 首次发币）
- 所有交易利润的 10% 持续注入投资人池
- 投资人按持有份额比例分润

**示例：用户通过 Veo 3.1 生成视频（全链路）**
```
上游成本 (fal.ai):  ¥2.50
用户支付:           ¥2.75  (2.50 × 1.10)
利润:               ¥0.25  (2.75 - 2.50)
├── 城市合伙人:     ¥0.075 (0.25 × 30%)
├── 团队合伙人:     ¥0.075 (0.25 × 30%)
├── 平台:           ¥0.075 (0.25 × 30%)
└── 投资人池:       ¥0.025 (0.25 × 10%)
```

## 1.2 Claw → Queen 关联链路 (分润寻址)

计费时需要知道"这笔费用应该分给哪个城市合伙人和核心合伙人"，寻址链路如下：

```
┌─ Claw ────────────────────────────────────┐
│  claw_id (e.g. "claw:abc123...")          │
│  来自 Ed25519 公钥的 SHA-256 前 40 字节     │
└──────────────┬────────────────────────────┘
               │ NodeBinding (node_id → queen_user_id)
               ↓
┌─ Queen ───────────────────────────────────┐
│  User (user_id)                           │
│  └─ OAuthProvider="claw", OAuthID=claw_id │
└──────────────┬────────────────────────────┘
               │ CityClient (user_id → partner_id)
               │ [注册时通过 ref_code 归属]
               ↓
┌─ CityPartner (城市合伙人) ────────────────┐
│  ID, Name, City, RefCode, CommRate        │
│  ClawID (合伙人自己也是一个 Claw 节点)      │
└──────────────┬────────────────────────────┘
               │ CityPartner.TeamPartnerID (显式外键)
               │ [回退: CityPartner.City 匹配 TeamPartner.Region]
               ↓
┌─ TeamPartner (团队合伙人) ────────────────┐
│  ID, Name, Region, Level, ManageFeeRate   │
│  Level: overlord(领主) / cerebrate(脑虫)   │
│  ClawID (团队合伙人也是一个 Claw 节点)      │
└──────────────┬────────────────────────────┘
               │
               ↓
┌─ Platform (StarClaw) ─────────────────────┐
│  最终结算到 SettlementBill                 │
└───────────────────────────────────────────┘
```

**关键数据模型 (Queen 侧):**

| 模型 | 文件 | 关键字段 | 作用 |
|------|------|----------|------|
| `NodeBinding` | queen/api/internal/model/ | `node_id → queen_user_id` | Claw 节点绑定 Queen 用户 |
| `CityClient` | model/city.go | `user_id → partner_id` | 用户归属城市合伙人 |
| `CityPartner` | model/city.go | `city, ref_code, claw_id` | 城市合伙人 |
| `TeamPartner` | model/partner.go | `region, level, claw_id` | 团队合伙人 (overlord/cerebrate) |
| `Commission` | model/city.go | `partner_id, amount, rate` | 佣金记录 |
| `SettlementBill` | model/settlement.go | `partner_id, partner_type` | 月度结算单 |
| `CreditAccount` | | `claw_id → balance` | 星能余额 (扣费来源) |

**⚠️ 架构缺口：CityPartner → CorePartner 显式关联**

✅ **已解决：** `CityPartner` 新增 `TeamPartnerID` 字段（DB列: `core_partner_id`），在团队合伙人添加城市合伙人时自动设置。

**Queen 内部 API:** `GET /internal/billing/resolve-partners?claw_id=xxx` 返回 `city_partner_id` + `core_partner_id`。

**团队合伙人分级:** 领主 Overlord（普通）/ 脑虫 Cerebrate（核心，最多 5 席，团队投票选举）。

## 2. 架构概览

```
User
  ↓
Claw Chat Handler
  ↓
Agent Runtime (LLM ↔ Tool 循环)
  ↓
┌─────────────────────────────────────────────────────┐
│         Registry.Execute() — 唯一入口               │
│                    ↓                                 │
│  ┌──────────────────────────────────────────────┐   │
│  │         Billing Gateway (新增)                │   │
│  │                                              │   │
│  │  Before:                                     │   │
│  │    1. 查询用户余额 → 余额不足则拒绝          │   │
│  │    2. 查询 Tool 的 ResourceType + PriceHint  │   │
│  │                                              │   │
│  │  Execute:                                    │   │
│  │    3. 调用 Tool.Execute() — 开发者的代码     │   │
│  │                                              │   │
│  │  After:                                      │   │
│  │    4. 计算实际成本 (定价表 + 工具上报)        │   │
│  │    5. 写 ToolUsageRecord                     │   │
│  │    6. 扣费 (本地余额 or Router星能)          │   │
│  │    7. 分润 (如果涉及第三方 agent/plugin)     │   │
│  │    8. 异步同步到 Router (对账)               │   │
│  └──────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────┘
```

## 3. 核心组件

### 3.1 Resource Price Registry (资源定价表)

统一维护所有可计费资源的价格。来源有三：
- **内置定价** — 已知的 fal.ai / DashScope / Minimax 等定价
- **Router 同步** — 从 Router 的 ModelConfig 拉取最新价格
- **Plugin 自声明** — 第三方 plugin 在 spec 中声明价格

```go
// claw/api/internal/billing/price_registry.go

type ResourceType string

const (
    ResTokens         ResourceType = "tokens"
    ResVideoGeneration ResourceType = "video_generation"
    ResImageGeneration ResourceType = "image_generation"
    ResMusicGeneration ResourceType = "music_generation"
    ResAudioTTS        ResourceType = "audio_tts"
    ResAudioSTT        ResourceType = "audio_stt"
    ResWebSearch       ResourceType = "web_search"
    ResBrowser         ResourceType = "browser"
    ResPluginAPI       ResourceType = "plugin_api"
)

type ResourcePrice struct {
    ResourceType  ResourceType
    ToolName      string  // e.g. "video_generation", "image_generation"
    SubType       string  // e.g. model name: "veo3.1", "sora2", "kling-v3"
    PricingModel  string  // "per_call", "per_token", "per_second", "per_char"
    UpstreamCNY   float64 // 上游成本 (元)
    MarkupRate    float64 // 加成比例, e.g. 1.3 = 30% margin
    UserPriceCNY  float64 // 用户价格 = UpstreamCNY * MarkupRate (元)
}

type PriceRegistry struct {
    prices map[string]*ResourcePrice // key = "tool:subtype"
    mu     sync.RWMutex
}

// 内置定价表示例
var defaultPrices = []*ResourcePrice{
    // Video generation (fal.ai)
    {ResVideoGeneration, "video_generation", "veo3",     "per_call", 2.50, 1.3, 3.25},
    {ResVideoGeneration, "video_generation", "veo3.1",   "per_call", 2.50, 1.3, 3.25},
    {ResVideoGeneration, "video_generation", "sora2",    "per_call", 2.00, 1.3, 2.60},
    {ResVideoGeneration, "video_generation", "kling-v3", "per_call", 0.50, 1.3, 0.65},
    {ResVideoGeneration, "video_generation", "luma",     "per_call", 0.80, 1.3, 1.04},
    // Image generation
    {ResImageGeneration, "image_generation", "flux-pro",   "per_call", 0.05, 1.3, 0.065},
    {ResImageGeneration, "image_generation", "flux-kontext","per_call", 0.05, 1.3, 0.065},
    // Music generation
    {ResMusicGeneration, "music_generation", "minimax",  "per_call", 0.10, 1.3, 0.13},
    // Web search
    {ResWebSearch, "web_search", "default", "per_call", 0.005, 1.5, 0.0075},
    // Browser
    {ResBrowser, "browser", "default", "per_call", 0.01, 1.5, 0.015},
    // DashScope video (万相)
    {ResVideoGeneration, "video_generation", "wan2.6-t2v", "per_call", 0.30, 1.3, 0.39},
    {ResVideoGeneration, "video_generation", "wan2.6-i2v", "per_call", 0.30, 1.3, 0.39},
}
```

### 3.2 BillableTool 接口 (可选扩展)

开发者**不需要**实现此接口，但可以实现以提供更精确的计费信息：

```go
// claw/api/internal/tool/tool.go — 新增可选接口

// BillableTool is an OPTIONAL interface that tools can implement
// to provide billing hints. If not implemented, the Gateway uses
// the default price from PriceRegistry based on tool name.
type BillableTool interface {
    // ResourceType returns the resource type for billing (e.g. "video_generation")
    ResourceType() ResourceType

    // EstimateCost returns an estimated cost BEFORE execution (for balance check).
    // Return 0 if unknown — the gateway will use PriceRegistry defaults.
    EstimateCost(args string) float64

    // ReportCost returns the ACTUAL cost AFTER execution.
    // Called with the tool result. Return 0 to use PriceRegistry defaults.
    // This is useful for tools where cost depends on output (e.g. token count).
    ReportCost(args string, result string) float64
}
```

### 3.3 Billing Gateway (计费网关)

拦截所有 `Registry.Execute()` 调用：

```go
// claw/api/internal/billing/gateway.go

type Gateway struct {
    db            *gorm.DB
    prices        *PriceRegistry
    revenueSplit  *RevenueSplitter
    routerSync    *RouterSyncClient
}

// Wrap wraps a tool registry with billing middleware.
// After calling Wrap, all Registry.Execute() calls go through billing.
func (g *Gateway) Wrap(registry *tool.Registry) {
    registry.SetExecuteHook(g.billingHook)
}

func (g *Gateway) billingHook(ctx context.Context, t tool.Tool, name string, args string) (string, error) {
    userID := ctx.Value(tool.CtxKeyUserID).(string)
    convID := ctx.Value(tool.CtxKeyConversationID).(string)

    // ── Before: Check balance ──
    if err := g.checkBalance(ctx, userID); err != nil {
        return "", fmt.Errorf("余额不足，无法执行 %s: %w", name, err)
    }

    // ── Before: Estimate cost (optional) ──
    var estimatedCost float64
    if bt, ok := t.(tool.BillableTool); ok {
        estimatedCost = bt.EstimateCost(args)
    }
    if estimatedCost == 0 {
        estimatedCost = g.prices.GetDefaultCost(name, "")
    }

    // ── Execute ──
    start := time.Now()
    result, err := t.Execute(ctx, args)
    elapsed := time.Since(start)

    // ── After: Calculate actual cost ──
    var actualCost float64
    if bt, ok := t.(tool.BillableTool); ok {
        actualCost = bt.ReportCost(args, result)
    }
    if actualCost == 0 {
        // 从参数中提取 sub_type (e.g. model name)
        subType := extractSubType(name, args)
        actualCost = g.prices.GetUserPrice(name, subType)
    }

    // ── After: Record + Deduct + Split ──
    record := &ToolUsageRecord{
        UserID:         userID,
        ConversationID: convID,
        ToolName:       name,
        SubType:        extractSubType(name, args),
        ResourceType:   g.getResourceType(t, name),
        CostCNY:        actualCost,
        UpstreamCNY:    g.prices.GetUpstreamCost(name, extractSubType(name, args)),
        DurationMs:     elapsed.Milliseconds(),
        Success:        err == nil,
        ErrorMsg:       errMsg(err),
    }

    // 异步处理：扣费 + 分润 + 同步
    go g.settle(ctx, record)

    return result, err
}

func (g *Gateway) settle(ctx context.Context, record *ToolUsageRecord) {
    // 1. 写入本地 usage 记录
    g.db.Create(record)

    // 2. 扣费
    g.deduct(record.UserID, record.CostCNY)

    // 3. 分润（如果该 tool 来自第三方 agent/plugin）
    agentID := ctx.Value(tool.CtxKeyAgentID)
    if agentID != nil {
        g.revenueSplit.Split(record, agentID.(string))
    }

    // 4. 同步到 Router（对账）
    g.routerSync.ReportUsage(record)
}
```

### 3.4 Tool Usage Record (工具使用记录)

```go
// claw/api/internal/model/tool_usage.go — 新增

type ToolUsageRecord struct {
    ID              string    `json:"id" gorm:"type:varchar(36);primaryKey"`
    UserID          string    `json:"user_id" gorm:"type:varchar(36);index;not null"`
    ConversationID  string    `json:"conversation_id" gorm:"type:varchar(36);index"`
    AgentID         string    `json:"agent_id" gorm:"type:varchar(36);index"`   // 哪个 agent 触发的
    ToolName        string    `json:"tool_name" gorm:"type:varchar(50);index;not null"` // video_generation, image_generation...
    SubType         string    `json:"sub_type" gorm:"type:varchar(50)"`         // model: veo3.1, sora2, kling-v3...
    ResourceType    string    `json:"resource_type" gorm:"type:varchar(30);index;not null"`
    CostCNY         float64   `json:"cost_cny" gorm:"type:decimal(10,6);default:0"`     // 用户支付价格 (元)
    UpstreamCNY     float64   `json:"upstream_cny" gorm:"type:decimal(10,6);default:0"` // 上游成本 (元)
    MarginCNY       float64   `json:"margin_cny" gorm:"type:decimal(10,6);default:0"`   // 毛利 = Cost - Upstream
    CityPartnerID   string    `json:"city_partner_id" gorm:"type:varchar(36);index"`     // 城市合伙人 ID
    CorePartnerID   string    `json:"core_partner_id" gorm:"type:varchar(36);index"`     // 核心合伙人 ID
    CityShare       float64   `json:"city_share" gorm:"type:decimal(10,6);default:0"`    // 城市合伙人分成 (40%)
    CoreShare       float64   `json:"core_share" gorm:"type:decimal(10,6);default:0"`    // 核心合伙人分成 (30%)
    PlatformShare   float64   `json:"platform_share" gorm:"type:decimal(10,6);default:0"`// 平台分成
    InvestorShare   float64   `json:"investor_share" gorm:"type:decimal(10,6);default:0"`// 投资人池分成 (10%)
    DurationMs      int64     `json:"duration_ms" gorm:"default:0"`
    Success         bool      `json:"success" gorm:"default:true"`
    ErrorMsg        string    `json:"error_msg,omitempty" gorm:"type:text"`
    SyncedToRouter  bool      `json:"synced_to_router" gorm:"default:false;index"`
    CreatedAt       time.Time `json:"created_at" gorm:"index"`
}
```

### 3.5 Revenue Splitter (分润引擎)

```go
// claw/api/internal/billing/revenue_split.go

// 分润比例 — 基于利润 (MarginCNY = UpstreamCNY × 10%)
// 投资人始终拿 10%，其余按场景分配
const (
    MarkupRate        = 0.10 // 加价 10%
    InvestorShareRate = 0.10 // 投资人池始终 10%
    // 场景分润 (剩余 90% 的分配):
    // Full chain:  City 30% + Core 30% + Platform 30%
    // City only:   City 30% + Platform 60%
    // Core only:   Core 70% + Platform 20%
    // Direct:      Platform 90%
)

type RevenueSplitter struct {
    db       *gorm.DB
    queenAPI *QueenAPIClient // 查询 Queen 的合伙人关联关系
}

// ResolvePartners 根据 claw_id 查询 Queen 的合侙人链路
// 返回 cityPartnerID, corePartnerID
// 调用 Queen API: GET /internal/billing/resolve-partners?claw_id=xxx
func (s *RevenueSplitter) ResolvePartners(clawID string) (cityID, coreID string) {
    // Queen 侧实现:
    // 1. NodeBinding: claw_id → user_id
    // 2. CityClient: user_id → partner_id (CityPartner)
    // 3. CityPartner.core_partner_id → CorePartner (新增字段)
    //    或回退: CityPartner.City 匹配 CorePartner.Region
    resp := s.queenAPI.Get("/internal/billing/resolve-partners?claw_id=" + clawID)
    return resp.CityPartnerID, resp.CorePartnerID
}

func (s *RevenueSplitter) Split(record *ToolUsageRecord, clawID string) {
    // 计算利润
    margin := record.UpstreamCNY * MarkupRate
    record.MarginCNY = margin
    record.CostCNY = record.UpstreamCNY + margin

    // 投资人始终拿 10%
    record.InvestorShare = margin * InvestorShareRate
    remaining := margin - record.InvestorShare // 90% of margin

    // 寻址: 找到该用户对应的城市合伙人和核心合伙人
    cityID, coreID := s.ResolvePartners(clawID)
    record.CityPartnerID = cityID
    record.CorePartnerID = coreID

    if cityID != "" && coreID != "" {
        // 城市+核心: 30/30/30/10
        record.CityShare = margin * 0.30
        record.CoreShare = margin * 0.30
        record.PlatformShare = margin * 0.30
    } else if cityID != "" {
        // 只有城市合伙人: 30/0/60/10
        record.CityShare = margin * 0.30
        record.PlatformShare = margin * 0.60
    } else if coreID != "" {
        // 只有核心合伙人: 0/70/20/10
        record.CoreShare = margin * 0.70
        record.PlatformShare = margin * 0.20
    } else {
        // 直连无合伙人: 0/0/90/10
        record.PlatformShare = remaining
    }

    // 写入 Commission 记录 (城市合伙人)
    if record.CityPartnerID != "" && record.CityShare > 0 {
        s.db.Create(&model.Commission{
            ID: uuid.New().String(), PartnerID: record.CityPartnerID,
            Type: "tool_usage", Amount: int64(record.CityShare * 100),
            Rate: CityShareRate, BaseAmount: int64(record.CostCNY * 100),
            Month: time.Now().Format("2006-01"), Status: "pending",
        })
    }

    // 写入 PartnerCommission 记录 (核心合伙人)
    if record.CorePartnerID != "" && record.CoreShare > 0 {
        s.db.Create(&model.PartnerCommission{
            ID: uuid.New().String(), PartnerID: record.CorePartnerID,
            Type: "tool_usage", Amount: int64(record.CoreShare * 100),
            Rate: CoreShareRate, BaseAmount: int64(record.CostCNY * 100),
            Month: time.Now().Format("2006-01"), Status: "pending",
        })
    }
}
```

### 3.6 Router Sync (双端对账)

```go
// claw/api/internal/billing/router_sync.go

type RouterSyncClient struct {
    client  *http.Client
    baseURL string   // StarAI Router API
}

// ReportUsage 将 Claw 端的工具使用记录同步到 Router
// Router 侧新增 POST /v1/internal/billing/report 接口接收
func (c *RouterSyncClient) ReportUsage(record *ToolUsageRecord) error {
    body, _ := json.Marshal(record)
    resp, err := c.client.Post(c.baseURL+"/v1/internal/billing/report", "application/json", bytes.NewReader(body))
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    if resp.StatusCode == 200 {
        // 标记已同步
        record.SyncedToRouter = true
    }
    return nil
}

// SyncPending 定期同步未同步的记录 (重试机制)
func (c *RouterSyncClient) SyncPending(db *gorm.DB) {
    var records []ToolUsageRecord
    db.Where("synced_to_router = false AND created_at > ?", time.Now().Add(-24*time.Hour)).
        Limit(100).Find(&records)
    for _, r := range records {
        if err := c.ReportUsage(&r); err == nil {
            db.Model(&r).Update("synced_to_router", true)
        }
    }
}
```

## 4. 开发者体验

### 4.1 内置工具 — 零改动

现有的 `video_tool.go`, `image_tool.go`, `music_tool.go` 等**不需要任何修改**。
Gateway 通过 tool name 自动匹配 PriceRegistry 中的定价。

### 4.2 第三方 Plugin — 零计费代码

```json
{
  "name": "weather_lookup",
  "description": "查询天气",
  "endpoint": {"url": "https://api.weather.com/v1/...", "method": "GET"},
  "pricing": {
    "resource_type": "plugin_api",
    "price_per_call_cny": 0.005
  }
}
```

Plugin spec 中新增可选的 `pricing` 字段。如果不填，默认按 `plugin_api` 通用价计费。

### 4.3 高级工具 — 可选精确计费

```go
// 工具开发者可以实现 BillableTool 接口提供精确成本
func (t *VideoTool) ResourceType() ResourceType { return ResVideoGeneration }

func (t *VideoTool) ReportCost(args, result string) float64 {
    var a videoArgs
    json.Unmarshal([]byte(args), &a)
    // 不同模型不同价格
    switch a.Model {
    case "veo3.1": return 2.50
    case "sora2":  return 2.00
    case "kling-v3": return 0.50
    default: return 0
    }
}
```

## 5. 数据流全景

```
┌─ Claw ─────────────────────────────────────────────────────────┐
│                                                                 │
│  User → Chat → Agent Runtime → Registry.Execute()               │
│                                     │                           │
│                        ┌────────────┤ billingHook              │
│                        │  Gateway   │                           │
│                        │            ├─→ checkBalance()          │
│                        │            ├─→ Tool.Execute()          │
│                        │            ├─→ PriceRegistry.lookup()  │
│                        │            ├─→ ToolUsageRecord.create()│
│                        │            ├─→ deduct(user)            │
│                        │            ├─→ RevenueSplitter.split() │
│                        │            └─→ RouterSync.report()     │
│                        └────────────┘                           │
└────────────────────────────┬────────────────────────────────────┘
                             │ POST /v1/internal/billing/report
                             ↓
┌─ Router ───────────────────────────────────────────────────────┐
│                                                                 │
│  /v1/internal/billing/report → 写入 UsageRecord (对账)          │
│                                                                 │
│  /v1/proxy/:provider/*path  → ProviderProxyHandler              │
│       │                          └─→ Meter.CalculateCost()  ←─ │
│       │                              (Router 侧也独立计费)      │
│       ↓                                                        │
│  对账逻辑: Claw 上报 ≈ Router 计量 → 差异报警                  │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

## 6. 分润流示例

用户用第三方开发者的"影视 Agent"生成一个 Veo 3.1 视频：

```
用户支付: ¥3.25
├── 上游成本 (fal.ai): ¥2.50
├── 毛利: ¥0.75
│   ├── 创作者 (影视 Agent 开发者): ¥0.525 (70%)
│   ├── 平台 (StarClaw):            ¥0.1875 (25%)
│   └── 推荐人:                      ¥0.0375 (5%)
│
└── 记录:
    ├── Claw: ToolUsageRecord (含分润明细)
    ├── Router: UsageRecord (对账)
    └── CreatorRevenue: pending → settled → paid_out
```

## 7. 实现优先级

| 阶段 | 内容 | 工时 |
|------|------|------|
| **P0** | `Registry.SetExecuteHook` + `Gateway.billingHook` 基础框架 | 1天 |
| **P0** | `PriceRegistry` 内置定价表 (video/image/music/search) | 0.5天 |
| **P0** | `ToolUsageRecord` 模型 + 写入 | 0.5天 |
| **P1** | 本地扣费逻辑 (Claw 余额) | 0.5天 |
| **P1** | `RevenueSplitter` 分润引擎 | 1天 |
| **P1** | `RouterSync` 双端同步 + Router 接收接口 | 1天 |
| **P2** | Router `ProviderProxyHandler` 加计量 | 0.5天 |
| **P2** | Plugin spec 的 `pricing` 字段支持 | 0.5天 |
| **P2** | 对账报表 + 差异报警 | 1天 |
| **P3** | 创作者后台 (收益查看/提现) | 2天 |

## 8. Registry 改造要点

核心改动只有一处 — `Registry.Execute()` 加 hook：

```go
// tool.go 改造

type ExecuteHook func(ctx context.Context, t Tool, name string, args string) (string, error)

type Registry struct {
    tools map[string]Tool
    hook  ExecuteHook // nil = no billing (backward compatible)
}

func (r *Registry) SetExecuteHook(hook ExecuteHook) {
    r.hook = hook
}

func (r *Registry) Execute(ctx context.Context, name string, args string) (string, error) {
    t, ok := r.tools[name]
    if !ok {
        return "", fmt.Errorf("tool not found: %s", name)
    }
    // If billing gateway is attached, route through it
    if r.hook != nil {
        return r.hook(ctx, t, name, args)
    }
    // No billing — direct execution (self-hosted mode)
    return t.Execute(ctx, args)
}
```

这样：
- **现有代码完全不用改** — 没设 hook 就走原来的路径
- **开发者不感知** — 他们只实现 `Tool` 接口，计费自动发生
- **可选增强** — 实现 `BillableTool` 接口可以提供更精确的成本
