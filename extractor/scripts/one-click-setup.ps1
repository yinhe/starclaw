# ============================================================
# Extractor 萃取器 — Server D 一键安装 (粘贴到 PowerShell 即可)
# ============================================================
$ErrorActionPreference = "Continue"
$H = "C:\extractor"

Write-Host "=== Step 1: 创建目录 ===" -ForegroundColor Cyan
New-Item -ItemType Directory -Force -Path $H,$H\data,$H\logs | Out-Null

Write-Host "=== Step 2: 检查环境 ===" -ForegroundColor Cyan
$pyOk = $null; try { $pyOk = python --version 2>&1; Write-Host "  Python: $pyOk" -ForegroundColor Green } catch { Write-Host "  Python: 未安装!" -ForegroundColor Red; exit 1 }

Write-Host "=== Step 3: 下载代码 (从 GitHub raw 或本地) ===" -ForegroundColor Cyan
# 这里我们用内联方式直接创建关键文件

# --- bridge/requirements.txt ---
New-Item -ItemType Directory -Force -Path $H\bridge | Out-Null
@"
fastapi>=0.111.0
uvicorn>=0.30.0
pydantic>=2.7.0
pyyaml>=6.0
httpx>=0.27.0
numpy>=1.26.0
pandas>=2.2.0
"@ | Out-File -Encoding utf8 "$H\bridge\requirements.txt"

Write-Host "=== Step 4: 安装 Python 依赖 ===" -ForegroundColor Cyan
Set-Location $H\bridge
pip install -r requirements.txt 2>&1 | Select-Object -Last 5

Write-Host "=== Step 5: 检查 xtquant ===" -ForegroundColor Cyan
$xtOk = python -c "from xtquant import xtdata; print('xtquant OK')" 2>&1
if ($xtOk -match "OK") {
    Write-Host "  xtquant: 已安装" -ForegroundColor Green
} else {
    Write-Host "  xtquant: 未找到，尝试查找 QMT 安装目录..." -ForegroundColor Yellow
    $qmtPaths = @(
        "C:\国金QMT交易端模拟\bin.x64\Lib\site-packages",
        "C:\国金QMT\bin.x64\Lib\site-packages",
        "C:\中信建投QMT\bin.x64\Lib\site-packages",
        "D:\QMT\bin.x64\Lib\site-packages"
    )
    $found = $false
    foreach ($p in $qmtPaths) {
        if (Test-Path "$p\xtquant") {
            $siteDir = python -c "import site; print(site.getsitepackages()[0])"
            Write-Host "  找到 xtquant: $p\xtquant" -ForegroundColor Green
            Write-Host "  复制到: $siteDir" -ForegroundColor Green
            Copy-Item -Recurse -Force "$p\xtquant" "$siteDir\xtquant"
            $found = $true
            break
        }
    }
    if (-not $found) {
        Write-Host "  未找到 xtquant，策略将以 MOCK 模式运行" -ForegroundColor Yellow
        Write-Host "  你可以手动将 QMT 安装目录下的 xtquant 文件夹复制到 Python site-packages" -ForegroundColor Yellow
    }
}

Write-Host "=== Step 6: 检查 Go ===" -ForegroundColor Cyan
$goOk = $null; try { $goOk = go version 2>&1; Write-Host "  Go: $goOk" -ForegroundColor Green } catch {
    Write-Host "  Go: 未安装，正在下载..." -ForegroundColor Yellow
    Write-Host "  请手动安装: https://go.dev/dl/go1.24.2.windows-amd64.msi" -ForegroundColor Yellow
}

Write-Host "=== Step 7: 检查 PostgreSQL ===" -ForegroundColor Cyan
$pgOk = $null; try { $pgOk = psql --version 2>&1; Write-Host "  PostgreSQL: $pgOk" -ForegroundColor Green } catch {
    Write-Host "  PostgreSQL: 未安装" -ForegroundColor Yellow
    Write-Host "  Go API 会在首次启动时自动用 SQLite 作为后备 (开发模式)" -ForegroundColor Yellow
    Write-Host "  生产环境请安装: https://www.postgresql.org/download/windows/" -ForegroundColor Yellow
}

Write-Host ""
Write-Host "=== 环境检查完成 ===" -ForegroundColor Cyan
Write-Host ""
Write-Host "下一步: 需要把 extractor 完整代码复制到 $H" -ForegroundColor Yellow
Write-Host "在你的开发机(E:\starclaw)上执行以下命令将代码 scp 过来:" -ForegroundColor Yellow
Write-Host ""
Write-Host "  scp E:\extractor-deploy.zip Administrator@139.224.10.5:C:\extractor\" -ForegroundColor White
Write-Host ""
Write-Host "或者通过 RDP 剪贴板直接拖拽 E:\extractor-deploy.zip 到 Server D 桌面" -ForegroundColor White
Write-Host "然后在 Server D 上执行:" -ForegroundColor White
Write-Host "  Expand-Archive -Path C:\extractor\extractor-deploy.zip -DestinationPath C:\extractor -Force" -ForegroundColor White
