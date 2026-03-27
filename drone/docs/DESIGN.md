# Drone 设计文档 — 🐝 工蜂采集 + 🧬 虫茧同化

## 1. 定位

Drone 是 StarClaw 虫族生态中的**数据采集微服务**，负责：
1. 从外部 AI Agent/Skill 平台采集原始数据（Collector 采集器）
2. 将异构格式同化为 StarClaw AgentTemplate 原生格式（Cocoon 虫茧）
3. 批量导入 Queen 市场并触发审核流程（Importer 导入器）
4. 定时调度增量/全量采集（Scheduler 调度器）

## 2. 虫族类比

```
虫族 Drone（工蜂）采集矿物 → 送回 Hatchery
StarClaw Drone 采集外部 Skill → 虫茧同化 → 送回 Queen 市场

虫族变态过程: 幼虫 → 虫茧包裹 → 基因重写 → 新单位
Skill 同化过程: 原始数据 → Cocoon 处理 → 格式/prompt/工具转换 → 原生 Agent
```

## 3. 三层同化架构

### Level 1: 自动变态 (Auto-Morph) — 95% 走此路径

处理速度: < 1秒/个，纯规则引擎

1. **格式转换**: 外部 JSON → StarClaw AgentTemplate 字段映射
2. **工具映射**: `web_browser` → `web_search`, `code_interpreter` → `code` 等
3. **Prompt 适配**: 删除平台特有指令，添加 StarClaw 标准格式
4. **翻译**: 英文 name/description → 中文（通过 StarAI API）
5. **分类**: 规则优先 + LLM 兜底，归入 7 个 category
6. **质量评分**: prompt 长度/结构/工具数 → 0-100 分
7. **去重**: source_id 精确匹配 + prompt embedding 相似度 > 0.95

### Level 2: LLM 进化 (LLM-Evolve) — 热门/高价值 Agent

处理速度: ~5秒/个，调用一次 StarAI LLM

1. **Prompt 重写**: 角色定义 + 能力范围 + 输出格式 + 删除平台残留
2. **增强生成**: 示例对话、卖点描述、模型推荐、icon 建议
3. **质量复评**: 重写后再评分，质量下降则保留原版

### Level 3: DevClaw 深度同化 — 顶级复杂 Agent（手动触发）

处理速度: ~2分钟/个，DevClaw 5角色团队协作

1. 设计虫分析意图 → 编码虫重写 → 测试虫沙盒验证 → 审查虫安全检查 → 文档虫生成文案
2. 通过 Overlord → DevClaw → Claw 内部 API 链路
3. 输出带 "✅ DevClaw Certified" 标记

## 4. 数据源适配

### ClawHub.ai (直通级)
- 协议: MIT 开源
- 格式: AgentSkills bundle → 几乎直接映射 AgentTemplate
- 同化: L1 即可，仅需字段重命名

### SkillHub.club
- 格式: Claude/Codex skill → instructions 映射 system_prompt
- 同化: L1，instructions 字段直接作为 system_prompt

### GPTs Store
- 格式: GPT 配置 → 需要深度适配
- 工具: DALL-E/Code Interpreter/Browsing → 映射
- Prompt 风格: "As a GPT..." 需要清除
- 同化: L2 必须（prompt 重写）

### Coze Bot
- 格式: Bot + Plugin + Workflow
- 工具: Coze 插件体系 → 部分可映射，部分丢弃
- 同化: L1 + L2 (热门)

## 5. 质量认证标记

| 标记 | 含义 | 市场状态 |
|------|------|---------|
| 🔄 Imported | L1 自动变态导入 | pending_review |
| ⚡ Enhanced | L2 LLM 进化重写 | pending_review |
| ✅ Certified | L3 DevClaw 团队测试通过 | published + featured |
| ⭐ Featured | 管理员手动精选 | published + featured |

## 6. 定时调度

| 来源 | 频率 | 模式 |
|------|------|------|
| ClawHub | 每天 01:00 | 增量 |
| SkillHub | 每天 01:00 | 增量 |
| GitHub awesome | 每周一 02:00 | 全量 |
| Dify | 每周三 02:00 | 增量 |
| Coze | 每周二、五 03:00 | 增量 |
| GPTs Store | 每周六 04:00 | 增量 (Scrapling) |
| FlowGPT | 每周四 03:00 | 增量 (Scrapling) |

## 7. 部署

- Server C (43.106.158.26) 与 Queen/Nydus 同服务器
- Docker Compose: drone-api (Go :8110) + drone-worker (Python)
- Pheromone ESB 连接: 发布 `drone.harvest.completed` 事件
- Nydus 自动部署: `drone/` 目录变更时触发
