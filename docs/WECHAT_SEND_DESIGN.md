# WeChat Desktop Send — 可靠发送方案

## 背景

`wechat_send` 是 `wechat_cs(action="send_reply")` 的底层实现，负责通过桌面自动化向微信群/联系人发送消息。

新版微信（Weixin）使用 Qt/Chromium 渲染，UIA 不透明，只能通过坐标点击 + 键盘模拟完成操作。

## 核心原则

> **每一步操作前都校验焦点，焦点丢失立即中止，绝不向错误窗口发送按键。**

## 架构

```
┌─────────────────────────────────────────┐
│ Layer 0: 窗口发现 (EnumWindows)          │
│  - 遍历所有顶层窗口                      │
│  - 按进程名 Weixin/WeChat 筛选           │
│  - 按标题含"微信" + 尺寸>=500x400 筛选    │
│  → 输出: 精确 HWND                       │
├─────────────────────────────────────────┤
│ Layer 1: 可靠激活 (AttachThreadInput)     │
│  - 获取当前线程 ID 和目标线程 ID           │
│  - AttachThreadInput 绑定                │
│  - SetForegroundWindow + ShowWindow       │
│  - AttachThreadInput 解绑                │
│  - 校验: GetForegroundWindow() == HWND   │
│  - 最多重试 3 次，间隔递增                 │
│  → 输出: 微信已在前台，且已校验            │
├─────────────────────────────────────────┤
│ Layer 2: 焦点守卫 (FocusGuard)            │
│  - 封装函数: SafeClick / SafeKey          │
│  - 每次操作前: assert 前台 == HWND        │
│  - 一旦断言失败 → 立即 abort，不发任何键   │
│  → 保证: 任何按键绝不打到错误窗口          │
├─────────────────────────────────────────┤
│ Layer 3: 业务流程                         │
│  Step A: 搜索切群 (Ctrl+F → 粘贴 → Enter)│
│  Step B: 定位输入框 (点击底部)             │
│  Step C: 粘贴消息 (Ctrl+A → Ctrl+V)      │
│  Step D: 发送 (Ctrl+Enter)               │
│  Step E: 校验 (输入框已清空)               │
│  每步之间都经过 FocusGuard                 │
└─────────────────────────────────────────┘
```

## Layer 0: 窗口发现

不再使用 `FindWindow`（类名在新版微信上不稳定）。

使用 `EnumWindows` + `GetWindowThreadProcessId` 遍历所有顶层窗口：
1. 获取窗口所属进程 ID
2. 根据进程名筛选（`Weixin` 或 `WeChat`）
3. 根据窗口标题筛选（包含 `微信`）
4. 根据窗口尺寸筛选（宽 >= 500 且 高 >= 400）
5. 选取第一个满足条件的窗口句柄

## Layer 1: 可靠激活

使用 `AttachThreadInput` 技巧（Windows 标准抢焦点方式）：

```
1. currentThreadId = GetCurrentThreadId()
2. targetThreadId  = GetWindowThreadProcessId(hwnd)
3. AttachThreadInput(currentThreadId, targetThreadId, true)
4. ShowWindow(hwnd, SW_RESTORE)  // 如果最小化
5. SetForegroundWindow(hwnd)
6. AttachThreadInput(currentThreadId, targetThreadId, false)
7. 校验: GetForegroundWindow() == hwnd
8. 失败则重试，最多 3 次，间隔 300/600/1000ms
```

## Layer 2: 焦点守卫

所有 UI 交互都封装为安全函数：

```powershell
function SafeClick($x, $y) {
    if (GetForegroundWindow() -ne $targetHwnd) {
        throw "FOCUS_LOST before click at ($x,$y)"
    }
    # 执行点击
}

function SafeKey($keys) {
    if (GetForegroundWindow() -ne $targetHwnd) {
        throw "FOCUS_LOST before key: $keys"
    }
    # 执行按键
}
```

## Layer 3: 业务流程

### Step A: 搜索切群
- `SafeKey('^f')`         — 打开微信搜索（Ctrl+F 快捷键，比盲点坐标更稳定）
- `Clipboard + SafeKey('^v')` — 粘贴群名
- `Sleep 1500ms`          — 等待搜索结果
- `SafeKey('{ENTER}')`    — 选中第一个搜索结果
- `Sleep 1200ms`          — 等待聊天加载
- `SafeKey('{ESC}')`      — 关闭搜索面板

### Step B: 定位输入框
- 计算输入框坐标：窗口底部中央区域
- `SafeClick($inputX, $inputY)`

### Step C: 粘贴消息
- `Clipboard.SetText($message)`
- `SafeKey('^a')` + `SafeKey('^v')` — 全选并粘贴

### Step D: 发送
- `SafeKey('^{ENTER}')` — 使用 Ctrl+Enter 发送（已验证可靠）

### Step E: 校验
- `SafeClick($inputX, $inputY)`
- `SafeKey('^a')` + `SafeKey('^c')`
- 读取剪贴板，如果仍然等于原消息 → 发送失败

## 失败处理

| 场景 | 行为 |
|------|------|
| 找不到微信窗口 | `ERROR\|WeChat window not found` |
| 激活失败（3次） | `ERROR\|failed to bring WeChat to foreground after 3 retries` |
| 任何步骤焦点丢失 | `ERROR\|FOCUS_LOST at step X` — 立即中止 |
| 发送后消息仍在输入框 | `ERROR\|message not sent` |

## 与旧方案的对比

| 维度 | 旧方案 | 新方案 |
|------|--------|--------|
| 找窗 | `FindWindow` 按类名 | `EnumWindows` 按进程名+标题+尺寸 |
| 激活 | `AppActivate` | `AttachThreadInput` + `SetForegroundWindow` |
| 搜索 | 盲点坐标 | `Ctrl+F`（微信搜索快捷键） |
| 发送 | `Enter` | `Ctrl+Enter`（已验证可用） |
| 安全 | 无焦点校验 | 每步操作前校验，焦点丢失立即中止 |
| 误操作 | 按键可能打到错误窗口 | 绝不向非目标窗口发送按键 |

## 相关文件

- `claw/api/internal/tool/desktop_uia.go` — `wechat_send` 实现
- `claw/api/internal/tool/wechat_cs_tool.go` — `send_reply` action
- `claw/api/internal/worker/task_worker.go` — 后台任务观测链路
