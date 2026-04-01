# Cicada 🪰 蝉 — AI 电话机器人智能体

> **StarClaw 虫族成员** — 以持续不断的鸣叫闻名，替代人工坐席完成批量外呼

## 功能

- **日呼 800-1000 通**，效率是人工坐席的 10 倍
- **A-F 六级意向自动分类**，精准锁定高意向客户
- **通话录音+文字**自动保存，方便后续跟踪
- **中继线防封号**，本地号码来电显示
- **6 大行业内置话术**：房产、教育、金融、装修、招商、医美

## 架构

```
cicada/
├── bridge/          # Python Bridge (FastAPI :8099)
├── scripts/         # 行业话术模板 (YAML)
├── docs/            # 设计文档
└── README.md
```

## 快速开始

```bash
cd cicada/bridge
pip install -r requirements.txt

# 编辑配置
cp config.yaml config.local.yaml
# 填入: 容联云账号、DashScope API Key、LLM API Key

# 启动
python main.py
```

## 配置

| 环境变量 | 说明 | 默认值 |
|----------|------|--------|
| `CICADA_PORT` | 服务端口 | 8099 |
| `CLOOPEN_ACCOUNT_SID` | 容联云账号 | - |
| `CLOOPEN_AUTH_TOKEN` | 容联云密钥 | - |
| `DASHSCOPE_API_KEY` | DashScope 密钥 | - |
| `LLM_BASE_URL` | LLM 地址 | https://api.star-ai.net/v1 |
| `LLM_API_KEY` | LLM 密钥 | - |

## 设计文档

详见 [PHONE_BOT_DESIGN.md](../docs/design/PHONE_BOT_DESIGN.md)
