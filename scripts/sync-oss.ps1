# StarClaw Open-Source Sync Script (Windows)
# Syncs claw/ from private monorepo to starclaw-oss (GitHub)
#
# Usage:
#   .\scripts\sync-oss.ps1                          # sync only
#   .\scripts\sync-oss.ps1 -Push                    # sync + commit + push
#   .\scripts\sync-oss.ps1 -Push -Tag "v2026.0321.2000"  # sync + commit + push + tag
#   .\scripts\sync-oss.ps1 -Push -Message "feat: add X"  # custom commit message

param(
    [switch]$Push,
    [string]$Tag = "",
    [string]$Message = ""
)

$ErrorActionPreference = "Stop"

$MonoRepo = "E:\starclaw"
$OssRepo  = "E:\starclaw-oss"
$ClawDir  = "$MonoRepo\claw"

# ── Validate ──
if (-not (Test-Path "$ClawDir\api")) {
    Write-Error "claw/api not found at $ClawDir. Are you in the monorepo?"
    exit 1
}
if (-not (Test-Path "$OssRepo\.git")) {
    Write-Error "OSS repo not found at $OssRepo. Clone it first: git clone git@github.com:yinhe/starclaw.git $OssRepo"
    exit 1
}

Write-Host ""
Write-Host "========================================" -ForegroundColor Cyan
Write-Host "  StarClaw OSS Sync" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host "  Source: $ClawDir"
Write-Host "  Target: $OssRepo"
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""

# ── Step 1: Robocopy (mirror sync, excluding unwanted dirs/files) ──
Write-Host "[1/4] Syncing claw/ -> starclaw-oss/ ..." -ForegroundColor Yellow

$robocopyArgs = @(
    $ClawDir,
    $OssRepo,
    "/MIR",
    "/XD", "node_modules", ".git", "data", "build", "dist", "__debug_bin*",
    "/XF", "sync-oss.sh", "*.tar.gz", "server", "server.exe", "mcp-bridge.exe", "mcp-bridge-linux", "mcp-bridge-windows-amd64.exe",
    "/NFL", "/NDL", "/NJH", "/NJS"  # quiet output
)

$result = & robocopy @robocopyArgs
# Robocopy exit codes: 0=no change, 1=copied, 2=extra deleted, 3=both — all OK
# Only 8+ is an error
if ($LASTEXITCODE -ge 8) {
    Write-Error "Robocopy failed with exit code $LASTEXITCODE"
    exit 1
}
$LASTEXITCODE = 0  # Reset for git commands

Write-Host "  Sync complete." -ForegroundColor Green

# ── Step 2: Security audit — check for closed-source leaks ──
Write-Host "[2/4] Auditing for closed-source leaks ..." -ForegroundColor Yellow

$leakPatterns = @(
    "queen/api",
    "queen/docs",
    "overlord/api",
    "overlord/console",
    "synapse/api",
    "cerebrate/",
    "nydus/api",
    "spore/cmd",
    "larva/"
)

$leakFound = $false
foreach ($pattern in $leakPatterns) {
    $matches = Select-String -Path "$OssRepo\**\*.go","$OssRepo\**\*.md","$OssRepo\**\*.ts","$OssRepo\**\*.tsx" -Pattern $pattern -SimpleMatch -ErrorAction SilentlyContinue
    if ($matches) {
        Write-Host "  WARNING: Found '$pattern' in:" -ForegroundColor Red
        foreach ($m in $matches) {
            Write-Host "    $($m.Filename):$($m.LineNumber) — $($m.Line.Trim())" -ForegroundColor Red
        }
        $leakFound = $true
    }
}

if ($leakFound) {
    Write-Host "  Closed-source references detected! Review before pushing." -ForegroundColor Red
} else {
    Write-Host "  No closed-source leaks found." -ForegroundColor Green
}

# ── Step 3: Show diff summary ──
Write-Host "[3/4] Changes summary:" -ForegroundColor Yellow
Push-Location $OssRepo
git add -A
$diffStat = git diff --cached --stat
if (-not $diffStat) {
    Write-Host "  No changes to commit." -ForegroundColor Gray
    Pop-Location
    exit 0
}
Write-Host $diffStat

# ── Step 4: Commit + Push (if -Push flag) ──
if ($Push) {
    Write-Host "[4/4] Committing and pushing ..." -ForegroundColor Yellow

    if (-not $Message) {
        # Auto-generate commit message from monorepo's latest commit
        $latestMsg = git -C $MonoRepo log -1 --pretty=format:"%s"
        $Message = "sync: $latestMsg"
    }

    git commit -m $Message
    Write-Host "  Committed: $Message" -ForegroundColor Green

    if ($Tag) {
        git tag $Tag
        git push origin main --tags
        Write-Host "  Pushed with tag: $Tag" -ForegroundColor Green
    } else {
        git push origin main
        Write-Host "  Pushed to origin/main." -ForegroundColor Green
    }
} else {
    Write-Host ""
    Write-Host "  Dry run complete. Use -Push to commit and push." -ForegroundColor Gray
    Write-Host "  Example: .\scripts\sync-oss.ps1 -Push" -ForegroundColor Gray
    Write-Host "  With tag: .\scripts\sync-oss.ps1 -Push -Tag v2026.0321.2000" -ForegroundColor Gray
    git reset HEAD -- . | Out-Null  # unstage
}

Pop-Location

Write-Host ""
Write-Host "========================================" -ForegroundColor Cyan
Write-Host "  Done!" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
