# Server D 部署文档

## 目标

在 `139.224.10.5` 上以 Windows 原生方式运行 Extractor：

- Python Bridge: `:8098`
- Go API: `:8097`
- miniQMT: 已登录 `test1006`
- 数据库: 默认 SQLite `C:\extractor\data\extractor.db`

## 当前推荐架构

```text
miniQMT / xtquant
        ↓
Python Bridge (:8098)
        ↓
Go API (:8097)
        ↓
Claw AI (可选)
```

## 前置条件

- `C:\Python311\python.exe` 可用
- miniQMT 已安装并登录
- `xtquant` 已从 QMT 安装目录复制到 Python site-packages
- 代码已同步到 `C:\extractor`
- 已构建 `C:\extractor\extractor-api.exe`

## 一次性部署

### 1. 同步代码

```powershell
robocopy E:\starclaw\extractor C:\extractor /MIR /XD __pycache__ .git node_modules
```

### 2. 构建 Go API

```powershell
cd E:\starclaw\extractor\api
$env:CGO_ENABLED = "0"
go build -o C:\extractor\extractor-api.exe ./cmd/server
```

### 3. 准备环境变量文件

在 `C:\extractor\env.ps1` 中至少保留以下内容：

```powershell
$env:EXTRACTOR_DATABASE_DSN = "sqlite:C:\extractor\data\extractor.db"
$env:EXTRACTOR_BRIDGE_URL = "http://localhost:8098"
$env:EXTRACTOR_PORT = "8097"
$env:BRIDGE_PORT = "8098"
$env:QMT_PATH = "C:\中金财富QMT个人版模拟交易端\userdata_mini"
$env:QMT_SESSION_ID = "123456"
$env:USE_CLAW_CONFIRM = "false"
$env:EXTRACTOR_CLAW_URL = ""
$env:EXTRACTOR_CLAW_API_KEY = ""
$env:EXTRACTOR_CLAW_MODEL = "qwen-max"
```

## 手动启动

### 启动 Bridge

```powershell
cd C:\extractor\bridge
C:\Python311\python.exe main.py
```

### 启动 Go API

```powershell
cd C:\extractor
.\extractor-api.exe
```

### 健康检查

```powershell
Invoke-RestMethod http://localhost:8098/health
Invoke-RestMethod http://localhost:8097/health
Invoke-RestMethod http://localhost:8097/v1/claw/status
```

## 持久化启动

仓库脚本：

- `scripts/start-bridge-daemon.ps1`
- `scripts/start-api-daemon.ps1`
- `scripts/persist-extractor.ps1`

在 Server D 上执行：

```powershell
PowerShell -ExecutionPolicy Bypass -File C:\extractor\scripts\persist-extractor.ps1
```

脚本会注册两个计划任务：

- `ExtractorBridge`
- `ExtractorAPI`

日志位置：

- `C:\extractor\logs\bridge.log`
- `C:\extractor\logs\api.log`

## Claw AI 配置接入

如果要启用 Claw 二次确认，在 `C:\extractor\env.ps1` 中填写：

```powershell
$env:EXTRACTOR_CLAW_URL = "https://your-claw-host"
$env:EXTRACTOR_CLAW_API_KEY = "your-api-key"
$env:EXTRACTOR_CLAW_MODEL = "qwen-max"
$env:USE_CLAW_CONFIRM = "true"
```

然后重启 Bridge 和 Go API。

验证方式：

```powershell
Invoke-RestMethod http://localhost:8097/v1/claw/status
```

期望返回：

```json
{
  "connected": true,
  "url": "https://your-claw-host"
}
```

## 触发扫描

```powershell
Invoke-RestMethod -Method POST "http://localhost:8097/v1/scan" -ContentType "application/json" -Body "{}" -TimeoutSec 300
```

当前已验证结果：

- `scanned: 3155`
- `candidates: 10`
- `confirmed: 10`
- `orders: 10`
- `elapsed: ~149s`
- 下单账号: `test1006`

## 周一开盘验证

周一 `09:30` 后再次执行扫描，并同时观察：

- Bridge 日志
- miniQMT 委托面板
- miniQMT 成交面板

周末出现 `order_id = -1` 属于预期行为，不代表链路失败。
