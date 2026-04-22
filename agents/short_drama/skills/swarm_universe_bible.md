# 剧本库使用指南

> 本文件是短剧导演 Agent 的通用知识，教 Agent 如何加载和使用剧本项目。

---

## 一、三层数据模型

| 层 | 性质 | 加载时机 | 示例 |
|---|---|---|---|
| **Skills** | 通用生产方法论 | 常驻，所有对话自动加载 | 本文件、seedance_production.md、tts_voices |
| **剧本库** | 项目级数据 | 按需加载，用户指定项目 | 虫群宇宙、都市爱情等 |
| **单次输入** | 临时素材 | 用户粘贴/上传 | 新剧本、单集修改 |

---

## 二、剧本项目标准目录结构

每个剧本项目应包含以下文件：

```
project_root/
├── bible.md                     ← 项目圣经（角色库+世界观+[图N]标签）
├── drama/                       ← 剧本文件夹
│   ├── EP01_*.md                ← 故事剧本
│   ├── EP01_*_PROMPTS.md        ← Seedance 生成提示词
│   └── ...
├── production/                  ← 生产资产（可选）
│   ├── characters/              ← 角色三视图
│   └── ep01/                    ← 各集素材
└── README.md                    ← 项目简介
```

### bible.md 必须包含的内容

1. **系列概况** — 名称、类型、平台、时长规格
2. **角色外貌卡** — 每个角色的 [图N] 标签描述（一字不差全集复用）
3. **角色 Claw Image IDs** — 三视图的 Claw Image ID
4. **已完成集数** — 避免重复或冲突
5. **视觉风格** — 统一 style prefix
6. **禁区规则** — 该项目的创作禁忌

---

## 三、加载剧本项目的方式

### 方式 1：Gland 默认路径

如果用户在 Agent 配置中设置了 `drama_project_path`，开始对话时自动读取该路径下的 `bible.md`。

```
drama_project_path = "E:\starclaw\docs\swarm-universe"
→ 自动读取 E:\starclaw\docs\swarm-universe\bible.md
→ 自动扫描 drama/ 文件夹中的剧本列表
```

### 方式 2：用户临时指定路径

用户在对话中说：
- "加载 E:\my_projects\urban_romance"
- "切换到 D:\drama\comedy_series"

Agent 应：
1. 用 code 工具读取该路径下的 `bible.md`
2. 扫描 `drama/` 文件夹获取剧本列表
3. 汇报角色库和可用集数

### 方式 3：无项目（从零开始）

用户说"帮我从零拍一个新短剧"，无需加载任何项目。Agent 走标准创作流程。

---

## 四、加载后 Agent 应做的事

1. 读取 `bible.md` → 提取角色库、[图N] 标签、Image IDs
2. 扫描 `drama/` → 列出已有剧本（EP01, EP02...）
3. 扫描 `production/characters/` → 确认角色三视图是否存在
4. 汇报给用户：已加载的项目名、角色数、已有集数
5. 等待用户指令：拍哪一集 / 修改哪一集 / 从零写新集

---

## 五、多项目切换

- 通过 gland `drama_project_path` 设置默认项目
- 对话中可临时切换：`"切换到 D:\another_drama"`
- 切换时 Agent 清空当前角色库，重新读取新项目的 `bible.md`
- 项目之间的角色、世界观、风格完全隔离
