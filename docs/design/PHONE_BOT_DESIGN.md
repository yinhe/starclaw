# Cicada 🪰 蝉 — 电话机器人智能体架构设计文档

> **版本**: v1.0  
> **日期**: 2026-04-01  
> **状态**: 设计中  
> **作者**: StarClaw Team

---

## 目录

1. [产品定位](#1-产品定位)
2. [系统架构](#2-系统架构)
3. [核心模块](#3-核心模块)
4. [Agent 六元组设计](#4-agent-六元组设计)
5. [意向分类引擎](#5-意向分类引擎)
6. [语音管道](#6-语音管道)
7. [电话管道](#7-电话管道)
8. [CRM 数据模型](#8-crm-数据模型)
9. [外呼调度引擎](#9-外呼调度引擎)
10. [合规与安全](#10-合规与安全)
11. [商业模式](#11-商业模式)
12. [部署架构](#12-部署架构)
13. [实现路径](#13-实现路径)
14. [API 设计](#14-api-设计)

---

## 1. 产品定位

### 1.1 什么是电话机器人

电话机器人是一种 AI 外呼智能体，替代人工坐席完成批量外呼任务：

- **日呼量**: 800-1000 通/天（单个机器人）
- **核心能力**: 自动拨号 → 语音对话 → 意向分类 → 录音保存
- **交付形式**: 云端 SaaS，通过 Claw 节点后台登录管理，无需安装任何软件
- **数据安全**: 所有客户数据加密存储在用户自己的 Claw 节点，信息不外泄

### 1.2 目标行业

| 行业 | 典型场景 | 意向判定关键词 |
|------|----------|----------------|
| **房产** | 新盘推广、二手房匹配 | 位置、户型、分期、学区、价格 |
| **教育** | 课程推广、招生 | 年龄、科目、时间、费用、试听 |
| **金融** | 贷款、保险、理财 | 额度、利率、期限、资质 |
| **装修** | 新房装修获客 | 面积、风格、预算、时间 |
| **招商** | 加盟招商 | 投资额、区域、品牌、回本周期 |
| **医美** | 项目咨询、活动推广 | 项目、价格、效果、预约 |

### 1.3 与 Q8bot 的对比

| 维度 | Q8bot 量化智能体 | Cicada 蝉·电话机器人 |
|------|------------------|---------------------|
| **领域** | 金融量化交易 | 电话外呼营销 |
| **基础设施** | QMT 交易终端 | SIP 中继线 + ASR/TTS |
| **AI 核心** | 四维打分 + AI 二次确认 | 话术引擎 + 意向分类 |
| **数据** | 行情数据 + 持仓 | 客户号码 + 通话录音 |
| **收入模式** | 盈利分成 | 月度订阅 |
| **代码位置** | `extractor/` | `cicada/` (新顶层模块) |

---

## 2. 系统架构

### 2.1 整体架构图

```
┌─────────────────────────────────────────────────────────┐
│                    Claw 节点 (用户本地)                     │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌────────┐  │
│  │ Cicada  │  │  话术     │  │  CRM     │  │ 录音   │  │
│  │  Agent   │  │  引擎     │  │  管理     │  │ 存储   │  │
│  └────┬─────┘  └────┬─────┘  └────┬─────┘  └───┬────┘  │
│       │              │              │             │       │
│  ┌────┴──────────────┴──────────────┴─────────────┴───┐  │
│  │              Cicada Bridge (Python :8099)         │  │
│  └────────────────────────┬───────────────────────────┘  │
└───────────────────────────┼──────────────────────────────┘
                            │ HTTPS
          ┌─────────────────┼─────────────────┐
          ▼                 ▼                  ▼
  ┌──────────────┐ ┌──────────────┐  ┌──────────────┐
  │  云通信平台   │ │  DashScope   │  │   StarAI     │
  │  (容联云等)   │ │  ASR + TTS   │  │   LLM 算力    │
  │              │ │              │  │  (qwen-turbo) │
  │  SIP中继线    │ │  Paraformer  │  │              │
  │  号码池      │ │  CosyVoice   │  │              │
  └──────────────┘ └──────────────┘  └──────────────┘
```

### 2.2 通话时序

```
客户电话                 云通信平台              PhoneBot Bridge              LLM
    │                      │                        │                        │
    │  ← SIP INVITE ──────│                        │                        │
    │  ── 200 OK ─────────│                        │                        │
    │                      │                        │                        │
    │  ── 语音流 ─────────│── WebSocket ──────────│                        │
    │                      │   (实时音频)           │                        │
    │                      │                        │── ASR(流式) ──────────│
    │                      │                        │   "你好，请问..."      │
    │                      │                        │                        │
    │                      │                        │── 文字 ───────────────│
    │                      │                        │                        │── 生成回复
    │                      │                        │←─ 回复文字 ────────────│
    │                      │                        │                        │
    │                      │                        │── TTS(流式) ─────────│
    │                      │←─ 音频流 ──────────────│                        │
    │  ← 语音回复 ─────────│                        │                        │
    │                      │                        │                        │
    │  ... (多轮对话) ...   │                        │── 意向分类 ────────────│
    │                      │                        │   (通话结束后)          │
    │  ── BYE ────────────│                        │                        │
    │                      │── 录音URL ────────────│                        │
    │                      │                        │── 保存录音+分类 ───────│
```

### 2.3 延迟预算

端到端目标: **< 1.2 秒**（用户说完到机器人开始回复）

| 环节 | 目标延迟 | 技术选型 |
|------|----------|----------|
| 音频传输 | < 50ms | WebSocket 直连 |
| ASR 识别 | < 300ms | Paraformer 流式（边说边识别） |
| LLM 推理 | < 500ms | qwen-turbo（首token延迟） |
| TTS 合成 | < 200ms | CosyVoice 流式（边生成边播放） |
| 网络开销 | < 150ms | 阿里云内网（ASR/LLM/TTS 同可用区） |
| **合计** | **< 1.2s** | |

关键优化：**流式串联** — ASR 出第一句就送 LLM，LLM 出第一个字就送 TTS，TTS 出第一帧就推音频流。

---

## 3. 核心模块

### 3.1 模块划分

```
cicada/                            # 新顶层模块 🪰 蝉（类似 extractor/）
├── bridge/                        # Python Bridge (核心引擎)
│   ├── main.py                    # FastAPI 服务 (:8099)
│   ├── config.yaml                # 配置文件
│   ├── call_engine.py             # 外呼引擎（调度+状态机）
│   ├── voice_pipeline.py          # 语音管道（ASR+TTS 流式串联）
│   ├── sip_client.py              # SIP/云通信平台对接
│   ├── intent_classifier.py       # 意向分类引擎（A-F）
│   ├── script_engine.py           # 话术引擎（行业模板+动态生成）
│   ├── crm_manager.py             # 客户管理（分类+跟进+录音）
│   ├── scheduler.py               # 定时外呼调度器
│   ├── recorder.py                # 录音下载+存储+转写
│   ├── compliance.py              # 合规检查（退订/黑名单/频控）
│   └── requirements.txt
├── api/                           # Go API (可选，轻量)
│   ├── cmd/server/main.go
│   ├── internal/
│   │   ├── model/                 # 数据模型
│   │   │   ├── call.go            # CallRecord, CallTask
│   │   │   ├── customer.go        # Customer, CustomerTag
│   │   │   ├── script.go          # Script, ScriptNode
│   │   │   └── campaign.go        # Campaign, CampaignStats
│   │   └── handler/
│   │       ├── campaign.go        # 外呼任务管理
│   │       ├── customer.go        # 客户CRM
│   │       ├── analytics.go       # 数据统计
│   │       └── router.go          # 路由注册
│   ├── Dockerfile
│   └── go.mod
├── web/                           # React 前端（外呼管理面板）
│   ├── src/
│   │   ├── pages/
│   │   │   ├── DashboardPage.tsx  # 数据大盘
│   │   │   ├── CampaignPage.tsx   # 外呼任务管理
│   │   │   ├── CustomerPage.tsx   # 客户CRM看板
│   │   │   ├── RecordPage.tsx     # 录音回放+文字
│   │   │   ├── ScriptPage.tsx     # 话术管理
│   │   │   └── SettingsPage.tsx   # 配置（号码/时段/话术）
│   │   └── ...
│   └── ...
├── docs/
│   └── DESIGN.md                  # → 本文档
├── scripts/                       # 话术模板库
│   ├── real_estate.yaml           # 房产话术
│   ├── education.yaml             # 教育话术
│   ├── finance.yaml               # 金融话术
│   └── general.yaml               # 通用话术
├── docker-compose.yml
└── README.md
```

### 3.2 技术栈

| 层 | 技术 | 理由 |
|---|---|---|
| **Bridge** | Python + FastAPI | 语音库生态好（WebSocket、音频处理） |
| **API** | Go + SQLite | 与 StarClaw 其他模块一致 |
| **前端** | React + Vite + TailwindCSS | 与 Claw/Queen 一致 |
| **ASR** | 阿里 Paraformer (DashScope) | 中文最优、流式、低延迟 |
| **TTS** | 阿里 CosyVoice (DashScope) | 多音色、流式、自然度高 |
| **LLM** | qwen-turbo (via StarAI) | 低延迟、中文好、成本低 |
| **电话** | 容联云 / 阿里云通信 | SIP中继、号码池、防封号 |

---

## 4. Agent 六元组设计

Cicada 作为 Claw Agent 模板发布到 Queen 市场，虫族代号：**蝉 (Cicada)** — 以持续不断的鸣叫闻名，完美对应电话外呼的不间断呼叫特性。

### 4.1 基因 (Gene)

```yaml
name: "Cicada 蝉·电话机器人"
model: qwen-turbo          # 低延迟优先
temperature: 0.3           # 话术场景需要稳定输出
max_tokens: 200            # 电话回复短小精悍
system_prompt: |
  你是一个专业的电话外呼机器人。你的目标是通过自然、友好的对话识别客户意向。
  
  核心规则：
  1. 每句话不超过30字，语速适中，像真人一样自然
  2. 不主动透露自己是机器人，但被问到时如实回答
  3. 客户说"不需要"/"别打了"时，礼貌结束通话
  4. 根据客户提问深度判断意向等级(A-F)
  5. 始终引导客户说出需求关键词
  6. 禁止承诺任何不确定的信息
  
  当前行业: {industry}
  话术模板: {script_template}
  客户信息: {customer_info}
```

### 4.2 技能 (Skill)

| 技能 | 描述 | 类型 |
|------|------|------|
| `phone_call` | 发起外呼、挂断、转接人工 | plugin |
| `phone_listen` | 实时语音识别（流式ASR） | plugin |
| `phone_speak` | 文字转语音回复（流式TTS） | plugin |
| `crm_query` | 查询客户信息/历史通话 | plugin |
| `crm_update` | 更新客户分类/标签/备注 | plugin |
| `crm_schedule` | 设置跟进提醒 | plugin |
| `record_save` | 保存通话录音+文字 | plugin |
| `sms_send` | 发送短信（通话后发资料） | plugin |

### 4.3 本能 (Instinct)

| 本能 | 触发 | 行为 |
|------|------|------|
| `auto_dial` | cron: `0 9,14 * * 1-6` | 在设定时段自动开始外呼任务 |
| `call_classify` | event: 通话结束 | 分析通话全文，自动分类 A-F |
| `daily_report` | cron: `30 18 * * *` | 汇总今日外呼数据，生成日报 |
| `followup_remind` | cron: `0 9 * * *` | 检查今日需跟进的客户，发提醒 |
| `script_optimize` | cron: `0 22 * * 0` | 每周分析通话数据，优化话术 |
| `blacklist_sync` | event: 客户拒绝 | 立即加入黑名单，72小时同步 |

### 4.4 外接 (MCP)

| MCP 服务 | 用途 |
|----------|------|
| `cicada-bridge` | 蝉 Bridge（ASR/TTS/SIP 管道） |
| `dashscope` | 阿里云 DashScope（语音服务直连） |

### 4.5 工作流 (Workflow)

**标准外呼工作流**:
```
导入号码 → 去重+合规检查 → 创建外呼任务 → 自动拨号 → 语音对话 
→ 意向分类 → 保存录音 → 更新CRM → A/B类客户通知业务员 → 日报
```

**跟进工作流**:
```
检查跟进队列 → 调取历史通话 → 生成个性化话术 → 拨号 → 对话 → 更新状态
```

### 4.6 记忆 (Memory)

| 记忆类型 | 内容 | 作用域 |
|----------|------|--------|
| `fact` | 客户基本信息（姓名/需求/预算） | global |
| `preference` | 行业话术偏好 | agent |
| `instruct` | 管理员设定的话术规则 | global |
| `context` | 单次通话上下文 | agent |
| `summary` | 每日外呼总结 | agent |

---

## 5. 意向分类引擎

### 5.1 六级分类标准

```python
INTENT_LEVELS = {
    "A": {
        "name": "强意向",
        "criteria": "主动询问价格/付款/优惠/预约，≥3个深度问题",
        "action": "立即通知业务员，24小时内人工跟进",
        "color": "#FF4D4F",  # 红色（最高优先级）
        "score_range": [80, 100],
    },
    "B": {
        "name": "较强意向",
        "criteria": "询问具体细节(位置/配置/时间)，≥2个问题",
        "action": "48小时内人工跟进，发送资料短信",
        "color": "#FF7A45",  # 橙色
        "score_range": [60, 79],
    },
    "C": {
        "name": "一般意向",
        "criteria": "有兴趣但不深入，1个问题或'发个资料看看'",
        "action": "加入培育队列，3天后二次外呼",
        "color": "#FFC53D",  # 黄色
        "score_range": [40, 59],
    },
    "D": {
        "name": "弱意向",
        "criteria": "未拒绝但无实质问题，'嗯''好的'为主",
        "action": "加入低优队列，7天后二次外呼",
        "color": "#73D13D",  # 绿色
        "score_range": [20, 39],
    },
    "E": {
        "name": "明确拒绝",
        "criteria": "'不需要''别打了''我在开会'",
        "action": "标记拒绝，30天内不再外呼",
        "color": "#597EF7",  # 蓝色
        "score_range": [1, 19],
    },
    "F": {
        "name": "无效号码",
        "criteria": "空号/停机/无人接听3次/忙音",
        "action": "移入无效池，不再外呼",
        "color": "#8C8C8C",  # 灰色
        "score_range": [0, 0],
    },
}
```

### 5.2 分类算法

通话结束后，将完整对话文本送入 LLM 进行分类：

```python
CLASSIFY_PROMPT = """
分析以下电话通话记录，判断客户意向等级。

行业: {industry}
通话时长: {duration}秒
通话文字:
{transcript}

请按以下维度打分（每项0-100）：
1. 需求明确度：客户是否清楚表达了需求
2. 问题深度：客户提问的专业度和细节程度
3. 时间意愿：客户是否有明确的时间计划
4. 互动积极性：客户的回应频率和态度
5. 购买信号：是否出现价格/付款/预约等购买信号

输出 JSON：
{
  "scores": {"need": 0, "depth": 0, "timeline": 0, "engagement": 0, "buying_signal": 0},
  "total_score": 0,
  "level": "A/B/C/D/E/F",
  "key_interests": ["关键词1", "关键词2"],
  "summary": "一句话总结客户情况",
  "next_action": "建议的下一步动作"
}
"""
```

### 5.3 实时分类（通话中）

通话进行时，每收到一段 ASR 文字就增量评估，实时更新意向等级：

```
[00:05] 客户: "嗯好的"                    → 预判 D
[00:15] 客户: "在什么位置啊"               → 升为 C
[00:25] 客户: "有多大面积的"               → 升为 B
[00:40] 客户: "首付多少？能分期吗"          → 升为 A
```

运营者可在管理面板实时看到每通电话的意向变化。

---

## 6. 语音管道

### 6.1 ASR (语音识别)

**首选**: 阿里 Paraformer-v2 实时流式识别

```python
# DashScope 实时 ASR 配置
ASR_CONFIG = {
    "model": "paraformer-realtime-v2",
    "format": "pcm",                    # 原始音频格式
    "sample_rate": 16000,               # 16kHz
    "enable_punctuation": True,         # 自动加标点
    "enable_inverse_text_normalization": True,  # 口语→书面
    "enable_disfluency_detection": True,       # 去除"嗯""啊"
    "language_hints": ["zh", "en"],     # 中英混合
}
```

**关键能力**:
- **流式识别**: 边说边出文字，不用等说完
- **VAD（语音端点检测）**: 自动判断客户说完了没
- **热词**: 可注入行业术语提升识别率（如楼盘名、产品名）
- **方言**: 支持粤语、四川话等主要方言

### 6.2 TTS (语音合成)

**首选**: 阿里 CosyVoice 流式合成

```python
# DashScope TTS 配置
TTS_CONFIG = {
    "model": "cosyvoice-v1",
    "voice": "longxiaochun",            # 女声-亲切型（外呼推荐）
    "format": "pcm",
    "sample_rate": 16000,
    "speed": 1.0,                       # 语速 (0.5-2.0)
    "volume": 50,                       # 音量 (0-100)
    "pitch": 0,                         # 音调 (-500~500)
    "enable_subtitle": True,            # 返回时间戳（用于字幕同步）
}
```

**音色选择（按行业推荐）**:

| 行业 | 推荐音色 | 风格 |
|------|----------|------|
| 房产 | longxiaochun (女) | 亲切、专业 |
| 金融 | longshu (男) | 稳重、可信 |
| 教育 | longxiaoxia (女) | 温柔、耐心 |
| 通用 | longxiaochun (女) | 亲切、自然 |

### 6.3 流式串联

```python
async def voice_pipeline(audio_stream, llm_client, tts_client):
    """ASR → LLM → TTS 流式串联，最小化端到端延迟"""
    
    # 1. ASR 流式识别
    async for partial_text in asr_stream(audio_stream):
        if not is_sentence_complete(partial_text):
            continue  # 等待完整句子
        
        # 2. LLM 流式生成
        response_chunks = []
        async for chunk in llm_client.stream_chat(partial_text):
            response_chunks.append(chunk)
            
            # 3. 每收到一个完整短句就送 TTS
            sentence = extract_sentence(response_chunks)
            if sentence:
                async for audio_frame in tts_stream(sentence):
                    yield audio_frame  # 立即推送音频帧到电话
```

---

## 7. 电话管道

### 7.1 云通信平台对接

**首选**: 容联云（国内最成熟的 SIP 中继服务商）

```python
# 容联云 API 配置
SIP_CONFIG = {
    "provider": "cloopen",              # 容联云
    "account_sid": "{CLOOPEN_SID}",
    "auth_token": "{CLOOPEN_TOKEN}",
    "app_id": "{CLOOPEN_APP_ID}",
    "rest_url": "https://app.cloopen.com:8883",
    
    # 号码配置
    "caller_numbers": [
        {"number": "057188888888", "type": "landline", "region": "杭州"},
        {"number": "13800138000", "type": "mobile", "region": "杭州"},
    ],
    
    # 中继线配置
    "trunk": {
        "type": "relay",                # 中继线（非直拨）
        "max_concurrent": 10,           # 最大并发通话数
        "codec": "PCMA",               # 音频编码
    },
    
    # 回调地址
    "callback_url": "http://localhost:8099/callback/call-status",
    "recording_callback_url": "http://localhost:8099/callback/recording",
}
```

### 7.2 号码策略

| 号码类型 | 来电显示 | 接通率 | 适用场景 |
|----------|----------|--------|----------|
| **本地座机** | 0571-8888XXXX | 35-45% | 企业服务、B2B |
| **本地手机** | 138XXXXXXXX | 50-65% | 个人消费、C端 |
| **95/96号码** | 955XX / 96XX | 40-55% | 金融、保险 |
| **400号码** | 400-XXX-XXXX | 30-40% | 品牌认知度高时 |

**防封号策略**:
- 单号码日呼上限 200 通
- 多号码轮换（≥5个号码/机器人）
- 接通后才计入号码配额
- 异常号码自动降频
- 被标记号码自动替换

### 7.3 通话状态机

```
IDLE → DIALING → RINGING → CONNECTED → TALKING → HANGUP → RECORDING_SAVED
  │       │         │          │          │         │
  │       ▼         ▼          ▼          ▼         ▼
  │    FAILED   NO_ANSWER   REJECTED  TRANSFERRED  ERROR
  │    (忙/错号) (无人接听)  (拒接)    (转人工)    (异常)
```

---

## 8. CRM 数据模型

### 8.1 核心模型

```go
// Customer 客户
type Customer struct {
    ID            uint      `gorm:"primaryKey"`
    CampaignID    uint      `gorm:"index"`                    // 所属外呼任务
    Phone         string    `gorm:"size:20;index"`            // 电话（加密存储）
    PhoneHash     string    `gorm:"size:64;uniqueIndex"`      // 电话哈希（去重用）
    Name          string    `gorm:"size:50"`                  // 姓名（可选）
    Industry      string    `gorm:"size:30"`                  // 行业
    Region        string    `gorm:"size:30"`                  // 地区
    IntentLevel   string    `gorm:"size:1;index"`             // 意向等级 A-F
    IntentScore   int       `gorm:"default:0"`                // 意向分数 0-100
    Tags          string    `gorm:"type:text"`                // 标签 JSON
    KeyInterests  string    `gorm:"type:text"`                // 关键兴趣点 JSON
    Summary       string    `gorm:"type:text"`                // AI 总结
    TotalCalls    int       `gorm:"default:0"`                // 总通话次数
    LastCallAt    *time.Time                                  // 最近通话时间
    NextFollowAt  *time.Time                                  // 下次跟进时间
    Status        string    `gorm:"size:20;default:'pending'"` // pending/active/converted/blacklisted
    AssignedTo    string    `gorm:"size:50"`                  // 分配给哪个业务员
    Source        string    `gorm:"size:30"`                  // 号码来源
    CreatedAt     time.Time
    UpdatedAt     time.Time
}

// CallRecord 通话记录
type CallRecord struct {
    ID            uint      `gorm:"primaryKey"`
    CustomerID    uint      `gorm:"index"`
    CampaignID    uint      `gorm:"index"`
    CallSID       string    `gorm:"size:64;uniqueIndex"`      // 云通信平台通话ID
    CallerNumber  string    `gorm:"size:20"`                  // 主叫号码
    CalleeNumber  string    `gorm:"size:20"`                  // 被叫号码
    Direction     string    `gorm:"size:10"`                  // outbound/inbound
    Status        string    `gorm:"size:20"`                  // connected/no_answer/rejected/failed
    Duration      int       `gorm:"default:0"`                // 通话时长(秒)
    IntentLevel   string    `gorm:"size:1"`                   // 本次通话意向
    IntentScore   int       `gorm:"default:0"`                // 本次通话分数
    Transcript    string    `gorm:"type:text"`                // 通话全文（ASR）
    Summary       string    `gorm:"type:text"`                // AI 摘要
    RecordingURL  string    `gorm:"size:500"`                 // 录音文件URL
    RecordingPath string    `gorm:"size:500"`                 // 录音本地路径
    AIAnalysis    string    `gorm:"type:text"`                // AI 分析 JSON
    StartedAt     time.Time
    EndedAt       *time.Time
    CreatedAt     time.Time
}

// Campaign 外呼任务
type Campaign struct {
    ID            uint      `gorm:"primaryKey"`
    Name          string    `gorm:"size:100"`                 // 任务名称
    Industry      string    `gorm:"size:30"`                  // 行业
    ScriptID      uint                                         // 话术模板ID
    Status        string    `gorm:"size:20;default:'draft'"`  // draft/running/paused/completed
    TotalNumbers  int       `gorm:"default:0"`                // 总号码数
    CalledCount   int       `gorm:"default:0"`                // 已拨打数
    ConnectedCount int      `gorm:"default:0"`                // 接通数
    IntentACount  int       `gorm:"default:0"`                // A类客户数
    IntentBCount  int       `gorm:"default:0"`                // B类客户数
    DailyLimit    int       `gorm:"default:800"`              // 每日上限
    CallerNumbers string    `gorm:"type:text"`                // 外显号码池 JSON
    ScheduleStart string    `gorm:"size:5;default:'09:00'"`   // 开始时间
    ScheduleEnd   string    `gorm:"size:5;default:'18:00'"`   // 结束时间
    ScheduleDays  string    `gorm:"size:20;default:'1,2,3,4,5,6'"` // 周几外呼
    CreatedAt     time.Time
    UpdatedAt     time.Time
}

// Script 话术模板
type Script struct {
    ID            uint      `gorm:"primaryKey"`
    Name          string    `gorm:"size:100"`                 // 模板名称
    Industry      string    `gorm:"size:30"`                  // 行业
    Greeting      string    `gorm:"type:text"`                // 开场白
    KeyPoints     string    `gorm:"type:text"`                // 关键卖点 JSON
    QALibrary     string    `gorm:"type:text"`                // 常见问答库 JSON
    Objections    string    `gorm:"type:text"`                // 异议处理 JSON
    Closing       string    `gorm:"type:text"`                // 结束语
    Vocie         string    `gorm:"size:30;default:'longxiaochun'"` // TTS 音色
    IsBuiltin     bool      `gorm:"default:false"`            // 是否内置
    CreatedAt     time.Time
    UpdatedAt     time.Time
}
```

### 8.2 数据加密

客户电话号码通过 Carapace 加密存储：

```go
// 存储时加密
customer.Phone = carapace.Encrypt(rawPhone, "phonebot-customer")
customer.PhoneHash = sha256(rawPhone)  // 用于去重，不可逆

// 读取时解密
rawPhone = carapace.Decrypt(customer.Phone, "phonebot-customer")
```

---

## 9. 外呼调度引擎

### 9.1 调度策略

```python
class CallScheduler:
    """外呼调度器 — 管理批量外呼的节奏和顺序"""
    
    def __init__(self, config):
        self.daily_limit = config.daily_limit          # 日上限
        self.max_concurrent = config.max_concurrent    # 最大并发
        self.call_interval = config.call_interval      # 两通间隔(秒)
        self.retry_intervals = [3600, 7200, 86400]     # 重试间隔(1h, 2h, 24h)
        self.max_retries = 3                           # 最大重试次数
    
    def get_next_batch(self, campaign_id, batch_size=10):
        """获取下一批待拨号码，优先级：
        1. 重试队列中到期的号码
        2. 按导入顺序的新号码
        3. 跳过: 黑名单、已完成、未到重试时间的
        """
        pass
    
    def should_call_now(self):
        """检查当前是否在外呼时段"""
        now = datetime.now()
        # 工作日 9:00-12:00, 14:00-18:00
        # 避开午休和晚间
        pass
```

### 9.2 并发控制

```
┌─────────────────────────────────────┐
│          调度器 (Scheduler)           │
│                                     │
│  待呼队列 ──→ 并发池(10路) ──→ 完成  │
│  [████████]   [▓▓▓░░░░░░░]   [███]  │
│                                     │
│  速率: 3秒/通  并发: 10路  日限: 800  │
└─────────────────────────────────────┘
```

- **并发路数**: 受中继线限制，默认 10 路（可扩展到 30 路）
- **拨号间隔**: 3 秒（防止运营商频控）
- **理论日呼量**: 10路 × 8小时 × 平均45秒/通 = ~6400通（远超 1000）

---

## 10. 合规与安全

### 10.1 法律合规

| 法规 | 要求 | 实现 |
|------|------|------|
| **《个人信息保护法》** | 号码收集须合法授权 | 导入时要求上传授权凭证 |
| **《反电信诈骗法》** | 不得冒充身份 | 开场明确身份，被问时如实告知是AI |
| **工信部骚扰电话治理** | 退订机制 | 按键/语音退订，72小时生效 |
| **《广告法》** | 禁止虚假宣传 | LLM Prompt 中禁止绝对化用语 |

### 10.2 退订机制

```python
# 退订触发条件
UNSUBSCRIBE_TRIGGERS = [
    "不需要", "别打了", "不要再打", "加入黑名单",
    "投诉", "骚扰", "报警",
    # 按键退订
    "DTMF_9",  # 按9退订
]

async def handle_unsubscribe(customer_id, reason):
    """退订处理"""
    customer.status = "blacklisted"
    customer.blacklisted_at = now()
    customer.blacklist_reason = reason
    # 72小时内同步到所有外呼任务
    await sync_blacklist(customer.phone_hash)
    # 通知管理员
    await notify_admin(f"客户 {customer.id} 已退订: {reason}")
```

### 10.3 通话频控

```python
RATE_LIMITS = {
    "per_number_daily": 1,              # 同一号码每天最多呼1次
    "per_number_monthly": 3,            # 同一号码每月最多呼3次
    "per_caller_hourly": 60,            # 单个外显号每小时上限
    "per_caller_daily": 200,            # 单个外显号每日上限
    "campaign_daily": 1000,             # 单任务每日上限
    "global_concurrent": 30,            # 全局最大并发
}
```

### 10.4 数据安全

- **号码加密**: Carapace AES-256-GCM 加密存储
- **录音存储**: 本地存储在 Claw 节点 `data/cicada/recordings/`
- **传输加密**: Bridge ↔ 云通信平台全程 TLS
- **访问控制**: 录音回放需要登录 Claw 节点
- **数据删除**: 客户可要求删除所有相关数据（GDPR/个保法）

---

## 11. 商业模式

### 11.1 定价方案

| 套餐 | 日呼量 | 并发路数 | 月费 | 含话费 | 超出话费 |
|------|--------|---------|------|--------|---------|
| **入门版** | 200通/天 | 3路 | ¥980/月 | 3,000分钟 | ¥0.10/分钟 |
| **专业版** | 500通/天 | 5路 | ¥2,480/月 | 8,000分钟 | ¥0.08/分钟 |
| **企业版** | 1,000通/天 | 10路 | ¥4,980/月 | 20,000分钟 | ¥0.06/分钟 |
| **旗舰版** | 3,000通/天 | 30路 | ¥9,800/月 | 60,000分钟 | ¥0.05/分钟 |

### 11.2 成本结构

以**专业版**（500通/天）为例：

| 成本项 | 单价 | 月用量 | 月成本 |
|--------|------|--------|--------|
| SIP话费 | ¥0.06/分钟 | 8,000分钟 | ¥480 |
| ASR 识别 | ¥1.2/小时 | 133小时 | ¥160 |
| TTS 合成 | ¥2/万字 | 150万字 | ¥300 |
| LLM 算力 | ¥0.8/百万token | 500万token | ¥4 |
| 服务器 | - | - | ¥100 |
| **月总成本** | | | **¥1,044** |
| **月收入** | | | **¥2,480** |
| **毛利率** | | | **57.9%** |

### 11.3 收入分配（三体架构）

```
月费 ¥2,480
  ├── 通信成本 (话费+ASR+TTS): ¥940 (37.9%)
  ├── 城市合伙人佣金 (25%): ¥620
  ├── NAV 注入 (10%): ¥248
  └── 平台留存: ¥672 (27.1%)
      ├── AI 算力: ¥104
      ├── 运营: ¥200
      └── 利润: ¥368
```

### 11.4 市场规模

- 中国电话机器人市场 2025 年约 ¥80 亿
- 中小企业渗透率 < 5%（大部分用人工坐席）
- 目标: 年1内获取 100 个付费企业 = ¥300万 ARR

---

## 12. 部署架构

### 12.1 本地部署（Spore 模式）

```
用户 PC / 服务器
├── Claw 节点 (Spore 安装)
│   └── Cicada Agent (已安装)
├── Cicada Bridge (Python, 本地 :8099)
│   ├── ASR/TTS → DashScope API (公网)
│   ├── SIP → 容联云 API (公网)
│   └── LLM → StarAI / localhost (可配)
└── data/cicada/
    ├── recordings/     # 通话录音
    ├── cicada.db       # SQLite 数据库
    └── config.yaml     # 本地配置
```

### 12.2 云端部署（Hive 模式）

```
Hive 集群 (starclaw.me)
├── Claw Container (已含 Cicada Agent)
├── Cicada Bridge Container (:8099)
└── 共享存储 → 录音文件
```

### 12.3 环境变量

```yaml
# Cicada Bridge 配置
CICADA_PORT: 8099

# 云通信 (容联云)
CLOOPEN_ACCOUNT_SID: ""
CLOOPEN_AUTH_TOKEN: ""
CLOOPEN_APP_ID: ""

# 语音服务 (DashScope)
DASHSCOPE_API_KEY: ""
DASHSCOPE_ASR_MODEL: "paraformer-realtime-v2"
DASHSCOPE_TTS_MODEL: "cosyvoice-v1"
DASHSCOPE_TTS_VOICE: "longxiaochun"

# LLM (StarAI 或本地)
LLM_BASE_URL: "https://api.star-ai.net/v1"
LLM_API_KEY: ""
LLM_MODEL: "qwen-turbo"

# Claw 节点
CLAW_URL: "http://localhost:8081"
CLAW_TOKEN: ""

# 存储
CICADA_DATA_DIR: "./data/cicada"
CICADA_DB_PATH: "./data/cicada/cicada.db"
```

---

## 13. 实现路径

### Phase 1: 电话管道 (2 周)

**目标**: 能打通一个电话，听到 AI 回复

| 任务 | 预估 | 产出 |
|------|------|------|
| 容联云账号注册 + API 对接 | 2天 | `sip_client.py` |
| DashScope ASR 流式对接 | 2天 | `voice_pipeline.py` (ASR部分) |
| DashScope TTS 流式对接 | 2天 | `voice_pipeline.py` (TTS部分) |
| ASR→LLM→TTS 串联 | 2天 | `voice_pipeline.py` (完整) |
| 外呼发起 + 状态回调 | 2天 | `call_engine.py` |
| 基础 FastAPI 服务 | 1天 | `main.py` |
| **端到端测试** | 1天 | 拨打测试号码，AI 对话 |

### Phase 2: Agent + 分类 (1 周)

**目标**: 话术引擎 + A-F 自动分类 + 录音保存

| 任务 | 预估 | 产出 |
|------|------|------|
| 意向分类引擎 | 2天 | `intent_classifier.py` |
| 话术模板引擎 | 1天 | `script_engine.py` + 行业 YAML |
| 录音下载+存储 | 1天 | `recorder.py` |
| CRM 数据模型 (Go) | 1天 | `model/*.go` |
| Claw Agent 模板 | 1天 | JSON 插件 + 本能 |
| 合规模块 | 1天 | `compliance.py` |

### Phase 3: 前端 (1 周)

**目标**: 完整的外呼管理面板

| 任务 | 预估 | 产出 |
|------|------|------|
| Dashboard 数据大盘 | 1天 | `DashboardPage.tsx` |
| Campaign 外呼任务 | 1天 | `CampaignPage.tsx` |
| Customer CRM 看板 | 1天 | `CustomerPage.tsx` |
| 录音回放 + 文字 | 1天 | `RecordPage.tsx` |
| 话术管理 | 0.5天 | `ScriptPage.tsx` |
| 设置页 | 0.5天 | `SettingsPage.tsx` |
| 号码导入(CSV/Excel) | 1天 | 上传+解析+去重 |

### Phase 4: 市场发布 (3 天)

**目标**: Queen 市场上架 + 可购买

| 任务 | 预估 | 产出 |
|------|------|------|
| Queen 市场 AgentListing | 1天 | `seed_cicada.go` |
| Claw 安装流程 | 1天 | `template.go` 适配 |
| 文档 + 使用指南 | 1天 | README + 用户手册 |

---

## 14. API 设计

### 14.1 Bridge API (Python :8099)

```
# 通话控制
POST   /call/dial              # 发起外呼
POST   /call/hangup            # 挂断
POST   /call/transfer          # 转接人工
GET    /call/status/:call_sid  # 通话状态

# 任务管理
POST   /campaign/start         # 启动外呼任务
POST   /campaign/pause         # 暂停
POST   /campaign/resume        # 恢复
GET    /campaign/progress      # 进度

# 回调 (云通信平台调用)
POST   /callback/call-status   # 通话状态回调
POST   /callback/recording     # 录音完成回调
POST   /callback/dtmf          # 按键回调

# 健康检查
GET    /health                 # 服务状态
GET    /stats                  # 实时统计
```

### 14.2 Go API (:8097 或嵌入 Claw)

```
# 客户管理
GET    /v1/cicada/customers              # 客户列表 (分页+筛选)
GET    /v1/cicada/customers/:id          # 客户详情
POST   /v1/cicada/customers/import       # 批量导入 (CSV/Excel)
PUT    /v1/cicada/customers/:id          # 更新客户信息
PUT    /v1/cicada/customers/:id/assign   # 分配给业务员
DELETE /v1/cicada/customers/:id          # 删除客户

# 通话记录
GET    /v1/cicada/calls                  # 通话列表
GET    /v1/cicada/calls/:id              # 通话详情 (含录音+文字)
GET    /v1/cicada/calls/:id/recording    # 录音文件下载
GET    /v1/cicada/calls/:id/transcript   # 通话文字

# 外呼任务
GET    /v1/cicada/campaigns              # 任务列表
POST   /v1/cicada/campaigns              # 创建任务
PUT    /v1/cicada/campaigns/:id          # 更新任务
POST   /v1/cicada/campaigns/:id/start    # 启动
POST   /v1/cicada/campaigns/:id/pause    # 暂停
DELETE /v1/cicada/campaigns/:id          # 删除

# 话术管理
GET    /v1/cicada/scripts                # 话术列表
POST   /v1/cicada/scripts               # 创建话术
PUT    /v1/cicada/scripts/:id           # 更新
DELETE /v1/cicada/scripts/:id           # 删除
GET    /v1/cicada/scripts/builtin       # 内置话术模板

# 数据统计
GET    /v1/cicada/analytics/overview     # 总览
GET    /v1/cicada/analytics/trend        # 趋势 (日/周/月)
GET    /v1/cicada/analytics/intent       # 意向分布
GET    /v1/cicada/analytics/conversion   # 转化漏斗
```

---

## 附录 A: 话术模板示例（房产行业）

```yaml
# scripts/real_estate.yaml
name: "房产新盘推广"
industry: real_estate
voice: longxiaochun

greeting: |
  您好，我是{company}的置业顾问小美，耽误您一分钟时间。
  我们{project_name}最近推出了一批特价房源，
  位于{location}，想跟您简单介绍一下。

key_points:
  - "项目位于{location}，周边配套成熟"
  - "户型从{min_area}到{max_area}平米，满足不同需求"
  - "均价{price}元/平米，首付{down_payment}万起"
  - "周边有{school}等优质学校"

qa_library:
  - q: "在什么位置"
    a: "项目位于{location}，距离{landmark}约{distance}，交通非常方便"
  - q: "多少钱"
    a: "目前均价{price}元/平米，{min_area}平的户型总价约{total_price}万"
  - q: "能分期吗"
    a: "可以的，首付{down_payment_ratio}，月供大约{monthly_payment}元"
  - q: "有学区吗"
    a: "项目对口{school}，是本区重点学校"

objections:
  - trigger: "太贵了"
    response: "理解您的顾虑，我们现在有特价房源，比正常售价优惠{discount}万，数量有限"
  - trigger: "再考虑考虑"
    response: "没问题，我加您微信发份详细资料，您方便时看看？"
  - trigger: "不需要"
    response: "好的，打扰了，祝您生活愉快，再见"

closing:
  positive: "那我安排我们的置业顾问跟您详细沟通，您看明天上午方便吗？"
  neutral: "我把项目资料发到您手机上，您有空看一下，有问题随时联系我们"
  negative: "好的，感谢您的时间，祝您生活愉快，再见"
```

## 附录 B: 与现有模块的集成点

| 模块 | 集成方式 | 用途 |
|------|----------|------|
| **Claw** | Agent 模板 + 技能插件 | Cicada 作为 Claw Agent 运行 |
| **Queen** | 市场上架 + 星能扣费 | 付费订阅 + 通话消耗星能 |
| **Synapse** | LLM 算力 | 话术生成 + 意向分类 |
| **Carapace** | 数据加密 | 客户号码加密存储 |
| **Overlord** | 企业管理 | 多坐席管理 + 数据汇总 |
| **Pheromone** | 事件总线 | 通话完成事件 → CRM 通知 |

---

## 附录 C: Claw 智能体市场上架方案

### C.1 市场发布模式

与 Q8bot 完全一致的 Bundle 模式，通过 Queen 市场发布，用户在 Claw 节点一键安装：

```
Queen MarketplaceItem (seed_cicada.go)
├── Agent Template         # Cicada 蝉·电话机器人
├── Skills (8个)           # phone_call, phone_listen, phone_speak, crm_query, crm_update, crm_schedule, record_save, sms_send
├── Instincts (6个)        # auto_dial, call_classify, daily_report, followup_remind, script_optimize, blacklist_sync
├── MCP Server (1个)       # cicada-bridge (:8099)
├── Workflow (2个)         # 标准外呼流程, 客户跟进流程
└── Plugins (8个)          # 对应8个技能的JSON工具定义
```

### C.2 定价与订阅

```go
// Queen MarketplaceItem 定价
MarketplaceItem{
    Name:           "Cicada 蝉·电话机器人",
    Slug:           "cicada-phonebot",
    Category:       "sales",
    PricingType:    "subscription",        // 订阅制
    PriceMonthly:   98000,                 // ¥980/月 (入门版，单位：分)
    PriceTiers: []PriceTier{
        {Name: "入门版", Price: 98000,  Features: "200通/天, 3路并发, 3000分钟"},
        {Name: "专业版", Price: 248000, Features: "500通/天, 5路并发, 8000分钟"},
        {Name: "企业版", Price: 498000, Features: "1000通/天, 10路并发, 20000分钟"},
        {Name: "旗舰版", Price: 980000, Features: "3000通/天, 30路并发, 60000分钟"},
    },
    CreatorSplit:    80,                   // 创作者分成 80%
    Tags:           []string{"电话机器人", "外呼", "CRM", "销售", "AI语音"},
}
```

### C.3 安装流程

```
用户在 Claw 市场浏览 → 点击"Cicada 蝉·电话机器人" → 选择套餐
→ 星能支付 → Queen 下发 Bundle → Claw InstallRemote()
→ 创建 Agent + 安装技能 + 注册本能 + 配置 MCP + 导入工作流
→ 提示用户配置: 云通信账号(容联云) + DashScope API Key
→ 安装 Cicada Bridge (Python, 本地自动启动)
→ 完成 ✅
```

### C.4 收入流向

```
用户月费 ¥2,480 (专业版)
    │
    ├──→ Queen 市场扣款 (星能)
    │       ├── 创作者分成 80%: ¥1,984
    │       │     ├── 通信成本 (话费+ASR+TTS): ¥940
    │       │     └── 创作者利润: ¥1,044
    │       ├── 平台分成 15%: ¥372
    │       └── 推荐人分成 5%: ¥124
    │
    └──→ 三体架构分配 (对创作者利润部分)
          ├── 城市合伙人佣金: 按阶梯
          ├── NAV 注入: 10%
          └── 平台留存
```

### C.5 市场页面展示

```yaml
# 市场展示信息
title: "Cicada 蝉·电话机器人"
subtitle: "AI 替代人工坐席，日呼 800-1000 通，自动意向分类"
icon: "🪰"
cover_image: "cicada-cover.png"
screenshots:
  - "dashboard.png"      # 数据大盘
  - "campaign.png"       # 外呼任务
  - "customer-crm.png"   # 客户CRM
  - "recording.png"      # 录音回放
  - "intent-chart.png"   # 意向分布图
highlights:
  - "日呼 800-1000 通，效率是人工的 10 倍"
  - "A-F 六级意向自动分类，精准锁定高意向客户"
  - "通话录音+文字自动保存，方便后续跟踪"
  - "中继线防封号，本地号码来电显示"
  - "6 大行业内置话术模板，开箱即用"
  - "客户数据加密存储在您自己的节点，绝不外泄"
supported_industries:
  - {name: "房产", icon: "🏠"}
  - {name: "教育", icon: "📚"}
  - {name: "金融", icon: "💰"}
  - {name: "装修", icon: "🔨"}
  - {name: "招商", icon: "🤝"}
  - {name: "医美", icon: "💉"}
```

---

## 附录 D: 虫族命名注册

Cicada 加入 StarClaw 虫族大家庭：

| 代号 | Emoji | 英文 | 中文 | 职能 |
|------|-------|------|------|------|
| Claw | 🦞 | Claw | 龙虾·开源核心 | AI Agent 引擎 |
| Queen | 👑 | Queen | 虫后·中央管控 | 平台管理+计费 |
| Overlord | 👁️ | Overlord | 领主·企业管理 | 企业SaaS |
| Synapse | ⛽ | Synapse | 突触·算力网关 | AI 模型路由 |
| Nydus | 🕳️ | Nydus | 虫道·部署管道 | CI/CD |
| Spore | 🍄 | Spore | 孢子·桌面安装 | 桌面端 |
| Larva | 🐛 | Larva | 幼虫·移动客户端 | Flutter App |
| Cerebrate | 🧠 | Cerebrate | 脑虫·记忆系统 | 合伙人管理 |
| Carapace | 🛡️ | Carapace | 甲壳·安全层 | 加密+审计 |
| Pheromone | 🧪 | Pheromone | 信息素·事件总线 | NATS 消息 |
| Drone | 🐝 | Drone | 雄蜂·采集器 | 数据采集 |
| Forge | 🔥 | Forge | 熔炉·研发管控 | 项目管理 |
| Chrysalis | 🦋 | Chrysalis | 蛹·变形引擎 | 工作流引擎 |
| Extractor | ⛏️ | Extractor | 提取器·Q8bot | 量化交易 |
| **Cicada** | **🪰** | **Cicada** | **蝉·电话机器人** | **AI 外呼** |

---

*文档结束 — Cicada 🪰 蝉·电话机器人智能体架构设计 v1.0*
