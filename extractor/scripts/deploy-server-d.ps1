# Extractor 萃取器 — Server D (139.224.10.5) 一键部署脚本
# 在 Windows PowerShell 中以管理员身份运行
# 前提: QMT 已安装, Python 3.11+ 已安装, Go 1.24+ 已安装

$ErrorActionPreference = "Continue"
$EXTRACTOR_HOME = "C:\extractor"

Write-Host "========================================" -ForegroundColor Cyan
Write-Host " Extractor 萃取器 — Server D 部署" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan

# ===== Step 1: 检查依赖 =====
Write-Host "`n[1/7] 检查依赖..." -ForegroundColor Yellow

$pythonOk = $false
$goOk = $false
$dockerOk = $false

try { $pyVer = python --version 2>&1; Write-Host "  Python: $pyVer" -ForegroundColor Green; $pythonOk = $true } catch { Write-Host "  Python: 未找到!" -ForegroundColor Red }
try { $goVer = go version 2>&1; Write-Host "  Go:     $goVer" -ForegroundColor Green; $goOk = $true } catch { Write-Host "  Go: 未找到!" -ForegroundColor Red }
try { $dkVer = docker --version 2>&1; Write-Host "  Docker: $dkVer" -ForegroundColor Green; $dockerOk = $true } catch { Write-Host "  Docker: 未找到 (PostgreSQL将使用本地安装)" -ForegroundColor Yellow }

if (-not $pythonOk) {
    Write-Host "`n请先安装 Python 3.11+: https://www.python.org/downloads/" -ForegroundColor Red
    Write-Host "安装时勾选 'Add Python to PATH'" -ForegroundColor Red
    exit 1
}

# ===== Step 2: 创建目录结构 =====
Write-Host "`n[2/7] 创建目录结构..." -ForegroundColor Yellow

New-Item -ItemType Directory -Force -Path "$EXTRACTOR_HOME" | Out-Null
New-Item -ItemType Directory -Force -Path "$EXTRACTOR_HOME\data" | Out-Null
New-Item -ItemType Directory -Force -Path "$EXTRACTOR_HOME\logs" | Out-Null

Write-Host "  目录: $EXTRACTOR_HOME" -ForegroundColor Green

# ===== Step 3: 复制代码 =====
Write-Host "`n[3/7] 请将 extractor/ 代码复制到 $EXTRACTOR_HOME ..." -ForegroundColor Yellow
Write-Host "  方式1: 从开发机 robocopy E:\starclaw\extractor $EXTRACTOR_HOME /MIR /XD .git __pycache__ node_modules" -ForegroundColor Gray
Write-Host "  方式2: 打包 zip 传过来解压" -ForegroundColor Gray
Write-Host ""

if (-not (Test-Path "$EXTRACTOR_HOME\bridge\main.py")) {
    Write-Host "  等待代码就位... (检测到 bridge/main.py 后继续)" -ForegroundColor Yellow
    Write-Host "  你也可以先手动复制代码，然后重新运行此脚本" -ForegroundColor Yellow
    
    $confirm = Read-Host "代码已复制到 $EXTRACTOR_HOME ? (y/n)"
    if ($confirm -ne "y") {
        Write-Host "请先复制代码，然后重新运行此脚本" -ForegroundColor Yellow
        exit 0
    }
}

# ===== Step 4: 安装 Python 依赖 =====
Write-Host "`n[4/7] 安装 Python 依赖..." -ForegroundColor Yellow

Set-Location "$EXTRACTOR_HOME\bridge"
pip install -r requirements.txt 2>&1 | ForEach-Object { Write-Host "  $_" -ForegroundColor Gray }

# xtquant 需要从 QMT 安装目录复制或单独安装
Write-Host "`n  检查 xtquant..." -ForegroundColor Yellow
try {
    python -c "from xtquant import xtdata; print('xtquant OK')" 2>&1
    Write-Host "  xtquant: 已安装" -ForegroundColor Green
} catch {
    Write-Host "  xtquant: 未找到!" -ForegroundColor Red
    Write-Host "  请从 QMT 安装目录复制 xtquant 包到 Python site-packages" -ForegroundColor Yellow
    Write-Host "  通常在: C:\国金QMT交易端模拟\bin.x64\Lib\site-packages\xtquant" -ForegroundColor Yellow
    Write-Host "  复制到: $(python -c 'import site; print(site.getsitepackages()[0])')" -ForegroundColor Yellow
}

# ===== Step 5: PostgreSQL =====
Write-Host "`n[5/7] 设置 PostgreSQL..." -ForegroundColor Yellow

if ($dockerOk) {
    Write-Host "  可选: 使用 Docker 启动 PostgreSQL..." -ForegroundColor Green
    Set-Location "$EXTRACTOR_HOME"
    docker compose up -d postgres 2>&1 | ForEach-Object { Write-Host "  $_" -ForegroundColor Gray }
} else {
    Write-Host "  Docker 不可用，默认将使用 SQLite: C:\extractor\data\extractor.db" -ForegroundColor Green
    Write-Host "  如需 PostgreSQL，再手动安装并修改 env.ps1 中的 EXTRACTOR_DATABASE_DSN" -ForegroundColor Yellow
}

# ===== Step 6: 编译 Go API =====
Write-Host "`n[6/7] 编译 Go API..." -ForegroundColor Yellow

if ($goOk) {
    Set-Location "$EXTRACTOR_HOME\api"
    Write-Host "  go mod tidy..." -ForegroundColor Gray
    go mod tidy 2>&1 | ForEach-Object { Write-Host "  $_" -ForegroundColor Gray }
    Write-Host "  go build..." -ForegroundColor Gray
    go build -o "$EXTRACTOR_HOME\extractor-api.exe" ./cmd/server 2>&1
    if (Test-Path "$EXTRACTOR_HOME\extractor-api.exe") {
        Write-Host "  编译成功: $EXTRACTOR_HOME\extractor-api.exe" -ForegroundColor Green
    } else {
        Write-Host "  编译失败!" -ForegroundColor Red
    }
} else {
    Write-Host "  Go 未安装，跳过编译" -ForegroundColor Yellow
    Write-Host "  下载: https://go.dev/dl/" -ForegroundColor Yellow
}

# ===== Step 7: 创建启动脚本 =====
Write-Host "`n[7/7] 创建启动脚本..." -ForegroundColor Yellow

# 环境变量文件
$envContent = @"
# Extractor 环境变量
`$env:EXTRACTOR_DATABASE_DSN = "sqlite:C:\extractor\data\extractor.db"
`$env:EXTRACTOR_BRIDGE_URL = "http://localhost:8098"
`$env:EXTRACTOR_PORT = "8097"
`$env:EXTRACTOR_CLAW_URL = ""
`$env:EXTRACTOR_CLAW_API_KEY = ""
`$env:EXTRACTOR_CLAW_MODEL = "qwen-max"
`$env:QUEEN_URL = "https://api.starclaw.net"
`$env:QUEEN_TOKEN = "sc2026-xK9mWqL3vNpR7tYhBjF5sDcEaGiUoZ4"
`$env:BRIDGE_PORT = "8098"
`$env:QMT_PATH = "C:\国金QMT交易端模拟\userdata_mini"
`$env:QMT_SESSION_ID = "123456"
`$env:REAL_TRADE = "false"
`$env:USE_CLAW_CONFIRM = "false"
"@
$envContent | Out-File -Encoding utf8 "$EXTRACTOR_HOME\env.ps1"
Write-Host "  环境变量: $EXTRACTOR_HOME\env.ps1" -ForegroundColor Green

# 启动 Go API
$startApiContent = @"
# 启动 Extractor Go API
Write-Host "Starting Extractor API on :8097..." -ForegroundColor Cyan
Set-Location "$EXTRACTOR_HOME"
. .\env.ps1
if (Test-Path ".\extractor-api-300.exe") {
    .\extractor-api-300.exe
} else {
    .\extractor-api.exe
}
"@
$startApiContent | Out-File -Encoding utf8 "$EXTRACTOR_HOME\start-api.ps1"

# 启动 Python Bridge
$startBridgeContent = @"
# 启动 Extractor Python Bridge
Write-Host "Starting Extractor Bridge on :8098..." -ForegroundColor Cyan
Set-Location "$EXTRACTOR_HOME\bridge"
. "$EXTRACTOR_HOME\env.ps1"
python main.py
"@
$startBridgeContent | Out-File -Encoding utf8 "$EXTRACTOR_HOME\start-bridge.ps1"

# 执行一次扫描
$scanContent = @"
# 触发一次扫描
Write-Host "Triggering scan..." -ForegroundColor Cyan
`$result = Invoke-RestMethod -Uri "http://localhost:8097/v1/scan" -Method POST -ContentType "application/json" -Body '{}'
`$result | ConvertTo-Json -Depth 5
"@
$scanContent | Out-File -Encoding utf8 "$EXTRACTOR_HOME\scan-once.ps1"

Write-Host "  start-api.ps1    — 启动 Go API" -ForegroundColor Green
Write-Host "  start-bridge.ps1 — 启动 Python Bridge" -ForegroundColor Green
Write-Host "  scan-once.ps1    — 触发一次扫描" -ForegroundColor Green

# ===== 完成 =====
Write-Host "`n========================================" -ForegroundColor Cyan
Write-Host " 部署完成!" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""
Write-Host "启动顺序:" -ForegroundColor Yellow
Write-Host "  1. 确保 QMT 客户端已启动并登录" -ForegroundColor White
Write-Host "  2. 打开 PowerShell 窗口1: .\start-bridge.ps1" -ForegroundColor White
Write-Host "  3. 打开 PowerShell 窗口2: .\start-api.ps1" -ForegroundColor White
Write-Host "  4. 打开 PowerShell 窗口3: .\scan-once.ps1" -ForegroundColor White
Write-Host ""
Write-Host "首次测试建议:" -ForegroundColor Yellow
Write-Host "  - env.ps1 中 REAL_TRADE=false (不会实际下单)" -ForegroundColor White
Write-Host "  - env.ps1 中 USE_CLAW_CONFIRM=false (跳过AI确认，先验证打分)" -ForegroundColor White
Write-Host "  - 确认打分结果合理后再打开实盘和AI确认" -ForegroundColor White
Write-Host "  - 如需开机自启，执行 C:\extractor\scripts\persist-extractor.ps1" -ForegroundColor White
Write-Host ""
Write-Host "访问地址:" -ForegroundColor Yellow
Write-Host "  API:    http://localhost:8097/health" -ForegroundColor White
Write-Host "  Bridge: http://localhost:8098/health" -ForegroundColor White
Write-Host "  扫描:   POST http://localhost:8097/v1/scan" -ForegroundColor White
