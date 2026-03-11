# Router ⛽ — Extractor（提取器/气矿）

> star-ai.net — StarClaw 生态的 AI 算力提取器

## 虫族映射

**Extractor（提取器）** — 虫族在瓦斯气矿上建造提取器，采集高级资源（瓦斯）用于生产高级兵种。
star-ai.net 就是 StarClaw 虫群的提取器——坐落在各家 AI 提供商（气矿）之上，
为整个虫群汲取算力资源。

```
                    ┌────────────────────────────┐
                    │      StarClaw 虫群           │
                    │                              │
                    │  🦞🦞🦞  Claw 节点们         │
                    │      │  需要算力              │
                    │      ▼                        │
                    │  ⛽ Extractor (star-ai.net)   │
                    │      │  提取 & 路由            │
                    │      ▼                        │
                    │  ┌────────────────────┐       │
                    │  │ 气矿（AI Providers）│       │
                    │  │                    │       │
                    │  │ ⛏ OpenAI 气矿      │       │
                    │  │ ⛏ Anthropic 气矿   │       │
                    │  │ ⛏ Qwen 气矿        │       │
                    │  │ ⛏ DeepSeek 气矿    │       │
                    │  │ ⛏ fal.ai 气矿      │       │
                    │  │ ⛏ Replicate 气矿   │       │
                    │  │ ⛏ GPU 算力商气矿    │       │
                    │  └────────────────────┘       │
                    └────────────────────────────────┘
```

**类比：**
- 一个气矿 = 一个 AI Provider（OpenAI、fal.ai、GPU 算力商…）
- 一个提取器 = star-ai.net 的一个 Provider 适配器
- 瓦斯 = AI 算力（Token、GPU 时间、推理次数）
- 更多提取器 = 接入更多 Provider = 虫群算力越充沛
- 气矿枯竭 = Provider 配额用完 → 自动切换到其他气矿（故障转移）

---

## 定位

star-ai.net 融合三大能力：

| 能力 | 类似 | 说明 |
|------|------|------|
| **LLM 路由** | [OpenRouter.ai](https://openrouter.ai) | 统一 API 代理多家 LLM，一个 Key 用所有模型 |
| **媒体/算力 API** | [fal.ai](https://fal.ai) | 图片生成、视频生成、语音合成等 Serverless API |
| **算力市场** | 独创 | 算力提供商入驻，出售 GPU 时间 / 推理服务 |

---

## 双层架构

提取器采用 **Gateway + Proxy 双层架构**：

- **Gateway（Go，:8096）** — 前门。认证、计费、限流、智能路由、用量记录。国内模型（Qwen/DeepSeek）直连，不走 Proxy。
- **Proxy（Node.js，:8000）** — 海外中转站。专门代理国内无法直连的海外大模型（OpenAI、Anthropic、Grok、fal.ai、RunwayML）。部署在有海外网络的服务器上。

```
用户 / Claw 节点
    │
    ▼
┌───────────────────────────────────────────────────────┐
│  Gateway (Go, :8096)                                     │
│  ┌───────────────────────────────────────────────┐  │
│  │ 认证 (sk-star-xxx) → 计费 → 限流 → 智能路由          │  │
│  └──────────────┬────────────────┬───────────────┘  │
│                │                │                         │
│      国内模型 直连      海外模型 中转     管理类 API       │
│       (Go 原生)       ⇒ Proxy (:8000)     (Gateway 本地)    │
│                │                │                         │
│       ┌───────┴─────┐  ┌─────┴─────────────┐       │
│       │ 🇨🇳 国内气矿  │  │ 🇺🇸 海外气矿          │       │
│       │             │  │                     │       │
│       │ Qwen/       │  │ OpenAI (Chat/TTS/  │       │
│       │  DashScope  │  │  STT/Realtime WS)  │       │
│       │ DeepSeek    │  │ Anthropic (Claude) │       │
│       │ 国内算力商  │  │ Grok (多Key轮询)   │       │
│       │             │  │ fal.ai (Veo/Flux/  │       │
│       │             │  │  图生图/存储)      │       │
│       │             │  │ RunwayML (视频)     │       │
│       │             │  │ Google (Gemini)    │       │
│       └─────────────┘  └─────────────────────┘       │
│         ↑ 不经过 Proxy      ↑ 经过 Proxy 中转            │
│        国内网络直连       需要海外网络                   │
└───────────────────────────────────────────────────────┘
```

**为什么双层？**

| 维度 | Gateway (Go) | Proxy (Node.js) |
|------|-------------|----------------|
| **定位** | 前门（认证/计费/路由） | **海外大模型中转站** |
| **国内模型** | **直连**（Qwen/DeepSeek/国内算力商） | 不经过 |
| **海外模型** | 认证+计费 → 转发到 Proxy | OpenAI/Anthropic/Grok/fal.ai/RunwayML |
| **WebSocket** | 认证后升级代理到 Proxy | OpenAI Realtime 双向中继 |
| **SDK 生态** | 无需，纯 HTTP 代理 | fal.ai/RunwayML 官方 Node SDK |
| **部署位置** | 国内服务器 | 有海外网络的服务器 |
| **暴露** | 公网 :8096（star-ai.net） | 仅内网 :8000（不对外暴露） |

**核心原则：国内能直连的，绝不绕路。只有国内网络无法访问的海外 API 才走 Proxy 中转。**

---

## 目录结构

```
router/                              # ⛽ Extractor — star-ai.net
│
├── api/                             # 🚪 Go 后端（:8096）— 认证/计费/路由
│   ├── cmd/server/main.go           # 入口
│   ├── internal/
│   │   ├── handler/
│   │   │   ├── chat.go              # /v1/chat/completions → LLM 直连
│   │   │   ├── embeddings.go        # /v1/embeddings → LLM 直连
│   │   │   ├── images.go            # /v1/images/generations → Proxy
│   │   │   ├── audio.go             # /v1/audio/* → Proxy
│   │   │   ├── models.go            # /v1/models 聚合模型列表
│   │   │   ├── compute.go           # /v1/compute/* → Proxy（异步任务）
│   │   │   ├── realtime.go          # /v1/realtime/ws → Proxy WebSocket
│   │   │   └── keys.go              # API Key CRUD
│   │   ├── provider/
│   │   │   ├── registry.go          # Provider 注册中心（从 YAML 加载）
│   │   │   └── direct.go            # 国内 LLM 直连（Qwen/DeepSeek），海外走 Proxy
│   │   ├── router/
│   │   │   ├── router.go            # 智能路由引擎
│   │   │   ├── loadbalancer.go      # 负载均衡
│   │   │   ├── failover.go          # 故障转移
│   │   │   └── region.go            # 区域路由
│   │   ├── billing/
│   │   │   ├── meter.go             # Token/请求/GPU时间 计量
│   │   │   ├── pricing.go           # 定价引擎
│   │   │   ├── balance.go           # 余额检查 & 扣费
│   │   │   └── settlement.go        # 算力商月结
│   │   ├── middleware/
│   │   │   ├── auth.go              # sk-star-xxx 认证
│   │   │   ├── ratelimit.go         # 限流
│   │   │   └── logging.go           # 请求日志
│   │   └── model/
│   │       ├── provider.go          # Provider 模型
│   │       ├── api_key.go           # 用户 API Key
│   │       ├── usage.go             # 用量记录
│   │       └── compute_task.go      # 异步任务追踪
│   ├── providers/                   # 算力提供商声明式配置（YAML）
│   │   ├── openai.yaml
│   │   ├── anthropic.yaml
│   │   ├── qwen.yaml
│   │   ├── deepseek.yaml
│   │   ├── google.yaml
│   │   ├── fal.yaml
│   │   └── custom/                  # 第三方算力商入驻
│   │       └── example.yaml
│   ├── Dockerfile
│   └── go.mod                       # module github.com/yinhe/starclaw-router
│
├── proxy/                           # 🌏 海外中转站（Node.js，:8000）— 海外大模型代理
│   ├── server.js                    # Express 主服务（4600+ 行，已集成全部海外 Provider）
│   ├── config/
│   │   ├── index.js                 # 统一导出
│   │   ├── openaiClient.js          # OpenAI SDK 客户端
│   │   ├── falClient.js             # fal.ai SDK 客户端
│   │   ├── grokClient.js            # Grok 多客户端（最多 5 Key 轮询）
│   │   ├── runwayClient.js          # RunwayML SDK 客户端
│   │   ├── redis.js                 # Redis 连接
│   │   ├── bullQueues.js            # Bull 任务队列
│   │   ├── multer.js                # 文件上传配置
│   │   └── middlewares.js           # 限流 & API Key 验证
│   ├── routes/
│   │   └── imageToImageRoutes.js    # 图生图路由
│   ├── uploads/                     # 上传文件存储
│   ├── videos/                      # 生成视频存储
│   ├── package.json                 # Node.js 依赖
│   ├── Dockerfile                   # node:20-alpine + ffmpeg
│   └── .env.example                 # 环境变量模板
│
├── web/                             # 🖥️ React 前端（:3096）— 用户控制台
│   ├── src/
│   │   ├── pages/                   # 页面（Dashboard/Models/Compute/Keys/Usage/Billing）
│   │   ├── components/              # 公共组件
│   │   ├── lib/                     # API 层
│   │   └── stores/                  # Zustand 状态管理
│   ├── Dockerfile                   # node:20-alpine → nginx:alpine
│   ├── nginx.conf                   # 反代 /v1/ → api:8096
│   └── package.json                 # name: starclaw-router-web
│
├── docs/
│   ├── API.md                       # 统一 API 文档
│   ├── PROVIDER_GUIDE.md            # 算力商入驻指南
│   └── PRICING.md                   # 定价策略
│
├── docker-compose.yml               # api + web + proxy + mysql + redis
└── README.md                        # ← 本文件
```

---

## 三大功能

### 1. LLM 路由（类 OpenRouter）

统一 OpenAI 兼容格式代理所有主流 LLM：

```bash
# 一个 Key，所有模型
curl https://star-ai.net/v1/chat/completions \
  -H "Authorization: Bearer sk-star-xxx" \
  -d '{
    "model": "openai/gpt-4o",
    "messages": [{"role": "user", "content": "Hello"}]
  }'

# 切换模型只需改 model 字段
"model": "anthropic/claude-3.5-sonnet"
"model": "qwen/qwen-max"
"model": "deepseek/deepseek-chat"
"model": "google/gemini-2.0-flash"
```

### 2. 媒体/算力 API（类 fal.ai）

Serverless 模式调用图片、视频、音频等生成 API：

```bash
# 图片生成
POST /v1/compute/image/generate
{
  "model": "fal/flux-pro",
  "prompt": "a cyberpunk crayfish",
  "size": "1024x1024"
}
→ { "task_id": "xxx", "status": "processing" }

# 查询任务状态
GET /v1/compute/tasks/xxx
→ { "status": "completed", "output": { "url": "https://..." } }

# 视频生成
POST /v1/compute/video/generate
{
  "model": "fal/kling-video",
  "prompt": "...",
  "duration": 5
}

# 语音合成
POST /v1/compute/audio/tts
{
  "model": "fal/f5-tts",
  "text": "你好世界",
  "voice": "alloy"
}
```

### 3. 算力市场（算力提供商合作）

GPU 算力商可以入驻 star-ai.net，出售推理服务：

```yaml
# providers/custom/gpucloud-example.yaml
name: "GPUCloud"
type: "compute"
description: "高性价比 GPU 推理服务"
contact: "partner@gpucloud.com"
endpoint: "https://api.gpucloud.com/v1"
auth:
  type: "bearer"
  key_env: "GPUCLOUD_API_KEY"
models:
  - name: "gpucloud/llama-3.1-70b"
    type: "chat"
    input_price: 0.0005       # per 1K tokens (USD)
    output_price: 0.001
    context_length: 128000
    regions: ["cn-east", "cn-south"]
  - name: "gpucloud/sdxl-turbo"
    type: "image"
    price_per_call: 0.01
    regions: ["cn-east"]
settlement:
  method: "monthly"           # 月结
  currency: "CNY"
  min_payout: 100             # 最低结算 ¥100
```

**算力商合作流程：**

```
算力商申请入驻
  → 提供 API endpoint + 模型列表 + 定价
  → star-ai.net 审核 & 测试
  → 上线到模型列表
  → 用户调用 → star-ai.net 代理转发 → 算力商处理
  → 月结结算（star-ai.net 抽成 15-20%）
```

---

## 智能路由

```
用户请求: model="best/chat"（自动选择最优模型）

  ┌──────────────────────────────────────────────────┐
  │ 路由引擎                                          │
  │                                                    │
  │  1. 检查用户区域                                   │
  │     cn → 优先国内 Provider（Qwen/DeepSeek）        │
  │     us → 优先海外 Provider（OpenAI/Anthropic）     │
  │                                                    │
  │  2. 检查模型可用性                                  │
  │     Provider-A 健康? → ✅ 候选                     │
  │     Provider-B 健康? → ❌ 跳过（气矿枯竭）         │
  │     Provider-C 健康? → ✅ 候选                     │
  │                                                    │
  │  3. 选择策略                                       │
  │     - lowest_cost: 最便宜的候选                     │
  │     - lowest_latency: 最快的候选                    │
  │     - balanced: 综合成本+延迟                       │
  │     - priority: 按用户偏好顺序                      │
  │                                                    │
  │  4. 故障转移                                       │
  │     请求失败 → 自动重试下一个候选（气矿切换）       │
  └──────────────────────────────────────────────────┘
```

---

## Proxy 已集成能力（从 StarAI 项目移植）

Proxy 服务是从独立项目移植过来的成熟 Node.js 中转服务，已包含以下能力：

| 能力 | 端点 | 说明 |
|------|------|------|
| **OpenAI Chat** | `POST /chat/completions` | GPT 系列聊天，流式/非流式 |
| **OpenAI Realtime** | `WS /realtime/ws` | Realtime API WebSocket 双向中继 |
| **OpenAI TTS** | `POST /tts` | 文本转语音 |
| **OpenAI STT** | `POST /stt` | 语音转文本 |
| **Grok Chat** | `POST /grok/chat` | Grok 智能路由（最多 5 Key 轮询） |
| **Grok Completion** | `POST /grok/chat-completion` | Grok 高级模式（指定客户端） |
| **fal.ai Veo 3/3.1** | `POST /fal/veo3*` | 视频生成（队列：submit/status/result） |
| **fal.ai Flux Pro** | `POST /fal/flux-pro-kontext` | 高质量图像生成 |
| **fal.ai Storage** | `POST /fal/storage/upload` | 文件上传到 fal CDN |
| **fal.ai 图生图** | routes/imageToImageRoutes.js | 图像风格转换 |
| **RunwayML** | 视频处理端点 | RunwayML SDK 调用 |
| **文档解析** | PDF/DOCX/Excel | pdf-parse + mammoth + xlsx |
| **图像处理** | Sharp | 缩放/裁剪/格式转换 |
| **文件代理** | `POST /internal/fetch` | 安全代理下载 fal.media 文件 |

**已有特性：**
- ✅ Grok 多 Key 负载均衡（轮询 + 模型智能选择）
- ✅ fal.ai 异步队列（submit → poll status → get result）
- ✅ OpenAI Realtime WebSocket 双向中继（含 function calling 日志）
- ✅ Bull Queue + Redis 任务队列
- ✅ Socket.IO 实时通知
- ✅ 文件上传（Multer）+ 静态文件服务
- ✅ API Key 验证中间件
- ✅ 限流（express-rate-limit）

---

## 请求路由规则

Gateway 根据 **Provider 地理位置** 决定直连还是走 Proxy 中转：

### 🇨🇳 国内模型 — Gateway 直连（不走 Proxy）

| 请求 | 路由 | Provider |
|------|------|----------|
| `POST /v1/chat/completions` model=`qwen/*` | Gateway 直连 | Qwen/DashScope |
| `POST /v1/chat/completions` model=`deepseek/*` | Gateway 直连 | DeepSeek |
| `POST /v1/embeddings` model=`qwen/*` | Gateway 直连 | Qwen |
| `POST /v1/images/generations` model=`qwen/*` | Gateway 直连 | Qwen (wan2.6-t2i) |
| `POST /v1/audio/speech` model=`qwen/*` | Gateway 直连 | Qwen (cosyvoice) |
| 国内算力商模型 | Gateway 直连 | custom/*.yaml |

### 🇺🇸 海外模型 — 经 Proxy 中转

| 请求 | 路由 | Provider |
|------|------|----------|
| `POST /v1/chat/completions` model=`openai/*` | Gateway → Proxy | OpenAI |
| `POST /v1/chat/completions` model=`anthropic/*` | Gateway → Proxy | Anthropic |
| `POST /v1/chat/completions` model=`google/*` | Gateway → Proxy | Google Gemini |
| `POST /v1/chat/completions` model=`grok/*` | Gateway → Proxy | Grok (多Key轮询) |
| `POST /v1/images/generations` model=`fal/*` | Gateway → Proxy | fal.ai (Flux Pro) |
| `POST /v1/compute/video/*` | Gateway → Proxy | fal.ai (Veo 3/3.1) |
| `POST /v1/compute/image/*` | Gateway → Proxy | fal.ai |
| `POST /v1/audio/speech` model=`openai/*` | Gateway → Proxy | OpenAI TTS |
| `POST /v1/audio/transcriptions` | Gateway → Proxy | OpenAI Whisper |
| `WS /v1/realtime` | Gateway → Proxy | OpenAI Realtime WS |
| RunwayML 视频处理 | Gateway → Proxy | RunwayML |

### 🔧 管理类 API — Gateway 本地处理

| 请求 | 处理 |
|------|------|
| `GET /v1/models` | 聚合所有 Provider 模型列表 |
| `GET/POST /v1/keys` | API Key CRUD (MySQL) |
| `GET /v1/usage` | 用量查询 (MySQL) |
| `GET /v1/balance` | 余额查询 (MySQL) |

**路由决策树：**
```
model 前缀是 qwen/* 或 deepseek/* 或国内算力商?
  ├─ 是 → Gateway 直连 Provider API（国内网络直通）
  └─ 否 → Gateway 认证+计费 → 转发 Proxy (:8000) → 海外 Provider API
```

---

## 技术栈

| 层级 | 技术 |
|------|------|
| API（Go 后端） | Go + Gin + httputil.ReverseProxy |
| Proxy（海外中转） | Node.js + Express + fal.ai/OpenAI/Grok SDK |
| 任务队列 | Bull (Redis-backed) |
| 实时通信 | WebSocket (ws) + Socket.IO |
| 数据库 | MySQL 8（用户/Key/用量/结算） |
| 缓存 | Redis（限流/队列/模型状态） |
| Web（React 前端） | React + Vite + TailwindCSS + Zustand |
| 部署 | Docker Compose |
| 监控 | Prometheus + Grafana |

---

## 与其他模块关系

```
router/ ⛽ Extractor
    ├── api/     🚪 Go 后端 (:8096)            ← 公网入口 (star-ai.net)
    │   ├── 🇨🇳 国内模型 → 直连 (Qwen/DeepSeek)
    │   └── 🇺🇸 海外模型 → 转发 Proxy
    ├── web/     🖥️ React 前端 (:3096)          ← 用户控制台
    └── proxy/   🌏 海外中转 (Node.js, :8000)   ← 仅内网（海外网络服务器）
         └── OpenAI / Anthropic / Grok / fal.ai / RunwayML

claw/ 🦞 Claw               → 调用 star-ai.net API（默认 Provider）
queen/ 👑 Queen              → 计费系统互通，用户体系互通
overlord/ 👁️ Overlord        → 企业可配置私有 Extractor（自建气矿）
```

---

## 端口分配

| 服务 | 端口 | 暴露 | 说明 |
|------|------|:----:|------|
| API | :8096 | 公网 | Go 后端主入口（star-ai.net） |
| Proxy | :8000 | 内网 | 海外大模型中转站（部署在海外网络服务器） |
| Web | :3096 | 公网 | React 用户控制台前端 |
| MySQL | 共享 | 内网 | 同一数据库不同 schema |
| Redis | 共享 | 内网 | 限流 + 任务队列 + 缓存 |
