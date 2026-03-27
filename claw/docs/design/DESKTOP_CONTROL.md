# Desktop Control — AI 桌面操控系统

> Claw 的 "Computer Use" 能力：让 AI Agent 直接操控用户电脑上的任何软件。

## 1. 架构概览

```
┌─────────────────────────────────────────────────────────────────┐
│  Layer 3: 应用语义层 (Application Semantics)                      │
│  Excel.ReadCell("A1") / Word.Bold() / Browser.Navigate("url")   │
│  → COM Automation / Chrome DevTools Protocol                     │
├─────────────────────────────────────────────────────────────────┤
│  Layer 2: UI 元素层 (Accessibility Tree)                          │
│  ui_click(name="保存") / ui_type(name="搜索", text="关键词")      │
│  → Windows UI Automation API (.NET)                              │
├─────────────────────────────────────────────────────────────────┤
│  Layer 1: 像素层 (Screenshot + Mouse/Keyboard)                    │
│  screenshot → vision model → mouse_click(x,y)                   │
│  → PowerShell + user32.dll                                       │
└─────────────────────────────────────────────────────────────────┘
优先使用高层 → 高层不可用时自动降级到低层
```

### 设计原则

1. **有眼睛优先于盲人** — `ui_tree` 获取结构化元素 >> 截图让视觉模型猜坐标
2. **直连 API 优于 UI 操作** — `excel_write("A1", "值")` >> 点击单元格再打字
3. **零外部依赖** — 全部基于 Windows 内置 API，无需安装额外软件
4. **三层降级** — 语义层 → 元素层 → 像素层，确保任何软件都能操控

## 2. 完整操作清单 (28 个)

### Layer 2: UI 自动化 (6 个) — `desktop_uia.go`

| 操作 | 参数 | 说明 |
|------|------|------|
| `ui_tree` | `title?` (窗口名), `seconds?` (深度1-8) | 获取前台窗口 UI 元素树，返回扁平化列表（id/name/type/坐标） |
| `ui_click` | `title` (元素名称) | 按名称精确点击。先尝试 InvokePattern（原生），后降级物理点击 |
| `ui_type` | `title` (元素名), `text` (内容) | 向输入框填入文本。先尝试 ValuePattern，后降级剪贴板粘贴 |
| `ui_select` | `title` (下拉框名), `text` (选项值) | 展开下拉框并选择指定选项 |
| `ui_scroll` | `button` (方向: up/down/left/right), `seconds` (量1-10) | 鼠标滚轮滚动 |
| `ui_wait` | `title` (元素名), `seconds` (超时) | 轮询等待指定元素出现 |

**技术实现：** Windows UI Automation API (`System.Windows.Automation`)，通过 PowerShell 调用 .NET 程序集。

**核心优势：**
- 精确定位元素坐标（不依赖视觉模型）
- 支持所有 Windows 应用（包括 WPF、WinForms、UWP、Win32）
- 50ms 响应，vs 截图+视觉模型 6000ms

### Layer 3a: 浏览器 CDP (6 个) — `desktop_browser.go`

| 操作 | 参数 | 说明 |
|------|------|------|
| `browser_navigate` | `text` (URL) | 导航到网页，自动启动 Chrome/Edge |
| `browser_click` | `text` (CSS 选择器或按钮文字) | 点击页面元素 |
| `browser_type` | `title` (CSS 选择器), `text` (内容) | 填写表单字段 |
| `browser_read` | `text` ("text"/"links"/"inputs"/CSS选择器) | 读取页面内容 |
| `browser_js` | `text` (JavaScript 代码) | 执行任意 JS |
| `browser_tabs` | `title?` (切换到的标签名) | 列出或切换标签页 |

**技术实现：** Chrome DevTools Protocol (CDP)，通过 WebSocket 连接 `--remote-debugging-port=9222`。

**自动启动：** 如果 Chrome/Edge 未以 CDP 模式运行，自动启动并等待就绪。

### Layer 3b: Office COM (6 个) — `desktop_office.go`

| 操作 | 参数 | 说明 |
|------|------|------|
| `excel_read` | `text` (范围如 "A1:D20" 或 "Sheet2!A1:C10") | 读取单元格数据 |
| `excel_write` | `title` (单元格地址), `text` (值或 JSON 二维数组) | 写入，支持批量 |
| `excel_formula` | `title` (单元格), `text` (公式如 "=SUM(A1:B1)") | 设置公式并返回计算结果 |
| `word_read` | — | 读取活动文档（名称、页数、字数、正文） |
| `word_write` | `text` (内容), `button` ("append"/"replace"/"insert") | 写入文档 |
| `word_format` | `text` (格式命令，逗号分隔) | 格式化选中内容 |

**Word 格式命令：** `bold`, `italic`, `underline`, `fontsize:16`, `fontname:微软雅黑`, `heading:1`, `align:center`, `color:255`

**技术实现：** Windows COM Automation，通过 `[Runtime.Interopservices.Marshal]::GetActiveObject()` 连接已打开的 Office 实例。

### Layer 3c: 文件系统 (3 个) — `desktop_office.go`

| 操作 | 参数 | 说明 |
|------|------|------|
| `file_list` | `text` (目录路径) | 列出文件和子目录 |
| `file_read` | `text` (文件路径) | 读取文件内容（限 5000 字符） |
| `file_write` | `title` (文件路径), `text` (内容) | 写入文件（UTF-8） |

### Layer 1: 像素级操作 (7 个) — `desktop_tool.go`

| 操作 | 参数 | 说明 |
|------|------|------|
| `screenshot` | `region?` ("full" 或 "x,y,w,h") | 截取屏幕 |
| `mouse_click` | `x`, `y`, `button?`, `click_type?` | 坐标点击 |
| `mouse_move` | `x`, `y` | 移动光标 |
| `mouse_drag` | `x`, `y`, `x2`, `y2` | 拖拽 |
| `keyboard_type` | `text` | 输入文字（支持中文，走剪贴板） |
| `keyboard_hotkey` | `text` (如 "ctrl+c") | 组合键 |
| `keyboard_key` | `text` (如 "enter") | 特殊键 |

## 3. Agent 工作流示例

### 操作 Excel 做数据分析

```
Agent: 我来帮你分析这份销售数据。

1. desktop(action="excel_read", text="A1:F100")
   → 获取100行数据

2. [LLM 分析数据，生成公式]

3. desktop(action="excel_write", title="G1", text="增长率")
   desktop(action="excel_formula", title="G2", text="=(F2-E2)/E2*100")

4. desktop(action="excel_read", text="G2")
   → 确认公式计算结果
```

### 操作剪映做视频

```
Agent: 好的，我来帮你编辑这段视频。

1. desktop(action="launch_app", text="剪映")
2. desktop(action="wait", seconds=3)
3. desktop(action="ui_tree")
   → [获取剪映界面元素：导入、时间轴、导出等]

4. desktop(action="ui_click", title="导入素材")
5. desktop(action="ui_wait", title="打开", seconds=5)
6. desktop(action="ui_type", title="文件名", text="C:\\视频\\素材.mp4")
7. desktop(action="ui_click", title="打开")
8. desktop(action="ui_wait", title="时间轴", seconds=3)

9. desktop(action="ui_click", title="导出")
10. desktop(action="ui_select", title="分辨率", text="1080P")
11. desktop(action="ui_click", title="导出")
```

### 操作浏览器搜索信息

```
Agent: 让我帮你查一下这个问题。

1. desktop(action="browser_navigate", text="https://www.google.com")
2. desktop(action="browser_type", title="textarea[name=q]", text="StarClaw AI Agent")
3. desktop(action="browser_click", text="Google 搜索")
4. desktop(action="browser_read", text="text")
   → [读取搜索结果页面文本]
```

### 操作微信发消息

```
Agent: 我来帮你发一条消息。

1. desktop(action="launch_app", text="微信")
2. desktop(action="wait", seconds=2)
3. desktop(action="ui_tree")
4. desktop(action="ui_click", title="搜索")
5. desktop(action="ui_type", title="搜索", text="张三")
6. desktop(action="ui_wait", title="张三", seconds=3)
7. desktop(action="ui_click", title="张三")
8. desktop(action="ui_type", title="输入框", text="你好，下午3点开会")
9. desktop(action="keyboard_key", text="enter")
```

## 4. 性能对比

| 指标 | 像素层 (Layer 1) | UI 元素层 (Layer 2) | 语义层 (Layer 3) |
|------|-----------------|-------------------|----------------|
| **定位速度** | 3-6s (截图+视觉模型) | 50ms (UIA 查询) | 10ms (直连 API) |
| **定位精度** | ~60% (LLM 猜坐标) | ~99% (精确元素边界) | 100% (API 直达) |
| **LLM 成本** | $0.01/步 (视觉模型) | $0 (结构化数据) | $0 (API 调用) |
| **适用范围** | 任何可见内容 | Windows 应用 | 特定应用 (浏览器/Office) |
| **可靠性** | 低 (分辨率/DPI 敏感) | 高 (与分辨率无关) | 最高 (API 级稳定) |

## 5. 限制与未来规划

### 当前限制

| 限制 | 原因 | 计划 |
|------|------|------|
| 仅 Windows | PowerShell + user32.dll + .NET | P5: macOS (AXUIElement) + Linux (AT-SPI2) |
| 仅本地运行 | 需要桌面访问权限 | Spore Desktop 模式专用 |
| Office 需已打开 | COM 连接活动实例 | 可扩展为自动打开文件 |
| 浏览器需 CDP 端口 | Chrome --remote-debugging-port | 自动启动已实现 |

### 未来规划

| 阶段 | 内容 |
|------|------|
| **P4** | 智能任务编排 — 多步骤自动重试、失败检测、替代方案切换 |
| **P5** | macOS 支持 — AppleScript + Accessibility API |
| **P6** | Linux 支持 — AT-SPI2 + xdotool |
| **P7** | 视觉理解增强 — 截图标注 UI 元素 ID（overlay），减少坐标猜测 |
| **P8** | 应用插件 — 剪映/Figma/Photoshop 专属 API 连接器 |

## 6. 文件结构

```
claw/api/internal/tool/
├── desktop_tool.go      # 入口 + 参数定义 + 像素级操作 (Layer 1)
├── desktop_uia.go       # UI Automation — 元素级操作 (Layer 2)
├── desktop_browser.go   # Chrome DevTools Protocol (Layer 3a)
└── desktop_office.go    # Office COM + 文件系统 (Layer 3b/3c)
```

## 7. 安全

- **Docker 模式禁用** — `isDockerEnv()` 检测，Hive 托管容器不可使用桌面工具
- **Hosted 模式限制** — `STARCLAW_SERVER_DEPLOY_MODE=hosted` 时 code_tool 已屏蔽 execute/run_command
- **仅 Spore 模式可用** — 用户本地安装时才启用完整桌面操控
- **文件操作限制** — `file_read` 限制 5000 字符，`file_write` 使用 base64 防注入
