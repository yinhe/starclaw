# Synapse 计费系统

## 架构概览

```
用户充值 (Queen starclaw.net)
  │  ¥人民币 → 星力 (1分 = 1 Star = 10,000 内部单位)
  ▼
Synapse API 请求 (star-ai.net)
  │
  ├── 1. CheckBalance → Queen RPC (check-credit) 或本地余额
  ├── 2. CalculateCost → 根据模型定价计算费用
  ├── 3. Deduct → Queen RPC (deduct-credit) 或本地扣费
  └── 4. Record → UsageRecord 写入数据库
```

## 计费类型

### Token 计费 (chat / reasoning / vision / embedding)

```
上游成本 = (输入tokens ÷ 1000) × input_price_cny
         + (输出tokens ÷ 1000) × output_price_cny

或 (USD 定价):
上游成本 = ((输入tokens ÷ 1M) × input_price_usd
         + (输出tokens ÷ 1M) × output_price_usd) × 汇率(7.2)

用户价 = 上游成本 × 1.3 (30% 加价率)
```

### 按次计费 (image / video / music / training / image_edit)

```
上游成本 = price_per_call_cny  或  price_per_call_usd × 汇率(7.2)
用户价 = 上游成本 × 1.3
```

### 按字符计费 (tts)

按 `price_per_char` 或 `price_per_char_cny` 计算（工具层调用，非 chat 流程）。

### 按分钟计费 (stt)

按 `price_per_min` 计算（工具层调用，非 chat 流程）。

## 定价单位说明

| YAML 字段 | 单位 | 示例 |
|-----------|------|------|
| `input_price` / `output_price` | **USD / 百万 tokens** | GPT-4o: $2.50 / $10.00 |
| `input_price_cny` / `output_price_cny` | **元 / 千 tokens** | qwen-max: 0.0025 / 0.01 |
| `price_per_call` | **USD / 次** | DALL-E 3: $0.04 |
| `price_per_call_cny` | **元 / 次** | 万象视频: ¥0.60 |
| `price_per_char` / `price_per_char_cny` | **USD or 元 / 字符** | TTS: ¥0.00002/字 |
| `price_per_min` | **USD / 分钟** | Whisper: $0.006/min |

**换算关系**: `input_price_cny: 0.0025` = ¥0.0025/千tokens = ¥2.5/百万tokens

## 全部模型定价

### OpenAI (USD/百万tokens)

| 模型 | 类型 | 输入 | 输出 | 上下文 |
|------|------|------|------|--------|
| gpt-4.1 | chat | $2.00 | $8.00 | 1M |
| gpt-4.1-mini | chat | $0.40 | $1.60 | 1M |
| gpt-4.1-nano | chat | $0.10 | $0.40 | 1M |
| gpt-4o | chat | $2.50 | $10.00 | 128K |
| gpt-4o-mini | chat | $0.15 | $0.60 | 128K |
| chatgpt-4o-latest | chat | $5.00 | $15.00 | 128K |
| gpt-4o-search-preview | chat | $2.50 | $10.00 | 128K |
| gpt-4-turbo | chat | $10.00 | $30.00 | 128K |
| gpt-4 | chat | $30.00 | $60.00 | 8K |
| gpt-3.5-turbo | chat | $0.50 | $1.50 | 16K |
| o3 | reasoning | $2.00 | $8.00 | 200K |
| o3-mini | reasoning | $1.10 | $4.40 | 200K |
| o3-pro | reasoning | $20.00 | $80.00 | 200K |
| o4-mini | reasoning | $1.10 | $4.40 | 200K |
| o1 | reasoning | $15.00 | $60.00 | 200K |
| o1-mini | reasoning | $3.00 | $12.00 | 128K |
| o1-pro | reasoning | $150.00 | $600.00 | 200K |
| codex-mini-latest | chat | $1.50 | $6.00 | 200K |
| dall-e-3 | image | $0.04/次 | — | — |
| tts-1 | tts | $0.000015/字 | — | — |
| whisper-1 | stt | $0.006/分 | — | — |
| text-embedding-3-small | embedding | $0.02 | — | — |

### Anthropic (USD/百万tokens)

| 模型 | 类型 | 输入 | 输出 | 上下文 |
|------|------|------|------|--------|
| claude-opus-4 | chat | $15.00 | $75.00 | 200K |
| claude-sonnet-4 | chat | $3.00 | $15.00 | 200K |
| claude-3.7-sonnet | chat | $3.00 | $15.00 | 200K |
| claude-3.5-sonnet | chat | $3.00 | $15.00 | 200K |
| claude-3.5-haiku | chat | $0.80 | $4.00 | 200K |
| claude-3-opus | chat | $15.00 | $75.00 | 200K |
| claude-3-haiku | chat | $0.25 | $1.25 | 200K |

### Google Gemini (USD/百万tokens)

| 模型 | 类型 | 输入 | 输出 | 上下文 |
|------|------|------|------|--------|
| gemini-2.5-pro | chat | $1.25 | $10.00 | 1M |
| gemini-2.5-flash | chat | $0.15 | $0.60 | 1M |
| gemini-2.5-flash-lite | chat | $0.075 | $0.30 | 1M |
| gemini-2.0-flash | chat | $0.10 | $0.40 | 1M |
| gemini-2.0-flash-lite | chat | $0.075 | $0.30 | 1M |
| gemini-1.5-pro | chat | $1.25 | $5.00 | 2M |
| gemini-1.5-flash | chat | $0.075 | $0.30 | 1M |

### Grok / xAI (USD/百万tokens)

| 模型 | 类型 | 输入 | 输出 | 上下文 |
|------|------|------|------|--------|
| grok-3 | chat | $3.00 | $15.00 | 131K |
| grok-3-mini | reasoning | $0.30 | $0.50 | 131K |
| grok-3-fast | chat | $5.00 | $25.00 | 131K |
| grok-2 | chat | $2.00 | $10.00 | 131K |
| grok-2-mini | chat | $0.20 | $1.00 | 131K |
| grok-2-vision | vision | $2.00 | $10.00 | 32K |

### Qwen 通义千问 (元/千tokens → 元/百万tokens)

| 模型 | 类型 | 输入 | 输出 | 上下文 |
|------|------|------|------|--------|
| qwen-max | chat | ¥2.5/M | ¥10/M | 32K |
| qwen-plus | chat | ¥0.8/M | ¥2/M | 131K |
| qwen-turbo | chat | ¥0.3/M | ¥0.6/M | 1M |
| qwen-flash | chat | 免费 | 免费 | 131K |
| qwen-long | chat | ¥0.5/M | ¥2/M | 10M |
| qwen3-max | chat | ¥2.5/M | ¥10/M | 32K |
| qwen3.5-plus | chat | ¥0.8/M | ¥4.8/M | 131K |
| qwen3.5-flash | chat | 免费 | 免费 | 131K |
| qwq-plus | reasoning | ¥0.8/M | ¥2/M | 131K |
| qwq-max | reasoning | ¥2.5/M | ¥10/M | 32K |
| qwq-32b | reasoning | ¥2/M | ¥6/M | 32K |
| qwen3-vl-plus | vision | ¥0.8/M | ¥2/M | 131K |
| qwen3-vl-flash | vision | 免费 | 免费 | 131K |
| qwen-vl-max | vision | ¥3/M | ¥9/M | 32K |
| qwen3-coder-plus | chat | ¥0.8/M | ¥2/M | 131K |
| qwen3-coder-flash | chat | 免费 | 免费 | 131K |
| qwen-deep-research | reasoning | ¥2/M | ¥8/M | 131K |
| wan2.6-t2v-plus | video | ¥0.60/次 | — | — |
| wan2.6-t2i | image | ¥0.04/次 | — | — |
| cosyvoice-v1 | tts | ¥0.00002/字 | — | — |
| text-embedding-v3 | embedding | ¥0.7/M | — | — |

### DeepSeek (元/千tokens → 元/百万tokens)

| 模型 | 类型 | 输入 | 输出 | 上下文 |
|------|------|------|------|--------|
| deepseek-chat (V3) | chat | ¥1/M | ¥2/M | 128K |
| deepseek-reasoner (R1) | reasoning | ¥4/M | ¥16/M | 128K |

### MiniMax (元/千tokens)

| 模型 | 类型 | 输入 | 输出 |
|------|------|------|------|
| MiniMax-M2.5 | chat | ¥1/M | ¥10/M |
| MiniMax-M2.5-highspeed | chat | ¥1/M | ¥4/M |
| MiniMax-Text-01 | chat | ¥1/M | ¥5/M |
| MiniMax-VL-01 | vision | ¥3/M | ¥9/M |
| MiniMax-Hailuo-2.3 | video | ¥0.30/次 | — |
| MiniMax-Hailuo-2.3-Fast | video | ¥0.15/次 | — |
| MiniMax-Music-2.5+ | music | ¥0.15/次 | — |
| MiniMax-Music-2.5 | music | ¥0.10/次 | — |
| MiniMax-Speech-2.8-hd | tts | ¥0.0003/字 | — |
| image-01 | image | ¥0.04/次 | — |

### fal.ai (USD/次)

| 模型 | 类型 | 价格/次 |
|------|------|---------|
| **视频生成** | | |
| veo3 / veo3.1 | video | $0.50 |
| sora2 | video | $0.40 |
| kling-video/v3/pro/text-to-video | video | $0.10 |
| kling-video/v3/standard/text-to-video | video | $0.05 |
| minimax-video | video | $0.10 |
| luma-dream-machine | video | $0.10 |
| **图片生成** | | |
| flux-2-max | image | $0.08 |
| flux-2-pro | image | $0.05 |
| flux-2 (dev) | image | $0.025 |
| flux-2/turbo | image | $0.015 |
| flux-2/flash | image | $0.01 |
| flux-2/klein/4b | image | $0.005 |
| flux-pro | image | $0.05 |
| flux-schnell | image | $0.003 |
| **音乐生成** | | |
| ace-step | music | $0.10 |
| minimax-music-v2 | music | $0.10 |
| diffrhythm | music | $0.05 |
| stable-audio | music | $0.05 |
| **模型训练** | | |
| flux-2-trainer | training | $1.00 |
| flux-2-klein-4b-base-trainer | training | $0.30 |

### DashScope 万象 (元/次)

| 模型 | 类型 | 价格/次 |
|------|------|---------|
| wan2.6-t2v | video | ¥0.24 |
| wan2.6-t2v-plus | video | ¥0.60 |
| wan2.6-t2v-turbo | video | ¥0.06 |
| wan2.6-i2v | video | ¥3.00 |
| wan2.6-i2v-turbo | video | ¥0.50 |
| wan2.6-i2v-plus | video | ¥3.00 |
| wanx-v1 | image | ¥0.04 |
| cosyvoice-v1 | tts | ¥0.00002/字 |

## 价格同步器

`PriceSyncer` 每 6 小时自动从各厂商 API 拉取最新模型列表和定价：

- **OpenAI**: `/v1/models` + 内置定价表
- **Anthropic**: `/v1/models` + 内置定价表
- **Google**: `/v1/models` + 内置定价表
- **Grok (xAI)**: `/v1/models` + 内置定价表
- **Qwen (DashScope)**: `/compatible-mode/v1/models` + 内置定价表
- **DeepSeek**: `/models` + 内置定价表
- **MiniMax**: `/v1/models` + 内置定价表

同步结果写入快照文件: `/data/pricing-snapshot.json`

### 同步策略

1. 调用各厂商 `/models` API 获取最新模型列表
2. 对比内置定价表，更新内存中的价格
3. 如有新模型未在定价表中，使用保守兜底价格（¥2/M tokens）
4. 写入 JSON 快照供审计

## 加价率

- **默认加价率**: 30% (`markup: 1.3`)
- **汇率**: USD → CNY = 7.2
- **用户最终价**: `上游成本 × 1.3`

## 扣费路径

1. **Claw 用户** (有 ClawID): Queen RPC `check-credit` → `deduct-credit`
2. **本地用户** (无 ClawID): Synapse DB `balance` / `free_quota` 扣减
3. **星力换算**: 1分(¥0.01) = 1 Star = 10,000 内部单位

## 文件结构

```
synapse/api/
├── providers/          # 各厂商 YAML 定价配置
│   ├── openai.yaml
│   ├── anthropic.yaml
│   ├── google.yaml
│   ├── grok.yaml
│   ├── qwen.yaml
│   ├── deepseek.yaml
│   ├── minimax.yaml
│   ├── fal.yaml
│   └── dashscope.yaml
├── internal/
│   ├── billing/
│   │   ├── meter.go              # 计费引擎
│   │   ├── queen_credit.go       # Queen HTTP 信用客户端
│   │   └── pheromone_bridge.go   # Pheromone RPC 信用客户端
│   └── provider/
│       ├── registry.go           # 模型注册表
│       └── price_sync.go         # 价格同步器
└── BILLING.md                    # 本文档
```
