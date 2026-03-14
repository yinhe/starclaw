import type { Locale } from './i18n'

export interface DocSectionContent {
  title: string
  content: string
}

type DocsMap = Record<string, DocSectionContent>

const en: DocsMap = {
  quickstart: {
    title: 'Quick Start',
    content: `## Quick Start

### 1. Clone the repository

\`\`\`bash
git clone https://github.com/yinhe/starclaw.git
cd starclaw
\`\`\`

### 2. Configure environment

\`\`\`bash
cp .env.example .env
# Edit .env — add your API keys (OpenAI, DeepSeek, etc.)
\`\`\`

### 3. Start services

\`\`\`bash
docker compose up -d
\`\`\`

### 4. Open your browser

Navigate to **http://localhost:3000** and create your account.

The default setup includes:
- **Claw API** on port 8080
- **Claw Web** on port 3000
- **MySQL** on port 3306
- **Redis** on port 6379`,
  },
  configuration: {
    title: 'Configuration',
    content: `## Configuration

StarClaw is configured via environment variables in \`.env\`:

### AI Provider Keys

| Variable | Provider | Required |
|----------|----------|----------|
| \`OPENAI_API_KEY\` | OpenAI (GPT-4, etc.) | Optional |
| \`DEEPSEEK_API_KEY\` | DeepSeek | Optional |
| \`ANTHROPIC_API_KEY\` | Anthropic (Claude) | Optional |
| \`GOOGLE_API_KEY\` | Google (Gemini) | Optional |

### Core Settings

| Variable | Default | Description |
|----------|---------|-------------|
| \`STARCLAW_PORT\` | 8080 | API server port |
| \`STARCLAW_SECRET\` | (random) | JWT signing key |
| \`STARCLAW_DB_DSN\` | (docker) | MySQL connection string |

### Storage

By default, uploaded files are stored in \`./data/uploads/\`. You can configure S3-compatible storage:

\`\`\`env
STARCLAW_STORAGE_TYPE=s3
STARCLAW_S3_BUCKET=my-bucket
STARCLAW_S3_REGION=us-east-1
\`\`\``,
  },
  models: {
    title: 'Models',
    content: `## Supported Models

StarClaw supports any OpenAI-compatible API endpoint.

### Built-in Providers

- **OpenAI** — GPT-4o, GPT-4.1, o3, o4-mini
- **DeepSeek** — DeepSeek-V3, DeepSeek-R1
- **Anthropic** — Claude Sonnet 4, Claude Haiku
- **Google** — Gemini 2.5 Pro, Gemini Flash
- **Qwen** — Qwen3 series
- **Grok** — Grok-4

### Custom Providers

Add any OpenAI-compatible endpoint in Settings → Models:

1. Click "Add Provider"
2. Enter the base URL (e.g. \`http://localhost:11434/v1\` for Ollama)
3. Add your API key (if required)
4. Select available models

### Local Models (Ollama)

\`\`\`bash
# Install Ollama
curl -fsSL https://ollama.ai/install.sh | sh

# Pull a model
ollama pull llama3.1

# StarClaw will auto-detect Ollama on localhost:11434
\`\`\``,
  },
  tools: {
    title: 'Tools & MCP',
    content: `## Tools & MCP

### Built-in Tools

StarClaw ships with powerful built-in tools:

- **Web Search** — Search the internet via DuckDuckGo
- **Code Execution** — Run Python/JS code in a sandbox
- **File Operations** — Read, write, and manage files
- **Image Generation** — Generate images via fal.ai
- **Video Production** — AI video with scene chaining
- **Feishu / Slack / Discord** — Messaging integrations

### MCP Protocol

StarClaw supports the [Model Context Protocol](https://modelcontextprotocol.io/):

1. Go to Settings → MCP
2. Add an MCP server URL
3. Available tools auto-populate

### Custom Tools

Create custom tools via the API:

\`\`\`json
POST /api/tools
{
  "name": "my_tool",
  "description": "What this tool does",
  "parameters": { ... },
  "endpoint": "https://my-api.com/action"
}
\`\`\``,
  },
  update: {
    title: 'Updating',
    content: `## Updating StarClaw

### One-Click Update (Recommended)

In the StarClaw web UI:

1. Go to **Settings → System**
2. If a new version is available, click **"Update"**
3. StarClaw will pull the latest code, rebuild, and restart automatically

### Manual Update

\`\`\`bash
cd starclaw
git fetch origin main
git reset --hard origin/main
docker compose build api web
docker compose up -d --no-deps api web
\`\`\`

### Version Check

StarClaw checks for updates hourly. The version format is \`vYYYY.MMDD.HHMM\`.

Update sources (with fallback):
1. **GitHub** — Primary source
2. **Nydus Mirror** — Fallback for regions with limited GitHub access`,
  },
  security: {
    title: 'Security',
    content: `## Security

### Self-Hosted Privacy

StarClaw runs entirely on your own infrastructure:

- **No telemetry** — Zero data sent to any external service
- **No cloud dependency** — Works fully offline (with local models)
- **Your keys** — API keys are stored locally, never transmitted

### Authentication

- JWT-based authentication with configurable secret
- Session management with automatic expiry
- Admin / user role separation

### Network

- All services bind to \`127.0.0.1\` by default (not exposed to internet)
- Use a reverse proxy (nginx) for production deployments
- HTTPS recommended for any public-facing instance

### API Keys

- Model API keys are encrypted at rest
- Keys are never included in API responses
- Per-key access control and rate limiting`,
  },
}

const zh: DocsMap = {
  quickstart: {
    title: '快速开始',
    content: `## 快速开始

### 1. 克隆仓库

\`\`\`bash
git clone https://github.com/yinhe/starclaw.git
cd starclaw
\`\`\`

### 2. 配置环境

\`\`\`bash
cp .env.example .env
# 编辑 .env — 添加你的 API 密钥（OpenAI、DeepSeek 等）
\`\`\`

### 3. 启动服务

\`\`\`bash
docker compose up -d
\`\`\`

### 4. 打开浏览器

访问 **http://localhost:3000** 并创建你的账号。

默认安装包含：
- **Claw API** 端口 8080
- **Claw Web** 端口 3000
- **MySQL** 端口 3306
- **Redis** 端口 6379`,
  },
  configuration: {
    title: '配置',
    content: `## 配置

StarClaw 通过 \`.env\` 文件中的环境变量进行配置：

### AI 服务商密钥

| 变量 | 服务商 | 是否必需 |
|------|--------|----------|
| \`OPENAI_API_KEY\` | OpenAI (GPT-4 等) | 可选 |
| \`DEEPSEEK_API_KEY\` | DeepSeek | 可选 |
| \`ANTHROPIC_API_KEY\` | Anthropic (Claude) | 可选 |
| \`GOOGLE_API_KEY\` | Google (Gemini) | 可选 |

### 核心设置

| 变量 | 默认值 | 说明 |
|------|--------|------|
| \`STARCLAW_PORT\` | 8080 | API 服务端口 |
| \`STARCLAW_SECRET\` | (随机) | JWT 签名密钥 |
| \`STARCLAW_DB_DSN\` | (docker) | MySQL 连接字符串 |

### 存储

默认情况下，上传文件存储在 \`./data/uploads/\`。你可以配置 S3 兼容存储：

\`\`\`env
STARCLAW_STORAGE_TYPE=s3
STARCLAW_S3_BUCKET=my-bucket
STARCLAW_S3_REGION=us-east-1
\`\`\``,
  },
  models: {
    title: '模型',
    content: `## 支持的模型

StarClaw 支持任何 OpenAI 兼容的 API 端点。

### 内置服务商

- **OpenAI** — GPT-4o、GPT-4.1、o3、o4-mini
- **DeepSeek** — DeepSeek-V3、DeepSeek-R1
- **Anthropic** — Claude Sonnet 4、Claude Haiku
- **Google** — Gemini 2.5 Pro、Gemini Flash
- **Qwen（通义千问）** — Qwen3 系列
- **Grok** — Grok-4

### 自定义服务商

在 设置 → 模型 中添加任何 OpenAI 兼容端点：

1. 点击"添加服务商"
2. 输入 Base URL（例如 Ollama: \`http://localhost:11434/v1\`）
3. 添加 API 密钥（如果需要）
4. 选择可用模型

### 本地模型（Ollama）

\`\`\`bash
# 安装 Ollama
curl -fsSL https://ollama.ai/install.sh | sh

# 拉取模型
ollama pull llama3.1

# StarClaw 会自动检测 localhost:11434 上的 Ollama
\`\`\``,
  },
  tools: {
    title: '工具 & MCP',
    content: `## 工具 & MCP

### 内置工具

StarClaw 内置强大的工具：

- **网页搜索** — 通过 DuckDuckGo 搜索互联网
- **代码执行** — 在沙箱中运行 Python/JS 代码
- **文件操作** — 读取、写入和管理文件
- **图像生成** — 通过 fal.ai 生成图像
- **视频制作** — AI 视频生成，支持场景串接
- **飞书 / Slack / Discord** — 消息集成

### MCP 协议

StarClaw 支持 [模型上下文协议 (MCP)](https://modelcontextprotocol.io/)：

1. 进入 设置 → MCP
2. 添加 MCP 服务器 URL
3. 可用工具会自动加载

### 自定义工具

通过 API 创建自定义工具：

\`\`\`json
POST /api/tools
{
  "name": "my_tool",
  "description": "这个工具的功能描述",
  "parameters": { ... },
  "endpoint": "https://my-api.com/action"
}
\`\`\``,
  },
  update: {
    title: '更新',
    content: `## 更新 StarClaw

### 一键更新（推荐）

在 StarClaw Web 界面中：

1. 进入 **设置 → 系统**
2. 如果有新版本可用，点击 **"更新"**
3. StarClaw 会自动拉取最新代码、重新构建并重启

### 手动更新

\`\`\`bash
cd starclaw
git fetch origin main
git reset --hard origin/main
docker compose build api web
docker compose up -d --no-deps api web
\`\`\`

### 版本检查

StarClaw 每小时检查一次更新。版本格式为 \`vYYYY.MMDD.HHMM\`。

更新源（带备用）：
1. **GitHub** — 主要来源
2. **Nydus 镜像** — GitHub 访问受限地区的备用源`,
  },
  security: {
    title: '安全',
    content: `## 安全

### 自托管隐私

StarClaw 完全运行在你自己的基础设施上：

- **无遥测** — 不向任何外部服务发送数据
- **不依赖云端** — 使用本地模型可完全离线工作
- **你的密钥** — API 密钥存储在本地，绝不外传

### 身份认证

- 基于 JWT 的认证，可配置密钥
- 会话管理，自动过期
- 管理员 / 用户 角色分离

### 网络

- 所有服务默认绑定 \`127.0.0.1\`（不暴露到互联网）
- 生产部署建议使用反向代理（nginx）
- 公网实例建议启用 HTTPS

### API 密钥

- 模型 API 密钥静态加密存储
- 密钥不会出现在 API 响应中
- 支持按密钥的访问控制和速率限制`,
  },
}

const ja: DocsMap = {
  quickstart: {
    title: 'クイックスタート',
    content: `## クイックスタート

### 1. リポジトリをクローン

\`\`\`bash
git clone https://github.com/yinhe/starclaw.git
cd starclaw
\`\`\`

### 2. 環境を設定

\`\`\`bash
cp .env.example .env
# .env を編集 — API キーを追加（OpenAI、DeepSeek など）
\`\`\`

### 3. サービスを起動

\`\`\`bash
docker compose up -d
\`\`\`

### 4. ブラウザを開く

**http://localhost:3000** にアクセスしてアカウントを作成。

デフォルトのセットアップ：
- **Claw API** ポート 8080
- **Claw Web** ポート 3000
- **MySQL** ポート 3306
- **Redis** ポート 6379`,
  },
  configuration: {
    title: '設定',
    content: `## 設定

StarClaw は \`.env\` の環境変数で設定します：

### AI プロバイダーキー

| 変数 | プロバイダー | 必須 |
|------|-------------|------|
| \`OPENAI_API_KEY\` | OpenAI (GPT-4 等) | 任意 |
| \`DEEPSEEK_API_KEY\` | DeepSeek | 任意 |
| \`ANTHROPIC_API_KEY\` | Anthropic (Claude) | 任意 |
| \`GOOGLE_API_KEY\` | Google (Gemini) | 任意 |

### コア設定

| 変数 | デフォルト | 説明 |
|------|-----------|------|
| \`STARCLAW_PORT\` | 8080 | API サーバーポート |
| \`STARCLAW_SECRET\` | (ランダム) | JWT 署名キー |
| \`STARCLAW_DB_DSN\` | (docker) | MySQL 接続文字列 |

### ストレージ

デフォルトでは \`./data/uploads/\` に保存。S3 互換ストレージも設定可能：

\`\`\`env
STARCLAW_STORAGE_TYPE=s3
STARCLAW_S3_BUCKET=my-bucket
STARCLAW_S3_REGION=us-east-1
\`\`\``,
  },
  models: {
    title: 'モデル',
    content: `## 対応モデル

StarClaw は OpenAI 互換 API エンドポイントに対応。

### 内蔵プロバイダー

- **OpenAI** — GPT-4o、GPT-4.1、o3、o4-mini
- **DeepSeek** — DeepSeek-V3、DeepSeek-R1
- **Anthropic** — Claude Sonnet 4、Claude Haiku
- **Google** — Gemini 2.5 Pro、Gemini Flash
- **Qwen** — Qwen3 シリーズ
- **Grok** — Grok-4

### カスタムプロバイダー

設定 → モデルで OpenAI 互換エンドポイントを追加：

1. 「プロバイダーを追加」をクリック
2. Base URL を入力（例: Ollama \`http://localhost:11434/v1\`）
3. API キーを追加（必要な場合）
4. 利用可能なモデルを選択

### ローカルモデル（Ollama）

\`\`\`bash
# Ollama をインストール
curl -fsSL https://ollama.ai/install.sh | sh

# モデルをプル
ollama pull llama3.1

# StarClaw が localhost:11434 の Ollama を自動検出
\`\`\``,
  },
  tools: {
    title: 'ツール & MCP',
    content: `## ツール & MCP

### 内蔵ツール

StarClaw の強力な内蔵ツール：

- **Web 検索** — DuckDuckGo でインターネット検索
- **コード実行** — サンドボックスで Python/JS を実行
- **ファイル操作** — ファイルの読み書きと管理
- **画像生成** — fal.ai で画像を生成
- **動画制作** — シーンチェーン付き AI 動画
- **Feishu / Slack / Discord** — メッセージング連携

### MCP プロトコル

StarClaw は [Model Context Protocol](https://modelcontextprotocol.io/) に対応：

1. 設定 → MCP に移動
2. MCP サーバー URL を追加
3. 利用可能なツールが自動的にロード

### カスタムツール

API でカスタムツールを作成：

\`\`\`json
POST /api/tools
{
  "name": "my_tool",
  "description": "ツールの説明",
  "parameters": { ... },
  "endpoint": "https://my-api.com/action"
}
\`\`\``,
  },
  update: {
    title: 'アップデート',
    content: `## StarClaw のアップデート

### ワンクリック更新（推奨）

StarClaw Web UI で：

1. **設定 → システム** に移動
2. 新しいバージョンがある場合、**「更新」** をクリック
3. 自動的にコードを取得、ビルド、再起動

### 手動更新

\`\`\`bash
cd starclaw
git fetch origin main
git reset --hard origin/main
docker compose build api web
docker compose up -d --no-deps api web
\`\`\`

### バージョンチェック

StarClaw は1時間ごとに更新を確認。バージョン形式: \`vYYYY.MMDD.HHMM\`

更新ソース（フォールバック付き）：
1. **GitHub** — プライマリソース
2. **Nydus ミラー** — GitHub アクセスが制限された地域用`,
  },
  security: {
    title: 'セキュリティ',
    content: `## セキュリティ

### セルフホストのプライバシー

StarClaw は完全に自分のインフラで動作：

- **テレメトリなし** — 外部サービスへのデータ送信ゼロ
- **クラウド依存なし** — ローカルモデルで完全オフライン動作
- **あなたのキー** — API キーはローカルに保存、送信されない

### 認証

- 設定可能なシークレットによる JWT ベースの認証
- 自動期限付きセッション管理
- 管理者 / ユーザーの役割分離

### ネットワーク

- すべてのサービスはデフォルトで \`127.0.0.1\` にバインド
- 本番環境ではリバースプロキシ（nginx）を推奨
- 公開インスタンスには HTTPS を推奨

### API キー

- モデル API キーは暗号化して保存
- キーは API レスポンスに含まれない
- キーごとのアクセス制御とレート制限`,
  },
}

const ko: DocsMap = {
  quickstart: {
    title: '빠른 시작',
    content: `## 빠른 시작

### 1. 저장소 클론

\`\`\`bash
git clone https://github.com/yinhe/starclaw.git
cd starclaw
\`\`\`

### 2. 환경 설정

\`\`\`bash
cp .env.example .env
# .env 편집 — API 키 추가 (OpenAI, DeepSeek 등)
\`\`\`

### 3. 서비스 시작

\`\`\`bash
docker compose up -d
\`\`\`

### 4. 브라우저 열기

**http://localhost:3000** 에 접속하여 계정을 생성하세요.

기본 설정 포함:
- **Claw API** 포트 8080
- **Claw Web** 포트 3000
- **MySQL** 포트 3306
- **Redis** 포트 6379`,
  },
  configuration: {
    title: '설정',
    content: `## 설정

StarClaw는 \`.env\` 파일의 환경 변수로 설정합니다:

### AI 프로바이더 키

| 변수 | 프로바이더 | 필수 |
|------|-----------|------|
| \`OPENAI_API_KEY\` | OpenAI (GPT-4 등) | 선택 |
| \`DEEPSEEK_API_KEY\` | DeepSeek | 선택 |
| \`ANTHROPIC_API_KEY\` | Anthropic (Claude) | 선택 |
| \`GOOGLE_API_KEY\` | Google (Gemini) | 선택 |

### 핵심 설정

| 변수 | 기본값 | 설명 |
|------|--------|------|
| \`STARCLAW_PORT\` | 8080 | API 서버 포트 |
| \`STARCLAW_SECRET\` | (랜덤) | JWT 서명 키 |
| \`STARCLAW_DB_DSN\` | (docker) | MySQL 연결 문자열 |

### 스토리지

기본적으로 \`./data/uploads/\`에 저장. S3 호환 스토리지 설정 가능:

\`\`\`env
STARCLAW_STORAGE_TYPE=s3
STARCLAW_S3_BUCKET=my-bucket
STARCLAW_S3_REGION=us-east-1
\`\`\``,
  },
  models: {
    title: '모델',
    content: `## 지원 모델

StarClaw는 모든 OpenAI 호환 API 엔드포인트를 지원합니다.

### 내장 프로바이더

- **OpenAI** — GPT-4o, GPT-4.1, o3, o4-mini
- **DeepSeek** — DeepSeek-V3, DeepSeek-R1
- **Anthropic** — Claude Sonnet 4, Claude Haiku
- **Google** — Gemini 2.5 Pro, Gemini Flash
- **Qwen** — Qwen3 시리즈
- **Grok** — Grok-4

### 커스텀 프로바이더

설정 → 모델에서 OpenAI 호환 엔드포인트 추가:

1. "프로바이더 추가" 클릭
2. Base URL 입력 (예: Ollama \`http://localhost:11434/v1\`)
3. API 키 추가 (필요한 경우)
4. 사용 가능한 모델 선택

### 로컬 모델 (Ollama)

\`\`\`bash
# Ollama 설치
curl -fsSL https://ollama.ai/install.sh | sh

# 모델 다운로드
ollama pull llama3.1

# StarClaw가 localhost:11434의 Ollama를 자동 감지
\`\`\``,
  },
  tools: {
    title: '도구 & MCP',
    content: `## 도구 & MCP

### 내장 도구

StarClaw의 강력한 내장 도구:

- **웹 검색** — DuckDuckGo로 인터넷 검색
- **코드 실행** — 샌드박스에서 Python/JS 실행
- **파일 작업** — 파일 읽기, 쓰기 및 관리
- **이미지 생성** — fal.ai로 이미지 생성
- **비디오 제작** — 장면 체인 AI 비디오
- **Feishu / Slack / Discord** — 메시징 통합

### MCP 프로토콜

StarClaw는 [Model Context Protocol](https://modelcontextprotocol.io/)을 지원:

1. 설정 → MCP로 이동
2. MCP 서버 URL 추가
3. 사용 가능한 도구가 자동으로 로드

### 커스텀 도구

API로 커스텀 도구 생성:

\`\`\`json
POST /api/tools
{
  "name": "my_tool",
  "description": "도구 설명",
  "parameters": { ... },
  "endpoint": "https://my-api.com/action"
}
\`\`\``,
  },
  update: {
    title: '업데이트',
    content: `## StarClaw 업데이트

### 원클릭 업데이트 (권장)

StarClaw 웹 UI에서:

1. **설정 → 시스템**으로 이동
2. 새 버전이 있으면 **"업데이트"** 클릭
3. 자동으로 최신 코드를 가져오고, 빌드하고, 재시작

### 수동 업데이트

\`\`\`bash
cd starclaw
git fetch origin main
git reset --hard origin/main
docker compose build api web
docker compose up -d --no-deps api web
\`\`\`

### 버전 확인

StarClaw는 매시간 업데이트를 확인합니다. 버전 형식: \`vYYYY.MMDD.HHMM\`

업데이트 소스 (폴백 포함):
1. **GitHub** — 기본 소스
2. **Nydus 미러** — GitHub 접근이 제한된 지역용`,
  },
  security: {
    title: '보안',
    content: `## 보안

### 셀프호스팅 프라이버시

StarClaw는 완전히 자체 인프라에서 실행:

- **텔레메트리 없음** — 외부 서비스로 데이터 전송 없음
- **클라우드 의존 없음** — 로컬 모델로 완전 오프라인 작동
- **내 키** — API 키는 로컬에 저장, 전송되지 않음

### 인증

- 설정 가능한 시크릿으로 JWT 기반 인증
- 자동 만료 세션 관리
- 관리자 / 사용자 역할 분리

### 네트워크

- 모든 서비스는 기본적으로 \`127.0.0.1\`에 바인딩
- 프로덕션 배포에는 리버스 프록시 (nginx) 권장
- 공개 인스턴스에는 HTTPS 권장

### API 키

- 모델 API 키는 암호화하여 저장
- 키는 API 응답에 포함되지 않음
- 키별 접근 제어 및 속도 제한`,
  },
}

const allDocs: Partial<Record<Locale, DocsMap>> = { en, zh, ja, ko }

export function getDocsContent(locale: Locale): DocsMap {
  return allDocs[locale] ?? en
}

export const SECTION_IDS = ['quickstart', 'configuration', 'models', 'tools', 'update', 'security'] as const
