-- Seed bounty tasks for StarClaw ecosystem development
-- Run: cat seed_bounties.sql | docker exec -i starclaw-queen-mysql mysql -u root -p starclaw_queen
SET NAMES utf8mb4;
DELETE FROM bounties WHERE node_id = 'system' AND user_id = 'system';

INSERT INTO bounties (id, node_id, user_id, title, description, category, requirements, reward, currency, status, visibility, deadline, created_at, updated_at) VALUES

-- ═══════════════════════════════════════════
-- PUBLIC: 所有 Claw 用户可见
-- ═══════════════════════════════════════════

-- 1. [公开] 开源文档翻译（英文）— 扩展海外用户
(UUID(), 'system', 'system',
 'StarClaw 开源文档英文翻译',
 '将 StarClaw 核心文档翻译为英文，覆盖 README、快速开始、API 参考、架构设计四个章节。目标是让海外开发者能够快速理解和使用 StarClaw。\n\n当前中文文档位于 claw/README.md 和 queen/site/src/docs-content.ts。\n\n翻译质量要求：\n- 技术术语准确（Agent、RAG、MCP、Workflow 等保留英文）\n- 语言自然流畅，非机翻风格\n- 代码示例中的注释也需翻译\n- 虫族命名体系保留拼音+英文注释（如 Claw 小龙虾、Queen 虫后）',
 'creative_design',
 '交付 4 个 Markdown 文件：README_EN.md、QUICKSTART_EN.md、API_EN.md、ARCHITECTURE_EN.md。每个文件需完整翻译对应中文内容。',
 500, 'CNY', 'open', 'public',
 DATE_ADD(NOW(), INTERVAL 14 DAY), NOW(), NOW()),

-- 2. [公开] MCP 工具服务器开发 — 丰富工具生态
(UUID(), 'system', 'system',
 '开发 MCP 工具服务器 — 通用数据库查询助手',
 '开发一个 MCP（Model Context Protocol）兼容的工具服务器，让 StarClaw Agent 能够安全地查询 MySQL/PostgreSQL 数据库。\n\n功能要求：\n1. 支持 MySQL 和 PostgreSQL 两种数据库\n2. 只读查询（SELECT only），禁止 DDL/DML\n3. 查询超时保护（默认 30s）\n4. 结果行数限制（默认 1000 行）\n5. 表结构自动发现（供 Agent 了解数据库 schema）\n6. 支持参数化查询，防 SQL 注入\n\n技术栈：Go 或 Python，需实现 MCP stdio 传输协议。',
 'code_review',
 '交付内容：\n1. 完整的 MCP 服务器源码（Go 或 Python）\n2. Dockerfile\n3. 配置文件示例（数据库连接串）\n4. README 含安装和使用说明\n5. 在 StarClaw 中的测试截图',
 1000, 'CNY', 'open', 'public',
 DATE_ADD(NOW(), INTERVAL 21 DAY), NOW(), NOW()),

-- 3. [公开] 视频教程制作 — 降低用户上手门槛
(UUID(), 'system', 'system',
 '录制 StarClaw 入门视频教程（3 集）',
 '为 StarClaw 录制一套入门视频教程，帮助新用户快速上手。\n\n教程规划：\n\n第 1 集（5-8 分钟）：安装与初体验\n- Spore 一键安装演示（Windows/macOS）\n- 首次对话体验\n- 模型切换（Qwen/DeepSeek/GPT-4o）\n\n第 2 集（8-10 分钟）：Agent 与工作流\n- 创建自定义 Agent（人设/工具/知识库）\n- 使用内置技能（搜索/图片生成/代码执行）\n- 拖拽构建简单工作流\n\n第 3 集（5-8 分钟）：进阶功能\n- RAG 知识库上传文档\n- MCP 工具连接\n- 加入虫群网络\n\n视频质量要求清晰（1080p+），语音讲解清楚，可以有字幕。',
 'creative_design',
 '交付内容：\n1. 3 集 MP4 视频文件（1080p+）\n2. 每集配套文字脚本\n3. 视频缩略图（3 张）\n4. 可选：上传到 B 站或 YouTube',
 1500, 'CNY', 'open', 'public',
 DATE_ADD(NOW(), INTERVAL 30 DAY), NOW(), NOW()),

-- 4. [公开] 移动端测试 — 提升产品质量
(UUID(), 'system', 'system',
 'Larva App (Flutter) 全功能测试 + Bug 报告',
 '对 StarClaw 移动端 App（Larva，Flutter 开发）进行全面的功能测试和用户体验评估。\n\n测试范围：\n1. 登录/注册流程（节点令牌登录）\n2. AI 对话功能（文本/图片/语音）\n3. Agent 选择和切换\n4. 设置页面（模型配置、主题切换）\n5. 星能余额查看\n6. 网络异常处理\n\n需要在 iOS 和 Android 真机上各测试一遍。',
 'real_world',
 '交付内容：\n1. 测试报告（Markdown 格式）\n2. Bug 列表（含截图、复现步骤、严重级别）\n3. UX 改进建议（至少 5 条）\n4. 各页面截图合集',
 300, 'CNY', 'open', 'public',
 DATE_ADD(NOW(), INTERVAL 14 DAY), NOW(), NOW()),

-- ═══════════════════════════════════════════
-- PARTNER: 仅团队/城市合伙人可见
-- ═══════════════════════════════════════════

-- 5. [合伙人] Agent 模板开发 — 丰富企业模板库
(UUID(), 'system', 'system',
 '开发「电商客服」Agent 模板 — EcomSupportClaw',
 '为 StarClaw Agent 市场开发一套电商客服 Agent 模板。该模板应覆盖以下场景：\n\n1. 售前咨询：产品推荐、规格对比、价格查询\n2. 售后服务：退换货流程、物流查询、投诉处理\n3. 订单管理：查询订单状态、修改地址、催发货\n\n需要利用 StarClaw 的 RAG 知识库加载产品数据，MCP 工具对接订单系统。\n\n参考现有 SupportClaw 模板的架构（Dispatcher + Responder + Escalator 模式）。',
 'expert_consult',
 '交付内容：\n1. Agent 模板 JSON 配置文件（含 3-5 个角色 Agent）\n2. 每个 Agent 的 System Prompt\n3. 推荐的工具列表和 MCP 配置\n4. 测试对话样例（至少 10 组）\n5. README 说明文档',
 800, 'CNY', 'open', 'partner',
 DATE_ADD(NOW(), INTERVAL 21 DAY), NOW(), NOW()),

-- 6. [合伙人] 安全渗透测试 — 平台安全保障
(UUID(), 'system', 'system',
 'StarClaw 安全渗透测试 — Web API + P2P 通信',
 '对 StarClaw 平台进行安全渗透测试，覆盖以下攻击面：\n\n1. Web API 安全\n- JWT 认证绕过\n- IDOR（不安全的直接对象引用）\n- SQL 注入 / XSS / CSRF\n- 文件上传漏洞\n- Rate Limit 绕过\n\n2. P2P 通信安全\n- Ed25519 签名验证绕过\n- Gossip 协议消息伪造\n- 中间人攻击\n\n3. 代码沙箱安全\n- 代码执行逃逸（Docker 沙箱突破）\n- 资源耗尽攻击（CPU/内存/磁盘）\n\n测试环境：使用 Spore 本地部署或 app.starclaw.me 在线演示（请勿对生产环境造成破坏性影响）。',
 'expert_consult',
 '交付内容：\n1. 安全测试报告（含漏洞等级 P0-P3）\n2. 每个漏洞的复现步骤和 PoC\n3. 修复建议\n4. 测试使用的工具列表',
 2000, 'CNY', 'open', 'partner',
 DATE_ADD(NOW(), INTERVAL 30 DAY), NOW(), NOW()),

-- 7. [合伙人] 城市落地方案 — 渠道拓展
(UUID(), 'system', 'system',
 '编写 StarClaw 城市合伙人落地推广方案',
 '为 StarClaw 城市合伙人编写一套可复用的本地化推广方案，目标是帮助城市合伙人快速获取前 10 个企业客户。\n\n方案应包含：\n1. 目标客户画像（行业/规模/痛点）\n2. 获客渠道（线上+线下各 3 种以上）\n3. 首次接触话术模板（电话/微信/邮件）\n4. 演示脚本（15 分钟产品演示流程）\n5. 定价谈判策略（Spark→Pulse→Surge 升级路径）\n6. 客户成功案例模板（可填充）\n7. 常见异议处理 Q&A（至少 15 个问题）\n\n方案需可直接交付给新入驻的城市合伙人使用。',
 'expert_consult',
 '交付内容：\n1. 完整推广方案文档（PDF/Markdown）\n2. 演示脚本 PPT（可编辑）\n3. 话术模板集合（Word/Markdown）\n4. Q&A 手册',
 1200, 'CNY', 'open', 'partner',
 DATE_ADD(NOW(), INTERVAL 21 DAY), NOW(), NOW()),

-- 8. [合伙人] 竞品分析报告 — 战略决策支持
(UUID(), 'system', 'system',
 '编写 AI Agent 平台竞品深度分析报告',
 '对国内外主要 AI Agent 平台进行深度竞品分析，帮助 StarClaw 团队明确差异化定位和产品演进方向。\n\n分析对象（至少覆盖 5 个）：\n- Dify、Coze（字节）、FastGPT、Flowise\n- LangChain/LangGraph、CrewAI、AutoGen\n- 百度智能体、阿里百炼\n\n分析维度：\n1. 产品功能对比矩阵（Agent/RAG/工作流/MCP/多模态/编程/部署）\n2. 技术架构对比（开源/闭源/部署模式/扩展性）\n3. 商业模式对比（免费/SaaS/企业版/社区版）\n4. 社区生态（GitHub Stars/Discord/插件市场）\n5. StarClaw 的差异化优势和改进建议\n6. 市场机会与威胁分析',
 'data_labeling',
 '交付内容：\n1. 完整分析报告（30+ 页，Markdown 或 PDF）\n2. 功能对比矩阵表格（可编辑 Excel/CSV）\n3. SWOT 分析图\n4. 改进建议优先级排序（P0-P3）',
 1500, 'CNY', 'open', 'partner',
 DATE_ADD(NOW(), INTERVAL 21 DAY), NOW(), NOW());
