# Overlord × Team Agent — 企业级团队智能体方案

> **从"管 AI 的工具"到"AI 帮你干活的平台"**
> 团队智能体是 Overlord 的核心差异化，也是整个 StarClaw 商业化的尖刀产品。

---

## 一、产品重新定位

### 1.1 核心问题

```
当前 Overlord 卖的是什么？
  → "管理你的 AI 节点"（节点管控 + 团队隔离 + 预算审计）
  → 本质上是 IT 管理工具，买单的是 IT 部门

问题：
  1. IT 管理工具的竞争对手太多（飞书/钉钉/企微自带 AI 管控）
  2. "管控"是成本中心叙事，企业不愿为"管控"付高价
  3. 没有讲清楚"AI 帮企业做了什么"，只讲了"企业怎么管 AI"
  4. 节点/Token 这些技术概念，企业决策者听不懂
```

### 1.2 重新定位

```
┌────────────────────────────────────────────────────────┐
│                                                        │
│  Before: Overlord = 企业 AI 管控平台                    │
│    "管理你公司的 AI 节点和用量"                           │
│    买单人: IT 部门 / CTO                                │
│    竞品: 飞书 AI / 钉钉 AI / Dify Enterprise           │
│    定位: 成本中心（管控 = 花钱的事）                      │
│                                                        │
│  After: Overlord = 企业 AI 团队平台                     │
│    "给你的公司雇一支 AI 团队"                             │
│    买单人: 业务部门 / CEO                                │
│    竞品: 咨询公司 / 外包公司 / 招聘                       │
│    定位: 利润中心（干活 = 赚钱的事）                      │
│                                                        │
│  一句话:                                                │
│  不是"给你一个管 AI 的后台"                              │
│  而是"给你的公司雇一支永远在线的 AI 团队"                  │
│                                                        │
└────────────────────────────────────────────────────────┘
```

### 1.3 新的价值主张

```
给企业决策者:
  "你花 ¥2,000/月，得到一支 7×24 在线的 AI 开发团队"
  "相当于半个实习生的价格，干 3 个初级工程师的活"
  "不请假、不离职、不摸鱼、可审计、可回溯"

给 IT 部门:
  "原有的管控能力一个不少（SSO/RBAC/审计/预算）"
  "加上了 AI 团队编排，让 AI 从对话工具变成生产力"

给员工:
  "不用自己写 Prompt，直接说需求，AI 团队帮你搞定"
  "有架构师帮你设计、有码农帮你编码、有测试帮你验收"
```

### 1.4 竞品重新对标

```
旧竞品（IT 管控赛道）:
  飞书 AI        → ¥30/人/月 × 100人 = ¥3,000/月
  钉钉 AI        → ¥9.9/人/月 × 100人 = ¥990/月
  Dify Enterprise → 面议

新竞品（AI 劳动力赛道）:
  外包公司       → 1 个初级开发 ¥8,000/月
  Devin (Cognition) → $500/月 = ¥3,600/月（单 Agent）
  GitHub Copilot Workspace → $39/人/月 × 20 dev = $780/月 = ¥5,600/月
  Cursor Business → $40/人/月 × 20 dev = $800/月 = ¥5,700/月

StarClaw Overlord Pro:
  ¥2,000/月 → 多 Agent 团队协作 + 企业管控 + 私有部署
  比 Devin 便宜 44%，且支持多 Agent 协作 + 企业合规
  比 Copilot/Cursor 便宜 64%，且不限人数只限节点
  比外包便宜 75%，且 7×24 在线 + 可审计
```

---

## 二、产品架构升级

### 2.1 三层架构

```
┌─────────────────────────────────────────────────┐
│  Layer 3: 企业 AI 团队  (新增核心层)              │
│                                                  │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐        │
│  │ DevClaw  │ │ MarketClaw│ │ SupportClaw│       │
│  │ 开发团队  │ │ 营销团队  │ │ 客服团队   │       │
│  └──────────┘ └──────────┘ └──────────┘        │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐        │
│  │ DataClaw │ │ OpsClaw  │ │ LegalClaw │        │
│  │ 数据团队  │ │ 运维团队  │ │ 法务团队   │       │
│  └──────────┘ └──────────┘ └──────────┘        │
│                                                  │
│  每个团队 = 多个专业 Agent 协作                    │
│  自动规划 → 并行执行 → 审查 → 交付                 │
├─────────────────────────────────────────────────┤
│  Layer 2: 企业管控  (已有 → 增强)                 │
│                                                  │
│  RBAC · SSO · 审计 · 预算 · 合规 · 白牌           │
│  团队隔离 · 模型路由 · Webhook · 数据安全          │
├─────────────────────────────────────────────────┤
│  Layer 1: AI 基础设施  (已有)                     │
│                                                  │
│  Claw 节点 · 模型接入 · 知识库 · 工具 · MCP       │
│  Nydus 隧道 · Molt 更新 · Hivemind 节点发现      │
└─────────────────────────────────────────────────┘
```

### 2.2 Layer 3 与 Layer 2 的关系

```
Layer 2 管控能力全部复用，且为 Team Agent 提供企业级保障:

RBAC:
  → 管理员可以创建/解散 AI 团队
  → 部门经理只能使用被分配的 AI 团队
  → 普通员工只能向 AI 团队提交任务

SSO:
  → 员工 SSO 登录后自动关联到所属部门
  → 部门关联的 AI 团队自动可见

审计:
  → AI 团队的每个决策、每行代码都有记录
  → 可追溯：谁提的需求 → 哪个 Agent 做的 → 审查意见 → 最终交付

预算:
  → AI 团队有独立预算上限（按部门分配）
  → 超预算自动暂停 + 告警
  → 月报按团队分拆成本

合规:
  → AI 团队产出经过审查 Agent 把关
  → 敏感操作需人工审批（知识库写入、外部 API 调用）
  → 数据不出私有环境
```

---

## 三、企业 Team Agent 模板库

### 3.1 官方模板

| 模板 | 角色构成 | 目标客户 | 典型任务 |
|------|---------|---------|---------|
| **DevClaw** (开发团队) | Architect + Drone×3 + Tester + Reviewer + DocBot | 研发部门 | 功能开发、Bug 修复、技术文档 |
| **MarketClaw** (营销团队) | Strategist + Copywriter×2 + Designer + Analyst | 市场部门 | 文案创作、活动策划、数据分析 |
| **SupportClaw** (客服团队) | Dispatcher + Responder×3 + Escalator + Analyst | 客服部门 | 工单处理、FAQ 维护、客户分析 |
| **DataClaw** (数据团队) | Architect + ETL-Bot + Analyst×2 + Reporter | 数据部门 | 数据清洗、报表生成、趋势分析 |
| **OpsClaw** (运维团队) | Monitor + Medic + Guardian + Reporter | 运维部门 | 监控告警、故障诊断、自动修复 |
| **LegalClaw** (法务团队) | Reviewer + Researcher + Drafter + Checker | 法务部门 | 合同审查、法规检索、文书起草 |
| **HRClaw** (人力团队) | Screener + Interviewer + Onboarder + Analyst | HR 部门 | 简历筛选、面试评估、入职流程 |
| **FinanceClaw** (财务团队) | Auditor + Analyst + Reporter + Checker | 财务部门 | 报表审计、费用分析、合规检查 |
| **QuantClaw** (量化团队) | Strategist + Researcher + Coder + Backtester + RiskGuard | 量化/投研 | 策略研发、因子挖掘、回测、风控 |
| **DramaClaw** (短剧团队) | Director + Screenwriter + Storyboarder + VideoMaker + Editor + Distributor | 短剧/MCN | 剧本创作、分镜设计、AI 视频生成、剪辑、分发 |
| **MusicClaw** (音乐团队) | Producer + Lyricist + Composer + Visualizer + MVDirector + Distributor | 音乐/MCN | 词曲创作、编曲、MV 制作、发行分发 |
| **EcomClaw** (电商团队) | ProductMgr + Copywriter + Designer + Optimizer + Analyst | 电商/品牌商 | 商品文案、主图详情页、SEO 优化、销售分析 |
| **SalesClaw** (销售团队) | Prospector + Qualifier + ProposalWriter + Negotiator + Analyst | B2B 销售 | 线索挖掘、商机评估、方案撰写、跟进策略 |
| **TransClaw** (翻译团队) | Coordinator + Translator×2 + Reviewer + Localizer + TermMgr | 出海/跨国企业 | 多语种翻译、本地化、术语管理、文化适配 |
| **TeachClaw** (教育团队) | Planner + CourseBuilder + ExamMaker + Tutor + Grader | 学校/培训机构 | 课程设计、题库生成、AI 辅导、自动批改 |
| **DesignClaw** (设计团队) | ArtDirector + UIDesigner + Illustrator + BrandGuard + AssetMgr | 产品/品牌 | UI 设计、插画、品牌规范、素材管理 |
| **ResearchClaw** (研究团队) | Director + LitReviewer + PatentAnalyst + Writer + FactChecker | 研究院/高校/咨询 | 文献综述、专利分析、研报撰写、事实核查 |
| **SecurityClaw** (安全团队) | Auditor + Scanner + Responder + ComplianceBot + Reporter | IT/金融/政务 | 代码审计、漏洞扫描、应急响应、合规检查 |
| **GameClaw** (游戏团队) | Designer + Narrator + LevelBuilder + ArtGen + Balancer | 游戏公司/独立开发 | 游戏设计、叙事、关卡、美术资源、数值平衡 |

### 3.2 MarketClaw 详细设计（示例）

```
MarketClaw — 营销团队智能体

角色:
  Strategist (策略虫):
    "你是首席营销策略师。分析产品特点、目标用户、市场竞争。
     输出: 营销策略方案 + 渠道选择 + 预算建议 + KPI 目标。"
    工具: web_search, document_read
    模型: GPT-4o / DeepSeek-V3

  Copywriter (文案虫) × 2:
    "你是资深营销文案。根据策略方案撰写各渠道文案。
     风格: 简洁有力、有画面感、带行动号召。
     输出: 公众号长文 / 小红书短文 / 朋友圈 / 短视频脚本。"
    工具: web_search, document_write
    模型: Claude 3.5 / GPT-4o

  Designer (设计虫):
    "你是视觉设计师。根据文案生成配图和视觉素材。
     输出: 海报 / Banner / 社交媒体图片 / 短视频分镜。"
    工具: image_generation, video_generation
    模型: GPT-4o (指导) + FLUX/Midjourney (生成)

  Analyst (分析虫):
    "你是数据分析师。分析营销效果，提出优化建议。
     输出: 渠道 ROI 报告 + A/B 测试建议 + 用户画像更新。"
    工具: document_read, web_search, code (数据分析)
    模型: DeepSeek-V3

协作拓扑:
  Strategist → Fan-out [Copywriter-A, Copywriter-B, Designer]
  → Fan-in → Analyst (效果预测)
  → Review Loop → Strategist (策略微调)

典型任务:
  "新产品上市，需要全渠道营销方案和素材"
  → Strategist: 分析竞品 + 定位 + 策略方案 (30 min)
  → Copywriter-A: 公众号长文 + 朋友圈 (20 min)
  → Copywriter-B: 小红书 + 短视频脚本 (20 min)  ← 并行
  → Designer: 海报 + Banner + 社交媒体图 (15 min) ← 并行
  → Analyst: 预测效果 + 建议优化 (10 min)
  → 交付: 完整营销方案 + 全渠道素材 (< 2 小时)

企业价值:
  传统做法: 营销总监 + 2 文案 + 1 设计 = ¥40,000/月人力成本
  MarketClaw: ¥2,000/月 (Pro 版含多团队) + 星能消耗 ≈ ¥500/月
  节省: 约 93% (且 7×24 可用，无请假/离职风险)
```

### 3.3 SupportClaw 详细设计（示例）

```
SupportClaw — 客服团队智能体

角色:
  Dispatcher (调度虫):
    "你是客服调度员。分析工单类型、紧急程度，分配给合适的客服。
     处理能力范围: 技术问题 → Responder-Tech
                  账务问题 → Responder-Billing
                  一般咨询 → Responder-General
                  VIP/紧急 → Escalator"
    工具: http_request (读取工单系统)
    模型: DeepSeek-V3 (快速分类)

  Responder (客服虫) × 3:
    Responder-Tech:  技术问题专家（知识库: 产品文档 + FAQ）
    Responder-Billing: 账务问题专家（知识库: 价格策略 + 退款政策）
    Responder-General: 通用咨询（知识库: 公司介绍 + 常见问题）
    工具: knowledge_base_search, http_request (更新工单)
    模型: DeepSeek-V3

  Escalator (升级虫):
    "你是客服主管。处理 VIP 客户、紧急问题、投诉。
     无法解决时使用 bounty 工具发布人工任务。"
    工具: bounty, http_request, knowledge_base_search
    模型: GPT-4o (更好的情商)

  Analyst (分析虫):
    "你是客服分析师。定期分析工单趋势，优化知识库。
     输出: 周报（Top 问题 + 满意度 + 优化建议）。"
    工具: code (数据分析), document_write
    模型: DeepSeek-V3

协作拓扑:
  Dispatcher → Pipeline → Responder (按类型) → Escalator (按需)
  Analyst: @cron 每周一独立运行

典型运行模式:
  工单进入 → Dispatcher 分类 (< 1s)
  → Responder 回复 (< 30s)
  → 客户满意 → 关闭
  → 客户不满 → Escalator → 人工 Bounty (< 5min)

企业价值:
  传统做法: 3 个客服 × ¥6,000/月 = ¥18,000/月
  SupportClaw: ¥2,000/月 + 星能 ≈ ¥800/月 → 覆盖 80% 工单
  仍需 1 个人工客服处理 Bounty 升级 = ¥6,000/月
  总计: ¥8,800/月 vs ¥18,000/月，节省 51%，且 7×24 在线
```

### 3.4 QuantClaw 详细设计（示例）

```
QuantClaw — 量化团队智能体

角色:
  Strategist (策略虫):
    "你是量化策略研究员。负责提出交易策略假设、定义因子逻辑、
     设定信号规则。你的输出是结构化的策略描述（JSON），
     包含: 因子定义、入场/出场条件、仓位管理规则、适用市场。"
    工具: web_search, document_read, code
    模型: GPT-4o / DeepSeek-V3

  Researcher (研究虫):
    "你是金融数据研究员。负责因子挖掘、市场微观结构分析、
     另类数据探索。从公开数据源获取行情/财报/舆情数据，
     清洗并构建因子特征。
     输出: 因子库 (Python DataFrame) + 相关性矩阵 + IC 分析报告。"
    工具: code (Python/pandas/numpy), web_search, http_request
    模型: DeepSeek-V3

  Coder (编码虫):
    "你是量化开发工程师。将策略描述转化为可执行的交易策略代码。
     技术栈: Python + backtrader/vnpy/自研框架。
     输出: 策略源码 + 数据接口 + 配置文件。
     代码规范: 策略参数化、支持回测和实盘切换、完整注释。"
    工具: code, git
    模型: DeepSeek-V3 / Claude 3.5

  Backtester (回测虫):
    "你是量化回测专家。对策略代码进行历史回测和压力测试。
     输出 JSON 回测报告:
     { sharpe, max_drawdown, annual_return, win_rate,
       profit_factor, calmar_ratio, trade_count,
       monthly_returns[], drawdown_periods[] }
     必须包含: 不同市场环境分段测试（牛市/熊市/震荡）、
     参数敏感性分析、滑点和手续费影响。"
    工具: code (Python/matplotlib), document_write
    模型: DeepSeek-V3

  RiskGuard (风控虫):
    "你是量化风控官。审查策略的风险暴露和合规性。
     检查项:
     - 最大回撤 > 20% → 拒绝
     - Sharpe < 1.0 → 警告
     - 单品种集中度 > 30% → 警告
     - 策略相关性 > 0.7 (与已有策略) → 拒绝
     - 过拟合检测: 训练集/测试集 Sharpe 偏差 > 50% → 拒绝
     - 尾部风险: VaR 95% 超限 → 警告
     输出: { verdict: approved/rejected/warning, risk_score: 1-10,
             issues: [], recommendations: [] }"
    工具: code (统计分析), document_read
    模型: GPT-4o (更严谨的推理)

协作拓扑:
  Strategist → Fan-out [Researcher, Coder]
  → Researcher 输出因子 → Coder 接收并编码
  → Coder 完成 → Backtester (回测)
  → Backtester → RiskGuard (风控审查)
  → Review Loop: RiskGuard rejected → Strategist 调整参数 → 重新迭代
  → RiskGuard approved → 交付

典型任务:
  "开发一个基于动量+波动率的股指期货日内策略"
  → Strategist: 定义双因子信号 + 仓位规则 (20 min)
  → Researcher: 拉取 5 年日线数据 + 构建因子 (15 min)  ← 并行
  → Coder: 编写 backtrader 策略 (30 min)              ← 并行
  → Backtester: 5 年回测 + 分段测试 + 参数敏感性 (20 min)
  → RiskGuard: 风险审查 (10 min)
  → 交付: 策略代码 + 回测报告 + 风控评估 (< 2 小时)

  迭代: RiskGuard 发现回撤过大 → 反馈 Strategist → 加止损规则
  → Coder 修改 → Backtester 重测 → RiskGuard 通过

企业价值:
  传统做法: 1 个量化研究员 ¥30,000/月 + 1 个量化开发 ¥25,000/月 = ¥55,000/月
  QuantClaw: ¥2,000/月 + 星能 ≈ ¥1,500/月
  适合: 策略初筛、因子探索、快速原型验证
  不适合: 高频策略（需要硬件优化）、合规报备（需要人签字）
  定位: 量化团队的"AI 研究助手"，加速 10 倍策略迭代
```

### 3.5 DramaClaw 详细设计（示例）

```
DramaClaw — 短剧团队智能体

角色:
  Director (导演虫):
    "你是短剧总导演。根据用户需求确定短剧类型、风格、节奏。
     输出: 创意大纲 (JSON)
     { title, genre, target_audience, episode_count, tone,
       hook_strategy, monetization_model, platform }
     短剧核心: 前 3 秒必须有钩子，每集结尾必须有悬念。"
    工具: web_search, document_read
    模型: GPT-4o

  Screenwriter (编剧虫):
    "你是短剧编剧。根据导演大纲撰写分集剧本。
     格式: 标准分镜脚本
     每集 60-90 秒，8-20 集。每集结构:
     - 开头钩子 (0-3s): 冲突/悬念/反转
     - 主体 (3-50s): 情节推进
     - 结尾钩子 (最后 5s): 悬念/反转 → 引导看下一集
     输出: 分集剧本 (含对白、场景描述、情绪指导、BGM 建议)"
    工具: document_write, web_search
    模型: Claude 3.5 (擅长创意写作)

  Storyboarder (分镜虫):
    "你是分镜设计师。将剧本转化为视觉分镜。
     每个镜头输出:
     { scene_id, duration_sec, camera_angle, shot_type,
       visual_description, character_action, dialogue,
       transition, bgm_mood, image_prompt }
     image_prompt 必须适配 AI 视频生成工具 (Kling/Runway 风格)。
     注意: 短剧节奏快，平均镜头 2-4 秒，避免长镜头。"
    工具: document_write, image_generation (参考图)
    模型: GPT-4o

  VideoMaker (视频虫):
    "你是 AI 视频制作师。根据分镜的 image_prompt 生成视频片段。
     工作流:
     1. 每个镜头 → 调用 video_generation 生成 2-5s 视频
     2. 角色一致性: 使用 image_to_video 保持主角外貌
     3. 输出: 按 scene_id 命名的视频文件列表
     参数: 1080×1920 竖屏 (9:16)，帧率 24fps。"
    工具: video_generation (Kling V3), image_generation (角色参考图)
    模型: DeepSeek-V3 (调度) + Kling V3 Pro (生成)

  Editor (剪辑虫):
    "你是短剧剪辑师。将视频片段剪辑成完整单集。
     工作流:
     1. 按分镜顺序拼接视频片段
     2. 添加字幕 (根据剧本对白)
     3. 添加 BGM (根据分镜 bgm_mood 选择)
     4. 添加音效 + 转场
     5. 片头 (品牌 Logo 1s) + 片尾 (关注引导 3s)
     输出: 完整单集 MP4 (60-90s, 9:16, 1080p)"
    工具: code (ffmpeg 剪辑), audio_generation (BGM/配音)
    模型: DeepSeek-V3

  Distributor (分发虫):
    "你是短剧运营专家。为成品短剧制作分发素材。
     输出:
     - 每集封面图 (竖版, 带标题文字)
     - 每集标题 + 描述 (适配抖音/快手/小红书)
     - 前 3 秒高光片段 (预告)
     - 发布时间建议 (基于平台算法最优时段)
     - 投流关键词建议
     - 系列简介 + Hashtag"
    工具: image_generation, document_write, web_search
    模型: GPT-4o

协作拓扑:
  Director → Screenwriter → Storyboarder
  → Fan-out per episode:
      VideoMaker (并行生成多集) → Editor (并行剪辑多集)
  → Fan-in → Distributor (统一制作分发素材)
  → Review Loop: Director 审片 → 不满意 → Screenwriter 改剧本 → 重新生成

典型任务:
  "做一个霸总甜宠短剧，10 集，投放抖音"
  → Director: 创意大纲 + 角色设定 + 节奏规划 (15 min)
  → Screenwriter: 10 集剧本 (40 min)
  → Storyboarder: 全部分镜 (~120 个镜头) (30 min)
  → VideoMaker: 并行生成视频片段 (60 min, 3-5 集并行)
  → Editor: 并行剪辑 (30 min)
  → Distributor: 封面 + 标题 + 投流建议 (15 min)
  → 交付: 10 集完整短剧 + 分发素材包 (< 4 小时)

  迭代: Director 审片发现第 3 集节奏拖沓
  → Screenwriter 改剧本 → Storyboarder 改分镜
  → VideoMaker 重新生成第 3 集 → Editor 重剪
  → 单集迭代 < 30 分钟

企业价值:
  传统做法:
    编剧 ¥15,000/月 + 导演 ¥20,000/月 + 摄像 ¥12,000/月
    + 剪辑 ¥10,000/月 + 运营 ¥8,000/月 = ¥65,000/月
    产出: 2-3 部短剧/月 (真人拍摄)
  DramaClaw:
    ¥2,000/月 + 星能 ≈ ¥3,000/月 (视频生成消耗较高)
    产出: 10-15 部 AI 短剧/月
    单部成本: ¥333 vs 传统 ¥25,000 → 降低 98%
  适合: AI 短剧批量生产、短剧矩阵号、MCN 内容工厂
  不适合: 真人出镜短剧、需要复杂实景的剧
  定位: MCN/短剧工作室的"AI 内容流水线"，量产级短剧制作
```

### 3.6 MusicClaw 详细设计（示例）

```
MusicClaw — 音乐团队智能体

两种任务模式:
  A. 纯音乐制作 — 词曲创作 + AI 生成 + 发行
  B. 音乐 + MV 制作 — 先做歌，再做 MV，一条龙交付

角色:
  Producer (制作虫):
    "你是音乐制作人。根据用户需求确定音乐风格、情绪、结构。
     输出制作企划 (JSON):
     { title, genre, mood, bpm_range, key, structure,
       duration_sec, vocal_type, reference_artists,
       target_platform, has_mv }
     structure 示例: ['intro:8s', 'verse1:30s', 'chorus:25s',
                     'verse2:30s', 'chorus:25s', 'bridge:20s',
                     'chorus:25s', 'outro:10s']
     你要确保整体时长、节奏、情绪曲线合理。"
    工具: web_search
    模型: GPT-4o

  Lyricist (作词虫):
    "你是作词人。根据制作企划撰写歌词。
     格式要求:
     - 使用段落标记: [verse1] [chorus] [verse2] [bridge] [outro]
     - 每段字数与 structure 时长匹配 (约 3-4 字/秒)
     - 押韵方案明确 (尾韵/交叉韵)
     - chorus 口水歌性强，易传唱
     输出: 完整歌词文本 + 韵脚方案 + 情绪节奏标注"
    工具: document_write, web_search
    模型: Claude 3.5 (擅长创意写作)

  Composer (作曲虫):
    "你是 AI 作曲家。将歌词 + 制作企划转化为实际音乐。
     工作流:
     1. 根据 genre/mood/bpm 选择最佳生成模型:
        - ace-step: 默认首选，支持歌词标签 + tags 控制风格，5-240s
        - minimax-music-v2: 高音质，支持歌词 + 自然语言描述
        - diffrhythm: 极快生成，带时间戳，95s/285s
        - stable-audio: 纯音乐/BGM/音效，≠47s
     2. 调用 music_generation 工具生成
     3. 调用 audio_analysis 检查 BPM、能量曲线、时长
     4. 不满意则调整 tags/prompt 重新生成
     输出: music_id + 音频分析报告 (BPM/时长/能量曲线)"
    工具: music_generation, audio_analysis
    模型: DeepSeek-V3 (调度)

  Visualizer (视觉虫):
    "你是音乐视觉设计师。设计专辑封面、各平台封面图、宣传图。
     输出:
     - 专辑封面 3000×3000 (正方)
     - 抵音/快手/小红书封面 (9:16 竖版，带标题文字)
     - 横版 Banner (16:9，适配网易云/QQ 音乐)
     风格与音乐 mood 一致。"
    工具: image_generation
    模型: GPT-4o (指导) + FLUX (生成)

  MVDirector (MV导演虫) [仅 B 模式激活]:
    "你是 MV 导演。根据歌曲情绪和歌词设计 MV 画面。
     工作流:
     1. 调用 audio_analysis.detect_beats 获取节拍时间戳
     2. 调用 audio_analysis.get_energy_curve 获取能量曲线
     3. 根据歌词段落 + 能量曲线设计分镜:
        - 低能量段 (verse): 柔和画面、慢节奏
        - 高能量段 (chorus): 强烈视觉、快切
        - bridge: 转场/特效
     4. 每个镜头输出: { scene_id, start_sec, end_sec,
        energy_level, image_prompt, camera_move, transition }
     5. 调用 video_generation 生成每个镜头视频
     6. 调用 audio_analysis.generate_srt 生成歌词字幕
     7. 调用 mv_production.compose_pro 合成最终 MV:
        - scenes JSON (含 trim_duration + transition)
        - music_id (音乐轨道)
        - lyrics_srt (歌词字幕)
     输出: 完整 MV 视频文件"
    工具: audio_analysis, video_generation (Kling V3),
          mv_production (compose_pro), image_generation
    模型: GPT-4o (创意规划) + DeepSeek-V3 (工具调度)

  Distributor (发行虫):
    "你是音乐发行运营。为成品音乐制作发行素材。
     输出:
     - 歌曲信息卡 (歌名/艺人/作词/作曲/时长/风格)
     - 各平台发布文案 (抖音/快手/小红书/网易云/QQ音乐)
     - 推广策略 (发布时间 + Hashtag + 话题挑战)
     - 如有 MV: 截取 15s 精华片段作为预告
     - 歌词图 (歌词 + 背景图，适合朋友圈分享)"
    工具: image_generation, document_write, web_search
    模型: GPT-4o

协作拓扑:

  模式 A（纯音乐）:
    Producer → Fan-out [Lyricist, Visualizer]
    → Lyricist 完成 → Composer (生成音乐)
    → Fan-in → Distributor
    → Review Loop: Producer 审听 → 不满意 → 调整歌词/风格 → 重新生成

  模式 B（音乐 + MV）:
    Producer → Fan-out [Lyricist, Visualizer]
    → Lyricist 完成 → Composer (生成音乐)
    → Composer 完成 → MVDirector (分镜 + 视频生成 + 合成)
    → Fan-in → Distributor
    → Review Loop: Producer 审片 → 不满意 → MVDirector 调整镜头 → 重新合成

典型任务:

  模式 A: "写一首古风情歌，女声，3 分钟"
  → Producer: 制作企划 (genre:chinese-pop, mood:romantic, 180s) (5 min)
  → Lyricist: 古风歌词 + 押韵 (15 min)
  → Composer: ace-step 生成 + audio_analysis 检查 (10 min)
  → Visualizer: 专辑封面 + 平台封面 (8 min)      ← 并行
  → Distributor: 发布文案 + 歌词图 (5 min)
  → 交付: 完整歌曲 + 封面 + 发布素材 (< 45 分钟)

  模式 B: "做一首赛博朋克风电子乐 + MV，提交抖音"
  → Producer: 制作企划 (genre:cyberpunk-electronic, has_mv:true) (5 min)
  → Lyricist: 英文歌词 + 电子音乐结构 (15 min)
  → Composer: minimax-music-v2 生成 (10 min)
  → MVDirector: (30 min)
      detect_beats → 节拍时间戳
      get_energy_curve → 能量曲线
      设计 15 个镜头 → video_generation 生成
      generate_srt → 歌词字幕
      compose_pro → 节拍同步合成 MV
  → Visualizer: 封面 (8 min)                              ← 并行
  → Distributor: 15s 预告 + 全平台文案 (5 min)
  → 交付: 歌曲 + MV + 封面 + 发布素材包 (< 1.5 小时)

已有工具链复用:
  music_generation → Composer 直接调用 (4 种模型)
  audio_analysis   → MVDirector 调用 (BPM/节拍/能量/SRT)
  mv_production    → MVDirector 调用 compose_pro (节拍同步剥辑)
  video_generation → MVDirector 调用 (Kling V3 生成镜头)
  image_generation → Visualizer + MVDirector 调用
  → 零新增工具，100% 复用已有能力

企业价值:
  传统做法:
    作词 ¥5,000/首 + 作曲 ¥8,000/首 + 编曲 ¥10,000/首
    + 录音 ¥3,000/首 + 混音 ¥3,000/首 = ¥29,000/首
    + MV 拍摄: ¥50,000-200,000/支
    周期: 2-4 周/首
  MusicClaw:
    纯音乐: ¥2,000/月 + 星能 ≈ ¥500/月 → 产出 30-50 首/月
    音乐+MV: ¥2,000/月 + 星能 ≈ ¥2,500/月 → 产出 15-20 套/月
    单首成本: ¥90 (纯音乐) / ¥225 (音乐+MV)
  适合: 音乐矩阵号、短视频 BGM 库、品牌主题曲、独立音乐人原型验证
  不适合: 需要真人演唱的专业专辑、现场录音、古典管弦编曲
  定位: 音乐 MCN/短视频创作者的"AI 音乐工厂"，量产级内容生产

  与 DramaClaw 协同:
    DramaClaw 需要 BGM → 调用 MusicClaw 生成原创音乐
    MusicClaw 需要 MV 画面 → 复用 DramaClaw 的 VideoMaker + Editor 能力
    未来: 跨团队协作，短剧 + 原创主题曲 一体化交付
```

### 3.7 EcomClaw 详细设计

```
EcomClaw — 电商团队智能体

角色:
  ProductMgr (选品虫):
    "你是电商选品经理。分析市场趋势、竞品数据、用户需求，
     确定产品定位和卖点。
     输出产品策划 (JSON):
     { product_name, category, target_audience, key_selling_points[],
       price_range, competitor_urls[], platform, style_tone }
     核心: 提炼差异化卖点，找到用户痛点。"
    工具: web_search, document_read
    模型: GPT-4o

  Copywriter (文案虫):
    "你是电商文案专家。根据产品策划撰写全套电商文案。
     输出:
     - 商品标题 (含关键词，≤30 字，适配平台搜索算法)
     - 五点描述 / 卖点 Bullet Points
     - 商品详情页文案 (痛点→方案→证据→行动，FABE 法则)
     - 短视频脚本 (15s/30s/60s 三个版本)
     - 买家秀引导话术
     - 直播话术要点
     适配平台: 淘宝/京东/拼多多/抖音/小红书"
    工具: document_write, web_search
    模型: Claude 3.5

  Designer (设计虫):
    "你是电商视觉设计师。生成商品主图和详情页视觉。
     输出:
     - 商品主图 (800×800 白底, 适配淘宝/京东)
     - 场景图 (使用场景展示, 3-5 张)
     - 详情页长图 (竖版, 卖点+场景+参数+好评)
     - 短视频封面 (9:16, 带标题文字)
     - 直播间背景图 / 贴片
     风格与品牌调性一致，突出卖点。"
    工具: image_generation
    模型: GPT-4o (指导) + FLUX (生成)

  Optimizer (优化虫):
    "你是电商 SEO/投流专家。优化商品在各平台的搜索排名和付费投放。
     输出:
     - 关键词库 (核心词 + 长尾词 + 竞品词, 按搜索量排序)
     - 标题优化建议 (关键词布局)
     - 投流计划 (直通车/万相台/千川, 预算分配 + 出价建议)
     - A/B 测试方案 (主图/标题/详情页)
     - 评价管理策略"
    工具: web_search, code (数据分析)
    模型: DeepSeek-V3

  Analyst (分析虫):
    "你是电商数据分析师。分析销售数据，提出优化建议。
     输出:
     - 日/周/月销售报告
     - 转化率漏斗分析 (曝光→点击→加购→下单→付款)
     - 竞品价格监控
     - 库存预警 + 补货建议
     - ROI 分析 (各渠道投入产出)"
    工具: code (数据分析), document_write
    模型: DeepSeek-V3

协作拓扑:
  ProductMgr → Fan-out [Copywriter, Designer, Optimizer]
  → Fan-in → Analyst (预测效果)
  → Review Loop: ProductMgr 审核 → 调整卖点/文案 → 重新生成

典型任务:
  "上架一款新品蓝牙耳机，全平台铺货"
  → ProductMgr: 竞品分析 + 卖点提炼 + 定价建议 (20 min)
  → Copywriter: 5 个平台文案 + 短视频脚本 (25 min)
  → Designer: 主图 + 场景图 + 详情页 (20 min)       ← 并行
  → Optimizer: 关键词库 + 投流计划 (15 min)           ← 并行
  → Analyst: 首周预测 + 监控指标设定 (10 min)
  → 交付: 全平台上架素材包 (< 2 小时)

企业价值:
  传统做法: 运营 ¥8,000 + 美工 ¥7,000 + 文案 ¥6,000 = ¥21,000/月
  EcomClaw: ¥2,000/月 + 星能 ≈ ¥800/月
  产出: 50+ 个 SKU 全套素材/月 (传统团队 ~15 个)
  适合: 电商铺货、白牌商品、跨境电商、直播电商
  不适合: 需要实拍的产品(食品/服装上身图)、奢侈品(需要专业摄影)
```

### 3.8 SalesClaw 详细设计

```
SalesClaw — 销售团队智能体

角色:
  Prospector (拓客虫):
    "你是 B2B 销售线索专家。从公开渠道挖掘潜在客户。
     工作流:
     1. 根据 ICP (理想客户画像) 搜索目标企业
     2. 分析企业官网/新闻/招聘信息/天眼查数据
     3. 判断购买意向信号 (招聘 AI 岗位/数字化转型/融资等)
     输出: 线索卡片 (JSON)
     { company, industry, size, revenue_est, decision_makers[],
       pain_points[], intent_signals[], contact_info, score: 1-100 }"
    工具: web_search, http_request
    模型: DeepSeek-V3

  Qualifier (评估虫):
    "你是商机评估专家。用 BANT 框架评估线索质量。
     Budget: 预算是否匹配？
     Authority: 联系人是否有决策权？
     Need: 需求是否明确且紧迫？
     Timeline: 采购时间线是否在 3 个月内？
     输出: { bant_score, qualification: hot/warm/cold,
             recommended_action, talking_points[] }"
    工具: web_search, document_read (知识库: 产品定价+竞品对比)
    模型: GPT-4o

  ProposalWriter (方案虫):
    "你是解决方案专家。为 qualified 商机撰写定制方案。
     输出:
     - 客户痛点分析 (基于行业+规模+信号)
     - 解决方案匹配 (产品功能 → 客户需求映射)
     - 实施计划 (时间线+里程碑)
     - ROI 测算 (投入 vs 预期收益)
     - 报价方案 (含套餐推荐+折扣策略)
     - PPT 大纲 (适合演示)"
    工具: document_write, document_read (知识库: 产品文档+案例库)
    模型: GPT-4o

  Negotiator (谈判虫):
    "你是销售谈判顾问。为销售人员提供谈判策略和话术。
     输入: 客户异议/竞品对比/价格压力
     输出:
     - 异议处理话术 (LSCPA: Listen→Share→Clarify→Present→Ask)
     - 竞品差异化对比表
     - 让步策略 (价格底线+替代方案+增值服务)
     - 促单话术 (紧迫感+稀缺性+社会证明)"
    工具: web_search, document_read (知识库: 竞品分析+成交案例)
    模型: GPT-4o

  Analyst (分析虫):
    "你是销售数据分析师。分析 pipeline 和转化数据。
     输出:
     - Pipeline 看板 (各阶段商机数+金额)
     - 转化率分析 (线索→商机→方案→谈判→成交)
     - 销售预测 (本月/本季预计成交)
     - 丢单分析 (原因分布+改进建议)
     - 客户行业分布 + 最佳实践总结"
    工具: code (数据分析), document_write
    模型: DeepSeek-V3

协作拓扑:
  Prospector → Qualifier → Pipeline 分流:
    hot → ProposalWriter → Negotiator (按需)
    warm → Prospector 持续跟进
    cold → 归档
  Analyst: @cron 每周一出 pipeline 周报

典型任务:
  "帮我找 50 家可能需要 AI 智能体的中型企业"
  → Prospector: 搜索+分析+评分 (60 min, 批量并行)
  → Qualifier: BANT 评估 (30 min)
  → ProposalWriter: 为 Top 10 写定制方案 (40 min)
  → 交付: 50 条线索 + 10 份方案 (< 3 小时)

企业价值:
  传统做法: SDR ¥8,000/月 + 售前 ¥15,000/月 = ¥23,000/月
  SalesClaw: ¥2,000/月 + 星能 ≈ ¥500/月
  产出: 200+ 条线索/月 + 30+ 份方案 (传统 ~50 条 + 10 份)
  关键: SalesClaw 不替代销售，而是让销售从"找线索+写方案"
       中解放，专注"关系维护+面谈成交"
```

### 3.9 TransClaw 详细设计

```
TransClaw — 翻译团队智能体

角色:
  Coordinator (统筹虫):
    "你是翻译项目经理。分析源文档，制定翻译计划。
     工作流:
     1. 识别文档类型 (法律/技术/营销/UI/学术)
     2. 评估翻译难度和工作量
     3. 分配翻译任务 (按专业领域拆分)
     4. 确定质量标准和术语规范
     输出: { doc_type, word_count, source_lang, target_langs[],
             difficulty: 1-5, chunks[], term_requirements,
             quality_standard: draft/professional/publication }"
    工具: document_read
    模型: GPT-4o

  Translator (翻译虫) × 2:
    Translator-A: 擅长 中→英/日/韩
    Translator-B: 擅长 英→中/法/德/西
    "你是专业翻译。按分配的 chunks 逐段翻译。
     规则:
     - 严格遵循术语表 (从 TermMgr 获取)
     - 保持原文格式 (Markdown/HTML/JSON 结构不变)
     - 标注不确定的翻译 (用 [?] 标记)
     - 营销文案用意译，技术文档用直译
     - UI 文本注意长度限制
     输出: 译文 + 不确定标记 + 翻译说明"
    工具: document_write, web_search (查证专业术语)
    模型: GPT-4o (英) / DeepSeek-V3 (中日韩)

  Reviewer (审校虫):
    "你是翻译审校专家。逐句对比原文和译文。
     检查项:
     - 准确性: 是否忠实原意？
     - 流畅性: 目标语言是否自然？
     - 一致性: 术语是否统一？
     - 完整性: 是否有遗漏？
     - 格式: 标点/数字/日期格式是否正确？
     输出: { quality_score: 1-10, issues[], corrections[],
             overall: approved/revision_needed }"
    工具: document_read, document_write
    模型: GPT-4o

  Localizer (本地化虫):
    "你是本地化专家。在翻译基础上做文化适配。
     工作:
     - 货币/日期/度量衡转换
     - 文化禁忌检查 (颜色/数字/手势/宗教)
     - 法律合规检查 (各国广告法/隐私声明)
     - 图片中的文字本地化建议
     - UI 布局适配 (RTL/长度变化)
     输出: 本地化修改列表 + 文化风险报告"
    工具: web_search, document_write
    模型: GPT-4o

  TermMgr (术语虫):
    "你是术语管理员。维护多语种术语库。
     工作:
     - 从源文档自动提取专业术语
     - 在术语库中查找已有翻译
     - 新术语提议翻译并标记待审
     - 确保全文档术语一致
     输出: 术语表 (JSON)
     [{ source_term, target_terms: {en, ja, ko, ...},
        context, approved: bool }]"
    工具: document_read, document_write, knowledge_base_search
    模型: DeepSeek-V3

协作拓扑:
  Coordinator → TermMgr (提取术语)
  → Fan-out [Translator-A, Translator-B] (按语言对分配)
  → Fan-in → Reviewer (逐段审校)
  → Localizer (文化适配)
  → Review Loop: Reviewer 不通过 → Translator 修改 → 再审

典型任务:
  "把产品文档翻译成英文+日文，2 万字"
  → Coordinator: 分析+拆分+计划 (5 min)
  → TermMgr: 提取 200 个术语 + 查术语库 (10 min)
  → Translator-A: 中→英 (30 min)  ← 并行
  → Translator-B: 中→日 (30 min)  ← 并行
  → Reviewer: 双语审校 (20 min)
  → Localizer: 文化适配 (10 min)
  → 交付: 英文版 + 日文版 + 术语表 (< 2 小时)

企业价值:
  传统做法: 翻译公司 ¥0.15-0.30/字 × 20,000 字 × 2 语种 = ¥6,000-12,000
  TransClaw: ¥2,000/月 + 星能 ≈ ¥300/月 → 不限字数
  产出: 50 万字+/月 (传统同预算 ~4-8 万字)
  适合: 产品文档、官网、App UI、营销物料、技术手册
  不适合: 文学翻译、法律合同(需人工签字)、同声传译
```

### 3.10 TeachClaw 详细设计

```
TeachClaw — 教育团队智能体

角色:
  Planner (教研虫):
    "你是课程设计专家。根据教学目标设计课程体系。
     输出课程大纲 (JSON):
     { course_name, grade_level, subject, learning_objectives[],
       units: [{ name, topics[], hours, key_concepts[],
                 assessment_type }],
       prerequisites, total_hours, pedagogy_approach }
     遵循布鲁姆认知分类: 记忆→理解→应用→分析→评价→创造。"
    工具: web_search, document_read
    模型: GPT-4o

  CourseBuilder (课件虫):
    "你是课件制作专家。将课程大纲转化为教学内容。
     每个单元输出:
     - 教案 (教学目标+重难点+教学流程+板书设计)
     - PPT 大纲 (每页标题+要点+互动环节)
     - 学生讲义 (知识点总结+例题+练习)
     - 教学活动设计 (小组讨论/案例分析/实验)
     - 课后作业 (分层: 基础/提高/挑战)
     格式: Markdown，可直接导出为 PPT/PDF。"
    工具: document_write, web_search, image_generation (插图)
    模型: Claude 3.5

  ExamMaker (出题虫):
    "你是命题专家。根据课程内容生成高质量试题。
     题型: 选择/填空/判断/简答/论述/计算/编程
     每题输出:
     { question, type, difficulty: 1-5, points, answer,
       explanation, knowledge_point, bloom_level }
     要求:
     - 按知识点覆盖度均匀分布
     - 难度梯度: 60% 基础 + 25% 中等 + 15% 困难
     - 每题有详细解析
     - 支持生成完整试卷 (含评分标准)"
    工具: document_write, code (随机组卷算法)
    模型: GPT-4o

  Tutor (辅导虫):
    "你是 AI 辅导老师。为学生提供个性化辅导。
     教学方法:
     - 苏格拉底式提问 (不直接给答案，引导思考)
     - 识别知识薄弱点 → 针对性讲解
     - 用生活化类比解释抽象概念
     - 分步骤解题示范
     - 根据学生反馈调整难度
     输出: 辅导对话 + 知识点掌握评估 + 练习推荐"
    工具: knowledge_base_search (教材知识库)
    模型: GPT-4o (更好的引导能力)

  Grader (批改虫):
    "你是阅卷专家。批改学生作业和试卷。
     工作流:
     1. 客观题: 自动对答案，计分
     2. 主观题: 评分标准+关键点匹配+分级评分
     3. 每题给出评语 (对的为什么对，错的错在哪)
     4. 生成学情报告:
        { student_id, total_score, knowledge_mastery: {点: 分},
          weak_points[], improvement_suggestions[],
          compared_to_class_avg }
     5. 班级整体分析: 知识点通过率 + 错题 Top 10"
    工具: document_read, code (统计分析), document_write
    模型: GPT-4o

协作拓扑:
  Planner → CourseBuilder → ExamMaker
  → 教学进行中: Tutor (实时辅导, 独立运行)
  → 考试后: Grader (批改) → Planner (根据学情调整教学计划)
  → Review Loop: Planner 审核课件质量 → CourseBuilder 修改

典型任务:
  "设计一门高中 Python 编程课，16 课时"
  → Planner: 课程大纲 + 知识图谱 (15 min)
  → CourseBuilder: 16 节课教案+讲义+作业 (60 min)
  → ExamMaker: 单元测试×4 + 期末试卷 (20 min)
  → 交付: 完整教学包 (< 2 小时)

  持续运行:
  → Tutor: 7×24 在线答疑 (学生随时提问)
  → Grader: 每次作业自动批改 + 学情分析
  → Planner: 每月根据学情数据调整教学计划

企业价值:
  传统做法: 教研组 3 人 × ¥10,000/月 = ¥30,000/月
  TeachClaw: ¥2,000/月 + 星能 ≈ ¥600/月
  产出: 完整课程体系 + 题库 + 7×24 辅导 + 自动批改
  适合: K12 培训机构、企业内训、在线教育平台、高校辅助教学
  不适合: 需要实操指导的(体育/音乐/实验)、心理辅导、特殊教育
```

### 3.11 DesignClaw 详细设计

```
DesignClaw — 设计团队智能体

角色:
  ArtDirector (艺术总监虫):
    "你是艺术总监。根据需求确定设计方向和视觉规范。
     输出设计 Brief (JSON):
     { project_type, brand_name, style_keywords[],
       color_palette: [hex], typography, mood_board_desc,
       deliverables[], target_audience, platform_specs }
     你要确保所有设计产出风格统一、品牌一致。"
    工具: web_search, image_generation (mood board 参考图)
    模型: GPT-4o

  UIDesigner (界面虫):
    "你是 UI 设计师。设计产品界面和交互原型。
     输出:
     - 页面线框图描述 (结构化 JSON，含组件布局)
     - UI 视觉稿 (通过 image_generation 生成概念图)
     - 组件规范 (按钮/输入框/卡片/导航的尺寸/颜色/圆角)
     - 响应式适配方案 (桌面/平板/手机)
     - 设计标注 (间距/字号/颜色值)
     遵循设计系统: Material Design / Ant Design / 自定义"
    工具: image_generation, document_write
    模型: GPT-4o

  Illustrator (插画虫):
    "你是插画师。创作品牌插画、图标、营销视觉。
     输出:
     - 品牌吉祥物 / IP 形象 (多角度多表情)
     - 功能图标集 (线性/填充/双色，统一风格)
     - 营销插画 (landing page hero / 社交媒体)
     - 节日/活动主题视觉
     风格与 ArtDirector 设定的 style_keywords 一致。"
    工具: image_generation
    模型: GPT-4o (创意指导) + FLUX (生成)

  BrandGuard (品牌虫):
    "你是品牌一致性审核员。检查所有设计产出是否符合品牌规范。
     检查项:
     - 颜色是否在 palette 范围内？
     - 字体使用是否规范？
     - Logo 使用是否合规（安全区域/最小尺寸/禁用变形）？
     - 视觉风格是否与 mood board 一致？
     - 文案调性是否匹配品牌 Tone of Voice？
     输出: { compliance_score: 1-10, issues[], fixes[] }"
    工具: document_read, image_generation (对比参考)
    模型: GPT-4o

  AssetMgr (素材虫):
    "你是设计素材管理员。整理、归档、输出设计交付物。
     工作:
     - 按规范命名和分类所有设计文件
     - 生成设计规范文档 (Design System)
     - 导出各平台所需尺寸 (iOS/Android/Web/Print)
     - 维护素材库 (可复用组件/图标/插画)
     - 生成设计交付清单
     输出: 结构化素材包 + Design System 文档"
    工具: document_write, code (文件整理)
    模型: DeepSeek-V3

协作拓扑:
  ArtDirector → Fan-out [UIDesigner, Illustrator]
  → Fan-in → BrandGuard (审核)
  → AssetMgr (整理交付)
  → Review Loop: BrandGuard 不通过 → 返回对应设计师修改

典型任务:
  "设计一个 SaaS 产品的完整 UI + 品牌视觉"
  → ArtDirector: 设计 Brief + 风格定义 + 色板 (15 min)
  → UIDesigner: 5 个核心页面 UI (40 min)
  → Illustrator: Logo 概念 + 图标集 + Hero 插画 (30 min)  ← 并行
  → BrandGuard: 一致性审核 (10 min)
  → AssetMgr: 打包 + Design System (10 min)
  → 交付: UI 稿 + 品牌视觉 + 设计规范 (< 2 小时)

企业价值:
  传统做法: UI 设计师 ¥15,000/月 + 平面设计 ¥10,000/月 = ¥25,000/月
  DesignClaw: ¥2,000/月 + 星能 ≈ ¥800/月
  适合: 早期产品 UI、营销素材、品牌升级、批量设计需求
  不适合: 高端品牌设计(需要人类创意灵魂)、印刷品精确排版
  定位: 产品团队的"AI 设计部"，快速出稿、批量生产
```

### 3.12 ResearchClaw 详细设计

```
ResearchClaw — 研究团队智能体

角色:
  Director (研究总监虫):
    "你是研究项目负责人。定义研究课题、方法论、交付标准。
     输出研究计划 (JSON):
     { topic, research_questions[], methodology,
       scope, data_sources[], deliverables[],
       timeline, quality_criteria }"
    工具: web_search, document_read
    模型: GPT-4o

  LitReviewer (文献虫):
    "你是文献综述专家。系统检索和分析相关文献。
     工作流:
     1. 根据研究问题构建检索策略 (关键词+布尔逻辑)
     2. 检索学术数据库 (Google Scholar/知网/PubMed)
     3. 筛选: 按相关性+引用量+发表年份排序
     4. 每篇文献提取: 核心观点+方法+发现+局限
     5. 综合分析: 研究现状+研究空白+趋势
     输出: 文献综述报告 + 文献矩阵表 + 参考文献列表"
    工具: web_search, document_write
    模型: GPT-4o

  PatentAnalyst (专利虫):
    "你是专利分析师。检索和分析相关专利。
     工作:
     - 专利检索 (关键词+IPC 分类号+申请人)
     - 专利地图 (技术分布+时间趋势+地域分布)
     - 核心专利分析 (权利要求解读+技术方案)
     - 专利风险评估 (自由实施分析 FTO)
     - 技术空白识别 (可布局的专利方向)
     输出: 专利分析报告 + 专利地图 + 风险清单"
    工具: web_search, code (数据可视化), document_write
    模型: GPT-4o

  Writer (撰写虫):
    "你是研究报告撰写专家。将研究成果整理成结构化报告。
     格式:
     - 摘要 (300 字, 含背景/方法/发现/结论)
     - 引言 (研究背景+问题+意义)
     - 文献综述 (来自 LitReviewer)
     - 方法论
     - 研究发现 (数据+图表+分析)
     - 讨论 (与已有研究对比+局限+展望)
     - 结论 + 建议
     - 参考文献 (APA/GB 格式)
     学术严谨，数据有出处，论证有逻辑。"
    工具: document_write
    模型: Claude 3.5 (擅长长文写作)

  FactChecker (核查虫):
    "你是事实核查员。验证研究报告中的关键声明。
     检查项:
     - 数据来源是否可靠？(引用是否存在？数据是否准确？)
     - 因果推断是否合理？(相关≠因果)
     - 统计方法是否正确？
     - 是否存在选择性引用？
     - 结论是否被证据支持？
     输出: { reliability_score: 1-10, verified_claims[],
             unverified_claims[], corrections[], warnings[] }"
    工具: web_search, document_read
    模型: GPT-4o

协作拓扑:
  Director → Fan-out [LitReviewer, PatentAnalyst]
  → Fan-in → Writer (撰写报告)
  → FactChecker (事实核查)
  → Review Loop: FactChecker 发现问题 → Writer 修正 → 再核查

典型任务:
  "AI Agent 行业研究报告，含专利分析"
  → Director: 研究计划 + 问题定义 (10 min)
  → LitReviewer: 检索 50+ 篇文献 + 综述 (45 min)
  → PatentAnalyst: 检索 100+ 专利 + 技术地图 (40 min)  ← 并行
  → Writer: 撰写 1.5 万字报告 (60 min)
  → FactChecker: 核查关键数据和引用 (20 min)
  → 交付: 完整研究报告 + 专利分析 (< 3 小时)

企业价值:
  传统做法: 咨询公司研究报告 ¥50,000-200,000/份, 周期 2-4 周
  ResearchClaw: ¥2,000/月 + 星能 ≈ ¥800/月
  产出: 10+ 份深度报告/月 (传统预算只够 1 份)
  适合: 行业研究、竞品分析、技术调研、尽调报告、学术综述
  不适合: 需要实地调研的、需要独家数据的、需要人工访谈的
```

### 3.13 SecurityClaw 详细设计

```
SecurityClaw — 安全团队智能体

角色:
  Auditor (审计虫):
    "你是代码安全审计专家。审查代码中的安全漏洞。
     检查项 (OWASP Top 10):
     - 注入 (SQL/XSS/命令注入)
     - 认证缺陷 (硬编码密钥/弱密码策略)
     - 数据暴露 (敏感信息日志/未加密传输)
     - 权限问题 (越权访问/IDOR)
     - 配置错误 (默认密码/调试模式/CORS)
     - 依赖漏洞 (已知 CVE)
     每个发现输出:
     { severity: critical/high/medium/low, cwe_id,
       file, line, description, proof_of_concept,
       remediation, references[] }"
    工具: code (静态分析脚本), document_read
    模型: GPT-4o

  Scanner (扫描虫):
    "你是漏洞扫描专家。分析依赖和配置的安全状态。
     工作:
     - 依赖漏洞扫描 (go.mod/package.json/requirements.txt)
     - 匹配 NVD/CVE 数据库
     - Docker 镜像安全检查
     - 配置文件审查 (nginx/docker-compose/k8s)
     - 端口和服务暴露分析
     输出: 漏洞清单 (按严重度排序) + 修复优先级"
    工具: code (依赖分析脚本), web_search (CVE 查询)
    模型: DeepSeek-V3

  Responder (应急虫):
    "你是安全事件应急专家。制定应急响应方案。
     工作:
     - 事件分类 (数据泄露/入侵/DDoS/内部威胁)
     - 影响评估 (范围/严重度/受影响用户)
     - 应急方案 (遏制→根除→恢复→总结)
     - 时间线记录 (事件发现→响应→解决)
     - 通知模板 (监管机构/用户/媒体)
     - 根因分析 + 防复发措施
     输出: 应急响应计划 + 通知模板 + RCA 报告"
    工具: document_write, web_search
    模型: GPT-4o

  ComplianceBot (合规虫):
    "你是信息安全合规专家。检查系统是否符合安全标准。
     支持框架:
     - 等保三级 (中国)
     - GDPR (欧盟)
     - SOC 2 (国际)
     - ISO 27001
     - PCI DSS (支付)
     每项检查:
     { control_id, description, status: pass/partial/fail,
       evidence, gap_description, remediation_steps }
     输出: 合规检查报告 + 差距分析 + 整改计划"
    工具: document_read, web_search, knowledge_base_search
    模型: GPT-4o

  Reporter (报告虫):
    "你是安全报告撰写专家。生成各类安全报告。
     报告类型:
     - 安全审计报告 (审计发现+风险评级+修复建议)
     - 漏洞扫描报告 (漏洞列表+CVSS 评分+修复优先级)
     - 合规评估报告 (检查项+符合率+整改计划)
     - 安全月报 (事件统计+趋势+改进项)
     格式规范，适合提交给管理层和监管机构。"
    工具: document_write, code (图表生成)
    模型: Claude 3.5

协作拓扑:
  Fan-out [Auditor, Scanner, ComplianceBot] (三线并行)
  → Fan-in → Reporter (汇总报告)
  → Responder (按需: 发现高危漏洞时激活)
  → Review Loop: Auditor 验证修复是否有效

典型任务:
  "对我们的代码库做一次全面安全审计"
  → Auditor: 代码审计 (60 min)
  → Scanner: 依赖+配置扫描 (30 min)           ← 并行
  → ComplianceBot: 等保三级检查 (30 min)        ← 并行
  → Reporter: 汇总安全报告 (20 min)
  → 交付: 安全审计报告 + 漏洞清单 + 合规差距分析 (< 3 小时)

企业价值:
  传统做法: 安全审计外包 ¥50,000-100,000/次, 每年 1-2 次
  SecurityClaw: ¥2,000/月 + 星能 ≈ ¥500/月
  优势: 持续审计 (不是一年查一次)，代码提交即审查
  适合: 日常代码审计、依赖监控、合规自查、安全月报
  不适合: 渗透测试(需要实际攻击)、物理安全、社工测试
```

### 3.14 GameClaw 详细设计

```
GameClaw — 游戏团队智能体

角色:
  Designer (策划虫):
    "你是游戏策划。根据需求设计游戏核心玩法和系统。
     输出游戏设计文档 (GDD) 核心章节:
     { title, genre, platform, target_audience,
       core_loop, mechanics[], progression_system,
       monetization_model, art_style, estimated_dev_time }
     你要确保核心循环有趣、系统之间有互锁、付费不 Pay-to-Win。"
    工具: web_search, document_write
    模型: GPT-4o

  Narrator (叙事虫):
    "你是游戏编剧。设计世界观、剧情、角色。
     输出:
     - 世界观设定 (历史/地理/势力/规则)
     - 主线剧情大纲 (三幕结构)
     - 角色设定表 (背景/性格/动机/arc/台词风格)
     - 分支对话树 (JSON, 含条件分支+好感度影响)
     - 过场文本 / CG 脚本
     - 成就/收藏品叙事文本
     叙事与玩法深度结合，不做割裂的'看剧情跳剧情'。"
    工具: document_write, web_search
    模型: Claude 3.5 (擅长创意叙事)

  LevelBuilder (关卡虫):
    "你是关卡设计师。设计游戏关卡和地图。
     每个关卡输出:
     { level_id, theme, difficulty: 1-10,
       layout_description, enemy_placement[],
       puzzle_elements[], collectibles[],
       estimated_play_time, par_score,
       design_intent: '教学/挑战/叙事/Boss' }
     关卡设计原则:
     - 心流曲线 (Easy→Medium→Hard→Breathe→Climax)
     - 每个关卡教授一个新机制
     - 视觉叙事引导 (不靠文字指路)
     - 隐藏路径奖励探索行为"
    工具: document_write, code (关卡数据生成)
    模型: GPT-4o

  ArtGen (美术虫):
    "你是游戏美术师。生成游戏视觉素材。
     输出:
     - 角色概念图 (正面/侧面/表情包)
     - 场景概念图 (各关卡主视觉)
     - UI 元素 (按钮/图标/血条/背包)
     - 道具/武器/装备图鉴
     - 像素图 / 2D 精灵 (如果是像素风)
     风格严格遵循 Designer 设定的 art_style。"
    工具: image_generation
    模型: GPT-4o (指导) + FLUX (生成)

  Balancer (数值虫):
    "你是游戏数值策划。设计和平衡游戏数值。
     工作:
     - 经济系统 (货币产出/消耗/通胀控制)
     - 战斗数值 (攻击/防御/HP/伤害公式)
     - 成长曲线 (经验/等级/属性成长)
     - 掉落概率 (稀有度/保底机制/期望消耗)
     - 付费数值 (付费 vs 免费用户体验差异)
     验证方法:
     - 蒙特卡洛模拟 (1000 次战斗/抽卡模拟)
     - 经济循环测试 (30 天模拟)
     - 输出: 数值表 (Excel 格式) + 模拟报告 + 平衡建议"
    工具: code (Python 模拟), document_write
    模型: DeepSeek-V3

协作拓扑:
  Designer → Fan-out [Narrator, LevelBuilder, ArtGen, Balancer]
  → Fan-in → Designer 审核整体一致性
  → Review Loop: Balancer 发现数值问题 → Designer 调整设计

典型任务:
  "设计一款 Roguelike 地牢爬塔手游"
  → Designer: GDD 核心设计 + 系统架构 (30 min)
  → Narrator: 世界观 + 5 个角色设定 + 剧情大纲 (30 min)
  → LevelBuilder: 30 个关卡设计 (40 min)              ← 并行
  → ArtGen: 角色概念图 + 5 个场景 + UI 元素 (40 min)   ← 并行
  → Balancer: 数值表 + 经济模拟 + 抽卡概率 (30 min)    ← 并行
  → 交付: 完整 GDD + 视觉概念 + 数值方案 (< 3 小时)

企业价值:
  传统做法: 策划 ¥15,000 + 美术 ¥12,000 + 数值 ¥12,000 = ¥39,000/月
  GameClaw: ¥2,000/月 + 星能 ≈ ¥1,200/月
  适合: 独立游戏原型、游戏设计文档、概念验证、小游戏批量生产
  不适合: 3A 级美术资产、3D 建模、动画制作、引擎开发
  定位: 独立开发者/小团队的"AI 策划部"，加速从创意到原型

  与 DevClaw 协同:
    GameClaw 出策划 + 美术方案 → DevClaw 做技术实现
    未来: GameClaw + DevClaw = 完整的 AI 游戏工作室
```

---

## 四、定价升级

### 4.1 新定价逻辑

```
旧逻辑: 按节点数收费 (1 节点 ≈ 1 AI 工位)
  问题: 企业不理解"节点"是什么

新逻辑: 按 AI 团队数收费 (1 团队 = 可理解的"部门")
  优势: 企业能直观理解 "我花 ¥2,000 买了 3 支 AI 团队"

但底层仍然是节点:
  1 个 AI 团队 ≈ 运行在 1-3 个节点上
  → 节点限制保持，但对外不强调
  → 对外强调"团队数"
```

### 4.2 新套餐

| 版本 | **AI 团队** | 年价 | 月付参考 | 定位 |
|------|:----------:|:----:|:------:|------|
| **Community** | 1 支 | **免费** | ¥0 | 个人/试用 |
| **Starter** | 3 支 | **¥5,990/年** | ≈¥499/月 | 小微企业 |
| **Pro** ⭐ | 10 支 | **¥23,990/年** | ≈¥1,999/月 | 中小企业 |
| **Enterprise** | 不限 | **¥59,990/年** | ≈¥4,999/月 | 中大型企业 |
| **White-Label** | 不限 | **面议** | ≈¥9,999+/月 | 渠道/白牌 |

### 4.3 功能矩阵（升级版）

| 功能 | Community | Starter | Pro | Enterprise | White-Label |
|------|:---------:|:-------:|:---:|:----------:|:-----------:|
| **AI 团队数** | **1** | **3** | **10** | **不限** | **不限** |
| 官方团队模板 | DevClaw | 全部 | 全部 | 全部 | 全部+定制 |
| 自定义团队模板 | ❌ | ❌ | ✅ | ✅ | ✅ |
| 并发任务数 | 1 | 3 | 10 | 不限 | 不限 |
| Sprint 迭代 | 2 轮 | 4 轮 | 不限 | 不限 | 不限 |
| 团队知识库 | 100MB | 1GB | 10GB | 100GB | 不限 |
| ─ 已有管控能力 ─ | | | | | |
| 多模型接入 | ✅ | ✅ | ✅ | ✅ | ✅ |
| RBAC 团队隔离 | 1 部门 | 3 部门 | **不限** | **不限** | **不限** |
| SSO | ❌ | ❌ | ✅ | ✅ | ✅ |
| 审计日志 | ❌ | ❌ | ✅ | ✅ | ✅ |
| 预算告警 | ❌ | ✅ | ✅ | ✅ | ✅ |
| 合规面板 | ❌ | ❌ | ❌ | ✅ | ✅ |
| 品牌定制 | ❌ | ❌ | ❌ | ❌ | ✅ |
| 技术支持 | 社区 | 工单 | 微信 | 专属经理 | 专属团队 |

### 4.4 价格锚定（新叙事）

```
给 CEO 算账:

"你的研发部门有 5 个工程师，月人力成本 ¥60,000"
"DevClaw AI 团队 ¥2,000/月 + 星能 ≈ ¥1,000/月"
"DevClaw 可以接手 30-50% 的重复性开发工作"
"省出 1.5 个人力 = ¥18,000/月"
"ROI: 投入 ¥3,000，产出 ¥18,000，回报率 500%"

"你的市场部 3 个人，月人力成本 ¥24,000"
"MarketClaw AI 团队可以接手文案 + 素材制作"
"省出 1 个文案岗 = ¥8,000/月"
"ROI: 投入 ¥3,000，产出 ¥8,000，回报率 167%"

总计:
  Overlord Pro ¥2,000/月 + 星能 ¥2,000/月 = ¥4,000/月
  3 支 AI 团队 (Dev + Market + Support)
  节省人力: ≈ ¥30,000/月
  净 ROI: 650%
```

---

## 五、管理控制台升级

### 5.1 首页重设计

```
当前首页: DashboardPage
  → 6 个统计卡片（节点数/CPU/内存/任务/Token/团队）
  → IT 视角，无业务价值感

新首页: TeamAgentDashboard
  → 以 AI 团队为中心的仪表盘

┌─────────────────────────────────────────────────────────┐
│  Overlord 控制台                                         │
│                                                         │
│  ┌─────────────────────────────────────────────────┐    │
│  │  🎯 AI 团队概览                                  │    │
│  │                                                  │    │
│  │  DevClaw          MarketClaw       SupportClaw   │    │
│  │  ██████████ 3/5   ██████░░░░ 1/2   ████████░░ 仪 │    │
│  │  运行中·Sprint 2   运行中·文案      待命·3工单/天  │    │
│  │                                                  │    │
│  │  DataClaw         OpsClaw                        │    │
│  │  ░░░░░░░░░░ 待命   ████████░░ 监控中             │    │
│  │  上次: 3天前       健康: 全部正常                  │    │
│  └─────────────────────────────────────────────────┘    │
│                                                         │
│  ┌──────────────┐  ┌──────────────┐  ┌─────────────┐   │
│  │ 本月节省       │  │ 本月消耗       │  │ 任务完成     │   │
│  │ ¥18,500      │  │ 3,200⚡       │  │ 47/52       │   │
│  │ ≈ 2.3 人力    │  │ ≈ ¥320       │  │ 成功率 90%  │   │
│  └──────────────┘  └──────────────┘  └─────────────┘   │
│                                                         │
│  ┌─────────────────────────────────────────────────┐    │
│  │  📊 实时动态                                      │    │
│  │                                                  │    │
│  │  10:32  DevClaw · Drone-A 完成后端 API ✅         │    │
│  │  10:28  DevClaw · Reviewer 审查通过 (8.5/10) ✅   │    │
│  │  10:15  MarketClaw · 公众号文案已生成              │    │
│  │  09:45  SupportClaw · 处理 12 个工单 (满意率 92%) │    │
│  │  09:00  OpsClaw · 日报: 全部服务正常 ✅            │    │
│  └─────────────────────────────────────────────────┘    │
│                                                         │
└─────────────────────────────────────────────────────────┘
```

### 5.2 导航重构

```
当前导航:
  总览 | 节点 | 团队 | 隧道 | 更新 | Webhook | 计费 | 分析 | 审计 | 解析

新导航:
  ┌─────────────────┐
  │ AI 团队          │  ← 新增，放在最前面
  │  ├── 总览仪表盘   │
  │  ├── 我的团队     │  ← 团队列表 + 创建
  │  ├── 任务中心     │  ← 所有 Mission 看板
  │  ├── 模板市场     │  ← 官方 + 自定义模板
  │  └── 知识库      │  ← 团队共享知识
  │                  │
  │ 管控             │  ← 已有功能（次级入口）
  │  ├── 部门管理     │  (原 团队管理)
  │  ├── 节点管理     │
  │  ├── 用量分析     │
  │  ├── 预算告警     │
  │  └── 审计日志     │
  │                  │
  │ 基础设施          │  ← 运维向（收起）
  │  ├── Nydus 隧道   │
  │  ├── 版本更新     │
  │  ├── Webhook     │
  │  └── SSO 配置    │
  │                  │
  │ 设置             │
  │  ├── 套餐订阅     │  (原 计费)
  │  ├── 合规面板     │
  │  └── 品牌定制     │
  └─────────────────┘
```

### 5.3 新增页面

| 页面 | 路由 | 功能 |
|------|------|------|
| **AI 团队总览** | `/` | 团队状态卡片 + 成本/ROI + 实时动态 |
| **我的团队** | `/teams` | 团队列表 + 创建向导 + 角色配置 |
| **团队详情** | `/teams/:id` | 角色状态 + 任务看板 + 时间线 + 交付物 |
| **任务中心** | `/missions` | 全局 Mission 看板（按团队/状态/时间筛选） |
| **任务详情** | `/missions/:id` | Sprint 进度 + Step 状态 + 代码 diff + 预览 |
| **模板市场** | `/templates` | 官方模板 + 自定义模板 + 安装/配置 |
| **知识库** | `/knowledge` | 团队知识库管理（上传/索引/搜索） |

### 5.4 员工工作台升级

```
当前员工工作台:
  AI 对话 | Agent 市场 | 工具集 | 个人中心

新员工工作台:
  ┌─────────────────────────────────────────────────────────┐
  │  StarClaw 工作台                                         │
  │                                                         │
  │  ┌─────────────────────────────────────────────────┐    │
  │  │  "有什么需要 AI 团队帮你做的？"                    │    │
  │  │  ┌─────────────────────────────────────────┐    │    │
  │  │  │ 帮我做一个宠物用品电商小程序...            │    │    │
  │  │  └─────────────────────────────────────────┘    │    │
  │  │                                                 │    │
  │  │  快捷入口:                                       │    │
  │  │  🔨 DevClaw: 提开发需求                          │    │
  │  │  📝 MarketClaw: 要文案素材                       │    │
  │  │  🎧 SupportClaw: 查客服数据                      │    │
  │  └─────────────────────────────────────────────────┘    │
  │                                                         │
  │  ┌─────────────────────────────────────────────────┐    │
  │  │  我的任务                                        │    │
  │  │                                                  │    │
  │  │  📦 宠物电商小程序    DevClaw   Sprint 2 · 60%   │    │
  │  │  📝 3月营销方案       MarketClaw 已完成 · 查看    │    │
  │  │  📊 周报-客服数据     DataClaw   已完成 · 下载    │    │
  │  └─────────────────────────────────────────────────┘    │
  │                                                         │
  │  ┌─────────────────────────────────────────────────┐    │
  │  │  💬 AI 对话 (保留)                               │    │
  │  │  直接与单个 Agent 对话（简单问答/翻译/摘要）      │    │
  │  └─────────────────────────────────────────────────┘    │
  └─────────────────────────────────────────────────────────┘

员工体验:
  1. 员工 SSO 登录 → 看到"提需求"入口
  2. 输入需求 → 系统自动匹配最佳 AI 团队
  3. AI 团队自动执行 → 员工在工作台看进度
  4. 完成后通知 → 员工验收 → 反馈 → 迭代
  5. 简单问题 → 直接用 AI 对话（不走团队流程）
```

---

## 六、企业级增强

### 6.1 多部门 AI 团队分配

```
Overlord 管理员为各部门分配 AI 团队:

research_dept (研发部):
  → DevClaw × 2 (前端团队 + 后端团队)
  → 预算: 2000⚡/月

marketing_dept (市场部):
  → MarketClaw × 1
  → 预算: 1000⚡/月

support_dept (客服部):
  → SupportClaw × 1 (7×24 运行)
  → 预算: 800⚡/月

finance_dept (财务部):
  → FinanceClaw × 1 (月末激活)
  → 预算: 500⚡/月

每个部门只能看到和使用自己的 AI 团队
RBAC:
  dept_manager → 可以向 AI 团队提任务、查看进度、验收
  dept_member  → 可以向 AI 团队提任务、查看自己的任务
  admin        → 可以分配团队、调整预算、查看所有
```

### 6.2 审批流程

```
企业级任务审批:

低风险任务（预估 < 100⚡）:
  → 自动执行，无需审批
  → 例: "帮我写个单元测试"

中风险任务（预估 100-1000⚡）:
  → 部门经理审批
  → 例: "开发一个新功能模块"

高风险任务（预估 > 1000⚡ 或涉及生产环境）:
  → IT 管理员审批
  → 例: "重构整个前端" / "修改数据库"

审批流:
  员工提需求 → Architect 出方案 + 预估
  → 系统判断风险等级
  → 对应级别审批人收到通知
  → 审批通过 → 执行
  → 审批拒绝 → 通知员工原因
```

### 6.3 知识隔离

```
团队知识库与部门绑定:

全局知识库（所有团队共享）:
  → 公司规章制度
  → 品牌指南
  → 公开产品文档

部门知识库（部门内团队共享）:
  → research_dept: 技术文档、架构设计、代码规范
  → marketing_dept: 品牌素材、历史文案、竞品分析
  → support_dept: FAQ、工单模板、客户画像

团队知识库（单个团队独有）:
  → DevClaw-前端: 组件库文档、设计系统
  → DevClaw-后端: API 规范、数据库设计

知识权限:
  团队 Agent 只能访问: 全局 + 所属部门 + 自身团队的知识
  不能访问其他部门的知识 → 数据隔离
```

### 6.4 SLA 与质量承诺

```
Enterprise 版 SLA:

可用性:
  → AI 团队 99.9% 可用（年停机 < 8.76 小时）
  → 单 Agent 故障自动切换备用节点

响应时间:
  → 任务提交 → 开始规划: < 30 秒
  → Sprint 0 完成: < 4 小时（中等复杂度）
  → 紧急任务优先调度: < 10 秒

质量:
  → 代码审查评分 ≥ 7/10
  → 测试覆盖率 ≥ 60%
  → 文案原创率 ≥ 90%
  → 客服满意率 ≥ 85%

赔付:
  → 未达 SLA → 当月订阅费按比例退还
  → 未达质量标准 → 免费重做 + 人工介入
```

---

## 七、销售叙事

### 7.1 30 秒电梯演讲

```
"StarClaw 是一个 AI 团队平台。

 您花 ¥2,000 一个月，就能雇到一支 7×24 在线的 AI 团队。
 
 这支团队有架构师、有码农、有测试、有文案、有客服。
 您说需求，它们自动分工、协作、审查、交付。
 
 所有数据在您自己的服务器上，所有操作可审计。
 比招人便宜 90%，比外包快 10 倍。"
```

### 7.2 行业场景话术

```
IT/互联网公司:
  "DevClaw 相当于给你的研发部加了 3 个永远在线的初级工程师。
   重复性开发、Bug 修复、写测试、写文档这些事情，
   全部丢给 AI 团队，你的资深工程师专注高价值工作。"

电商/消费品:
  "MarketClaw 每周能产出 50 篇高质量营销文案，
   配合 Designer 自动生成海报和社交媒体素材。
   大促期间文案产出效率翻 10 倍，且 SupportClaw 接管 80% 客服。"

金融/法律:
  "LegalClaw 每天能审查 20 份合同，标注风险条款，
   所有数据在你的内网，满足等保三级合规要求。
   FinanceClaw 月末自动核对报表，准确率 > 99%。"

教育机构:
  "SupportClaw 自动回答家长 80% 的常见问题，
   DataClaw 每周生成学生学情分析报告。
   一套系统替代 3 个行政岗位。"
```

### 7.3 对标竞品话术

```
vs 飞书 AI / 钉钉 AI:
  "他们是给对话框加了个 AI 按钮。
   我们是给你公司加了一支 AI 团队。
   他们: 你问一句，AI 答一句。
   我们: 你说一个需求，AI 团队帮你做完。"

vs Dify / FastGPT:
  "他们是开发工具，你需要自己搭 AI 应用。
   我们是成品团队，开箱即用，说需求就干活。
   他们适合有 AI 工程师的公司。
   我们适合所有公司。"

vs Devin / Cursor:
  "他们是单兵 AI，一个 Agent 单独干活。
   我们是团队 AI，多个 Agent 分工协作。
   你试过让一个人同时写后端、前端、测试、文档吗？
   我们有专人（专虫）负责每个环节，还有审查虫把关质量。"
```

---

## 八、实现路线图

### Phase 0 — MVP（4 周）

```
目标: 一个可 Demo 的企业 Team Agent 体验

Week 1-2: 后端
  1. TeamAgentTemplate + TeamInstance 模型 (overlord/api/internal/model/)
  2. Team Agent CRUD API (overlord/api/internal/handler/team_agent.go)
  3. DevClaw + MarketClaw 官方模板 (内置 JSON)
  4. 与 Claw Squad Engine 集成 (通过 HTTP 调用 Claw API)

Week 3: 控制台
  5. AI 团队总览首页 (overlord/console/src/pages/TeamDashboardPage.tsx)
  6. 我的团队页 (TeamListPage.tsx)
  7. 团队详情页 (TeamDetailPage.tsx)

Week 4: 员工工作台
  8. 需求提交入口 (overlord/web/src/pages/RequestPage.tsx)
  9. 我的任务列表 (TaskListPage.tsx)
  10. 任务详情+验收 (TaskDetailPage.tsx)
```

### Phase 1 — 企业增强（4 周）

```
  11. 多部门 AI 团队分配 + RBAC
  12. 审批流程（低/中/高风险）
  13. 知识库隔离（全局/部门/团队）
  14. 预算按团队分拆 + 月报
  15. SupportClaw + DataClaw 模板
  16. WebSocket 实时仪表盘
```

### Phase 2 — 产品化（4 周）

```
  17. 企业产品页重写 (queen/site + enterprise-product.md)
  18. 定价页更新 (新套餐 + 按团队)
  19. ROI 计算器 (在线工具: 输入员工数→输出节省金额)
  20. 白皮书: "企业 AI 团队 — 从管控到生产力"
  21. Demo 视频 (3 分钟: 从提需求到交付)
  22. 自定义团队模板编辑器
  23. 团队模板市场
```

### Phase 3 — 行业深耕（持续）

```
  24. 行业专属模板 (电商/金融/教育/制造)
  25. 行业知识库预置
  26. 行业案例库
  27. 与飞书/钉钉/企微工单系统集成
  28. MCP 集成企业内部系统
```

---

## 九、技术架构

### 9.1 Overlord ↔ Claw 交互

```
Team Agent 的执行引擎在 Claw 上，Overlord 是管控层:

Overlord (控制面):                Claw (数据面):
  TeamAgentTemplate              → 调用 Claw Squad API 创建 Squad
  TeamInstance                   → 调用 Claw API 创建 Agent
  Mission 审批/监控              → 调用 Claw Squad API 创建 Mission
  Dashboard 聚合                 ← 轮询/WebSocket Claw HiveBroadcaster

┌──────────────────┐            ┌──────────────────┐
│  Overlord API    │  ──HTTP──→ │  Claw API        │
│  (管控 + 编排)    │            │  (执行 + 协作)    │
│                  │            │                  │
│  handler/        │            │  squad/engine.go │
│   team_agent.go  │──创建──→  │   → planAndDispatch │
│                  │            │   → advanceMission │
│  model/          │            │   → triggerAutoReview │
│   team_agent.go  │  ←状态── │   → runCIGate     │
│                  │            │   → completeMission │
└──────────────────┘            └──────────────────┘

通信方式:
  Overlord → Claw: 通过 Claw 注册时上报的地址，HTTP 调用 Claw API
  Claw → Overlord: Webhook 回调 (已有 webhook 机制)
                    + Overlord 主动轮询 Claw Squad API
```

### 9.2 数据模型（Overlord 侧新增）

```go
// ── 团队智能体模板（Overlord 管理）──

type TeamAgentTemplate struct {
    ID          string    `json:"id" gorm:"type:varchar(36);primaryKey"`
    Name        string    `json:"name" gorm:"type:varchar(200);not null"`
    Category    string    `json:"category" gorm:"type:varchar(50);index"`
    Description string    `json:"description" gorm:"type:text"`
    Roles       string    `json:"roles" gorm:"type:json"`          // []TeamRole
    Topology    string    `json:"topology" gorm:"type:json"`       // TopologyConfig
    QualityGate string    `json:"quality_gate" gorm:"type:json"`   // QualityGateConfig
    IsOfficial  bool      `json:"is_official" gorm:"default:false"`
    CreatedAt   time.Time `json:"created_at"`
}

// ── 运行中的团队实例 ──

type TeamInstance struct {
    ID           string     `json:"id" gorm:"type:varchar(36);primaryKey"`
    TemplateID   string     `json:"template_id" gorm:"type:varchar(36);index"`
    TeamID       string     `json:"team_id" gorm:"type:varchar(36);index"` // → Team (部门)
    ClawNodeID   string     `json:"claw_node_id" gorm:"type:varchar(50);index"` // 运行在哪个 Claw
    ClawSquadID  string     `json:"claw_squad_id" gorm:"type:varchar(36)"` // Claw 侧 Squad ID
    Name         string     `json:"name" gorm:"type:varchar(200)"`
    Status       string     `json:"status" gorm:"type:varchar(20);default:forming;index"`
    EnergyBudget int        `json:"energy_budget" gorm:"default:0"`
    EnergyUsed   int        `json:"energy_used" gorm:"default:0"`
    Config       string     `json:"config" gorm:"type:json"`
    CreatedAt    time.Time  `json:"created_at"`
    UpdatedAt    time.Time  `json:"updated_at"`
    DisbandedAt  *time.Time `json:"disbanded_at"`
}

// ── 企业任务（Overlord 侧跟踪）──

type TeamMission struct {
    ID              string     `json:"id" gorm:"type:varchar(36);primaryKey"`
    TeamInstanceID  string     `json:"team_instance_id" gorm:"type:varchar(36);index"`
    ClawMissionID   string     `json:"claw_mission_id" gorm:"type:varchar(36)"` // Claw 侧 Mission ID
    RequestedBy     string     `json:"requested_by" gorm:"type:varchar(36);index"` // AdminUser ID
    TeamID          string     `json:"team_id" gorm:"type:varchar(36);index"`      // 部门 ID
    Title           string     `json:"title" gorm:"type:varchar(500)"`
    Goal            string     `json:"goal" gorm:"type:text"`
    Status          string     `json:"status" gorm:"type:varchar(20);default:pending_approval;index"`
    ApprovalLevel   string     `json:"approval_level" gorm:"type:varchar(20)"` // auto / manager / admin
    ApprovedBy      string     `json:"approved_by" gorm:"type:varchar(36)"`
    EstimatedEnergy int        `json:"estimated_energy" gorm:"default:0"`
    ActualEnergy    int        `json:"actual_energy" gorm:"default:0"`
    CreatedAt       time.Time  `json:"created_at"`
    CompletedAt     *time.Time `json:"completed_at"`
}
```

### 9.3 API 端点（新增）

```
Overlord 管理端:
  POST   /brood/team-agents                    — 创建 AI 团队
  GET    /brood/team-agents                    — 团队列表
  GET    /brood/team-agents/:id                — 团队详情 + 实时状态
  DELETE /brood/team-agents/:id                — 解散团队
  POST   /brood/team-agents/:id/assign         — 分配到部门
  GET    /brood/team-agent-templates           — 模板列表
  POST   /brood/team-agent-templates           — 创建自定义模板 (Pro+)
  GET    /brood/team-missions                  — 全局任务列表
  GET    /brood/team-missions/:id              — 任务详情（聚合 Claw 数据）
  POST   /brood/team-missions/:id/approve      — 审批任务
  GET    /brood/team-dashboard                 — AI 团队总览仪表盘
  GET    /brood/team-reports/monthly           — 月报（按部门/团队分拆）

员工端:
  POST   /api/missions                         — 提交需求（自动匹配团队）
  GET    /api/missions                         — 我的任务列表
  GET    /api/missions/:id                     — 任务详情 + 进度
  POST   /api/missions/:id/feedback            — 提交反馈
  POST   /api/missions/:id/accept              — 验收
  GET    /api/team-agents                      — 我能用的 AI 团队
```

---

## 十、关键设计决策

### Q1: 为什么是 Overlord 的功能，不是独立产品？

```
Team Agent 需要企业管控能力（RBAC/SSO/审计/预算/合规），
这些 Overlord 已经全部具备。

独立产品 = 重新实现所有管控能力 = 浪费
放入 Overlord = 复用所有管控能力 + 提升产品价值 = 双赢

类比: Slack 加了 AI 功能，不是另出一个 "Slack AI 产品"
```

### Q2: Team Agent 运行在 Claw 还是 Overlord？

```
执行在 Claw（Squad Engine），管控在 Overlord。

Overlord 不跑 LLM，不跑 Agent，不跑代码。
Overlord 只做: 模板管理 + 团队分配 + 任务审批 + 状态聚合 + 月报。

Claw 跑: Squad Engine + Agent + Git + Tool + LLM。

好处:
  1. Overlord 保持轻量（只是管控层）
  2. Claw 已经有完整的执行能力
  3. 私有部署时，Claw 在客户内网执行，数据不出内网
  4. Overlord 可以 SaaS 部署，Claw 私有部署 → 混合架构
```

### Q3: 和已有的 "Squad" 什么关系？

```
Squad 是面向 Claw 用户的（个人/开发者）
Team Agent 是面向企业的（通过 Overlord）

底层共用 Squad Engine
上层:
  个人用户 → Claw Web → Squad Page → 自由组队
  企业用户 → Overlord Console → Team Agent → 模板化团队 + 管控

Team Agent = Squad + 模板 + RBAC + 审批 + 预算 + 月报
```

### Q4: 如何让企业客户觉得 ¥2,000/月很值？

```
不要卖"AI 管控平台 ¥2,000/月"
要卖"3 支 AI 团队 ¥2,000/月"

¥2,000 / 3 支团队 = ¥667/支/月
一支 AI 团队 = 3-5 个 AI 角色 7×24 在线

对比:
  一个实习生 = ¥3,000/月（只能做简单任务，只干 8 小时）
  一支 AI 团队 = ¥667/月（能做复杂协作任务，7×24 在线）

ROI 可视化:
  控制台首页显示"本月 AI 团队帮你节省了约 ¥XX"
  月报显示"AI 团队完成 XX 个任务，等效 XX 人天"
  让客户每天都能感受到价值
```
