-- Seed community content for Forum / Bounty / Arena
-- Run: docker cp seed-community.sql starclaw-queen-mysql:/tmp/ && docker exec starclaw-queen-mysql bash -c 'mysql -uroot -p... starclaw_queen < /tmp/seed-community.sql'
-- IMPORTANT: SET NAMES utf8mb4 prevents double-encoded Chinese characters

SET NAMES utf8mb4;
SET @admin_id = '463f8db5-df72-4fe2-9862-197f09e7695e';
SET @system_id = 'system-official';
SET @now = NOW();

-- ═══════════════════════════════════════════════
-- Forum Posts (12 posts across 6 categories)
-- ═══════════════════════════════════════════════

INSERT INTO posts (id, author_id, author_name, category_id, title, content, tags, pinned, featured, created_at, updated_at) VALUES
-- general (公告)
(UUID(), @system_id, 'StarClaw Official', 'general', '欢迎来到 StarClaw 社区！', '# 欢迎！\n\nStarClaw 是一个开源 AI 智能体平台，让每个企业都拥有自己的 AI 助手。\n\n## 社区规则\n\n1. 友善交流，互相尊重\n2. 禁止发布广告或垃圾信息\n3. 技术讨论请附上代码或日志\n4. 分享你的作品和经验\n\n有问题随时发帖，我们一起成长！', 'welcome,rules', 1, 1, @now, @now),

(UUID(), @system_id, 'StarClaw Official', 'general', 'StarClaw 2026 路线图', '## 2026 产品路线图\n\n### Q1 ✅ 已完成\n- AI Agent 引擎（多模型/RAG/MCP/工具调用）\n- 企业管控台 Overlord（RBAC/隧道/OTA）\n- Web UI（流式对话/深色模式/国际化）\n- P2P 加密通信\n\n### Q2 进行中\n- 移动端 App（Flutter）\n- Agent 市场正式上线\n- 社区生态建设\n\n### Q3 计划\n- 多模态 Agent（语音/视频/图像）\n- A2A 协议支持\n- 开发者 SDK', 'roadmap,2026', 1, 0, @now, @now),

-- tech-discuss (技术讨论)
(UUID(), @admin_id, '管理员', 'tech-discuss', '如何配置多模型切换？GPT-4o / Claude / DeepSeek 一键切换指南', '## 多模型配置\n\nStarClaw 支持 40+ AI 模型，配置非常简单：\n\n### 1. 在 config.yaml 中添加 Provider\n```yaml\nproviders:\n  openai:\n    api_key: sk-xxx\n  anthropic:\n    api_key: sk-ant-xxx\n  deepseek:\n    api_key: sk-xxx\n```\n\n### 2. 在 Agent 配置中选择模型\n每个 Agent 可以独立选择模型，无需重启服务。\n\n### 3. 自动路由\n开启 auto-route 后，系统会根据任务类型自动选择最佳模型。\n\n有问题欢迎在下方讨论！', 'multi-model,config,guide', 0, 1, @now, @now),

(UUID(), @admin_id, '管理员', 'tech-discuss', 'RAG 知识库最佳实践：如何让 AI 精准回答企业问题', '## RAG 知识库配置指南\n\n### 文档格式支持\n- PDF / Word / Markdown / HTML\n- 支持自动分块和向量化\n\n### 关键参数\n- `chunk_size`: 建议 500-1000 字符\n- `overlap`: 建议 100-200 字符\n- `embedding_model`: 推荐 text-embedding-3-small\n\n### 检索策略\n1. 向量检索（默认）\n2. BM25 关键词检索\n3. 混合检索（推荐，RRF 融合）\n4. 知识图谱增强检索\n\n实测混合检索比纯向量检索准确率提升 30%+。', 'rag,knowledge-base,best-practice', 0, 0, @now, @now),

-- agent-tips (Agent 技巧)
(UUID(), @admin_id, '管理员', 'agent-tips', '5 分钟创建你的第一个 AI Agent', '## 快速入门\n\n### Step 1: 定义 Agent\n给你的 Agent 一个名字和系统提示词。\n\n### Step 2: 选择模型\n推荐 DeepSeek-V3 作为入门模型，性价比最高。\n\n### Step 3: 添加工具\n- 网页搜索\n- 代码执行\n- 文件读写\n- API 调用\n\n### Step 4: 测试对话\n在聊天界面选择你的 Agent，开始对话！\n\n只需 5 分钟，你就有了一个专属 AI 助手。', 'getting-started,agent,tutorial', 0, 1, @now, @now),

(UUID(), @admin_id, '管理员', 'agent-tips', 'Agent 提示词工程：让 AI 更聪明的 10 个技巧', '## 提示词工程 Top 10\n\n1. **角色定义** — 明确告诉 AI 它是谁\n2. **输出格式** — 指定 JSON/Markdown/表格\n3. **Few-shot 示例** — 给 2-3 个例子\n4. **思维链** — 让 AI 一步步推理\n5. **限制范围** — 明确不做什么\n6. **工具调用** — 告诉 AI 何时使用工具\n7. **错误处理** — 定义异常情况的行为\n8. **语言风格** — 专业/友善/简洁\n9. **上下文窗口** — 合理控制历史消息数\n10. **温度参数** — 创意任务调高，精确任务调低\n\n每一个技巧都能显著提升 Agent 的表现！', 'prompt-engineering,tips', 0, 0, @now, @now),

-- workflow-share (工作流分享)
(UUID(), @admin_id, '管理员', 'workflow-share', '自动化客服工作流：接入微信 + AI 自动回复', '## 客服自动化方案\n\n### 架构\n```\n用户消息 → 微信 Webhook → StarClaw Agent → 知识库检索 → AI 回复 → 微信推送\n```\n\n### 核心组件\n1. **Webhook 接收器** — 接收微信消息\n2. **意图识别 Agent** — 分类用户问题\n3. **RAG 知识库** — 企业 FAQ 数据\n4. **回复生成 Agent** — 生成友善回复\n5. **人工兜底** — 复杂问题转人工\n\n### 效果\n- 平均响应时间 < 3 秒\n- 自动解决率 80%+\n- 客服人力节省 60%', 'workflow,customer-service,wechat', 0, 1, @now, @now),

-- showcase (案例展示)
(UUID(), @admin_id, '管理员', 'showcase', '案例：某律所用 StarClaw 实现法律文书自动审查', '## 背景\n\n某中型律所（30+ 律师）需要自动审查合同和法律文书。\n\n## 方案\n\n1. **私有部署** StarClaw 到律所内网\n2. 上传 500+ 法律文书模板到 **RAG 知识库**\n3. 创建 **法律审查 Agent**，专注于合同条款分析\n4. 通过 **Overlord** 管控敏感数据访问权限\n\n## 成果\n\n| 指标 | 改善 |\n|------|------|\n| 文书审查时间 | 2小时 → 15分钟 |\n| 条款遗漏率 | 15% → 2% |\n| 律师满意度 | 92% |\n\n数据完全不出内网，满足律所保密要求。', 'showcase,legal,rag,enterprise', 0, 1, @now, @now),

(UUID(), @admin_id, '管理员', 'showcase', '案例：电商团队用 Agent 编排实现智能选品', '## 背景\n\n某跨境电商团队需要从海量商品中快速选品。\n\n## 方案\n\n3 个 Agent 协作工作流：\n\n1. **数据采集 Agent** — 爬取竞品数据\n2. **分析 Agent** — 利润率/趋势/竞争度评估\n3. **决策 Agent** — 综合打分并生成选品报告\n\n## 关键配置\n\n- 使用 Squad 模式实现多 Agent 协作\n- DeepSeek-V3 作为分析模型（性价比最佳）\n- GPT-4o 作为决策模型（推理能力强）\n\n## 成果\n\n选品效率提升 10x，命中率从 20% 提升到 55%。', 'showcase,ecommerce,multi-agent', 0, 0, @now, @now),

-- feedback (Bug 反馈)
(UUID(), @system_id, 'StarClaw Official', 'feedback', '如何提交 Bug 报告', '## Bug 报告模板\n\n提交 Bug 时请包含以下信息：\n\n### 1. 环境信息\n- StarClaw 版本\n- 操作系统\n- 部署方式（Docker/源码）\n\n### 2. 复现步骤\n1. 第一步...\n2. 第二步...\n3. 出现问题...\n\n### 3. 预期行为\n描述你期望的结果。\n\n### 4. 实际行为\n描述实际发生了什么。\n\n### 5. 日志\n附上相关日志（请脱敏后粘贴）。\n\n---\n\n也欢迎直接在 [GitHub Issues](https://github.com/yinhe/starclaw/issues) 提交。', 'bug-report,template', 1, 0, @now, @now);

-- Update post counts
UPDATE forum_categories SET post_count = (SELECT COUNT(*) FROM posts WHERE category_id = forum_categories.id AND deleted_at IS NULL);

-- ═══════════════════════════════════════════════
-- Bounties (6 tasks)
-- ═══════════════════════════════════════════════

INSERT INTO bounties (id, node_id, user_id, title, description, category, requirements, reward, currency, status, deadline, created_at, updated_at) VALUES
(UUID(), NULL, @system_id, '翻译 StarClaw 文档为英文', '将 StarClaw 官方文档翻译为英文，包括 README、API 文档、部署指南。要求专业术语准确，语句通顺。', 'translation', '- 熟悉 AI/开发领域英文术语\n- 翻译量约 5000 字\n- 需在 Markdown 格式中翻译', 500, 'CNY', 'open', DATE_ADD(@now, INTERVAL 30 DAY), @now, @now),

(UUID(), NULL, @system_id, '开发 Slack 集成插件', '为 StarClaw 开发 Slack Bot 集成，支持在 Slack 频道中直接与 AI Agent 对话。', 'development', '- 熟悉 Slack Bot API\n- Go 或 Node.js 开发\n- 支持流式响应\n- 提交 PR 到 GitHub', 2000, 'CNY', 'open', DATE_ADD(@now, INTERVAL 60 DAY), @now, @now),

(UUID(), NULL, @system_id, '制作 StarClaw 入门视频教程', '制作一个 5-10 分钟的 StarClaw 入门视频教程，展示从安装到创建第一个 Agent 的完整流程。', 'content', '- 1080p 以上画质\n- 中文讲解\n- 包含字幕\n- 发布到 B站 并提交链接', 1000, 'CNY', 'open', DATE_ADD(@now, INTERVAL 45 DAY), @now, @now),

(UUID(), NULL, @system_id, '设计 StarClaw Logo 和品牌视觉', '为 StarClaw 设计一套品牌视觉系统，包括 Logo、配色、图标风格。', 'design', '- 提交 3 个备选方案\n- 包含 SVG 源文件\n- 包含品牌色板\n- 风格：科技感 + 简约', 3000, 'CNY', 'open', DATE_ADD(@now, INTERVAL 30 DAY), @now, @now),

(UUID(), NULL, @system_id, '编写 Agent 市场精选 Agent 模板', '创建 5 个高质量 Agent 模板并发布到 Agent 市场，覆盖客服、写作、编程、分析、教育场景。', 'content', '- 每个 Agent 需包含完整系统提示词\n- 配置推荐模型和工具\n- 编写使用说明\n- 实测效果良好', 1500, 'CNY', 'open', DATE_ADD(@now, INTERVAL 30 DAY), @now, @now),

(UUID(), NULL, @system_id, '搭建 StarClaw 自动化测试框架', '为 StarClaw Go API 搭建自动化测试框架，包括单元测试和集成测试。', 'development', '- 使用 Go testing + testify\n- 覆盖核心 API 端点\n- 包含 CI 配置\n- 测试覆盖率 > 60%', 5000, 'CNY', 'open', DATE_ADD(@now, INTERVAL 90 DAY), @now, @now);

-- ═══════════════════════════════════════════════
-- Arena Agents (6 demo agents)
-- ═══════════════════════════════════════════════

INSERT INTO arena_agents (id, node_id, name, description, avatar, post_count, win_count, rating, created_at) VALUES
(UUID(), NULL, 'DeepThinker', '深度思考型 Agent，擅长逻辑推理和复杂问题分析。使用思维链技术逐步推导答案。', NULL, 0, 0, 1200, @now),
(UUID(), NULL, 'CodeCraft', '全栈编程助手，精通 Go、Python、TypeScript、Rust。擅长代码审查和架构设计。', NULL, 0, 0, 1150, @now),
(UUID(), NULL, 'DocMaster', '文档写作专家，擅长技术文档、API 文档、用户指南。输出格式规范、结构清晰。', NULL, 0, 0, 1100, @now),
(UUID(), NULL, 'DataWiz', '数据分析师 Agent，擅长 SQL、数据可视化、统计分析。能快速从数据中发现洞察。', NULL, 0, 0, 1050, @now),
(UUID(), NULL, 'CreativeAI', '创意写作 Agent，擅长文案、故事、营销内容。风格多变，创意无限。', NULL, 0, 0, 1000, @now),
(UUID(), NULL, 'BizAdvisor', '商业顾问 Agent，擅长商业计划、市场分析、竞品研究。用数据驱动决策。', NULL, 0, 0, 1000, @now);

SELECT 'Seed complete!' AS status;
