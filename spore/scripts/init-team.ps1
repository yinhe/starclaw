# Spore Team Initialization Script
# Sets up 7 Claw instances as a coordinated dev team

$ErrorActionPreference = "Continue"

$team = @(
    @{ name="claw-backend";  port=8081; role="Backend Dev";  username="backend-dev" },
    @{ name="claw-frontend"; port=8082; role="Frontend Dev"; username="frontend-dev" },
    @{ name="claw-qa";       port=8083; role="QA Engineer";  username="qa-engineer" },
    @{ name="claw-pm";       port=8084; role="Product Manager"; username="product-mgr" },
    @{ name="claw-design";   port=8085; role="UI Designer";  username="ui-designer" },
    @{ name="claw-devops";   port=8086; role="DevOps";       username="devops-eng" },
    @{ name="claw-lead";     port=8087; role="Tech Lead";    username="tech-lead" }
)

$tokens = @{}
$nodeIds = @{}

Write-Host "`n========================================" -ForegroundColor Cyan
Write-Host "  Spore Team Initialization" -ForegroundColor Cyan
Write-Host "========================================`n" -ForegroundColor Cyan

# ── Step 1: Initialize each instance (create owner) ──
Write-Host "[Step 1] Initializing owner accounts..." -ForegroundColor Yellow

foreach ($inst in $team) {
    $url = "http://localhost:$($inst.port)/v1/setup/status"
    try {
        $status = Invoke-RestMethod -Uri $url -Method GET -ErrorAction Stop
        if ($status.setup_completed) {
            Write-Host "  $($inst.name) - already initialized, logging in..." -ForegroundColor Gray
            # Try token login with stored token or skip
            $tokens[$inst.name] = $null
        } else {
            $body = @{ username = $inst.username; password = "claw2026" } | ConvertTo-Json
            $result = Invoke-RestMethod -Uri "http://localhost:$($inst.port)/v1/setup" -Method POST -Body $body -ContentType "application/json" -ErrorAction Stop
            $tokens[$inst.name] = $result.token
            Write-Host "  $($inst.name) - OK (owner: $($inst.username), token: $($result.owner_token.Substring(0,8))...)" -ForegroundColor Green
        }
    } catch {
        # May already be set up, try password login
        try {
            $loginBody = @{ password = "claw2026" } | ConvertTo-Json
            $loginResult = Invoke-RestMethod -Uri "http://localhost:$($inst.port)/v1/auth/owner-login" -Method POST -Body $loginBody -ContentType "application/json" -ErrorAction Stop
            $tokens[$inst.name] = $loginResult.token
            Write-Host "  $($inst.name) - logged in (existing owner)" -ForegroundColor Green
        } catch {
            Write-Host "  $($inst.name) - FAILED: $_" -ForegroundColor Red
        }
    }
}

# ── Step 2: Get Node IDs from each instance ──
Write-Host "`n[Step 2] Collecting Node IDs..." -ForegroundColor Yellow

foreach ($inst in $team) {
    $jwt = $tokens[$inst.name]
    if (-not $jwt) { continue }
    try {
        $headers = @{ Authorization = "Bearer $jwt" }
        $info = Invoke-RestMethod -Uri "http://localhost:$($inst.port)/v1/node/info" -Method GET -Headers $headers -ErrorAction Stop
        $nodeIds[$inst.name] = $info.node_id
        Write-Host "  $($inst.name) -> $($info.node_id.Substring(0,16))..." -ForegroundColor Green
    } catch {
        Write-Host "  $($inst.name) - failed to get node info: $_" -ForegroundColor Red
    }
}

# ── Step 3: Register peers on claw-lead (Captain) ──
Write-Host "`n[Step 3] Registering peers on claw-lead (Captain)..." -ForegroundColor Yellow

$leadJwt = $tokens["claw-lead"]
if (-not $leadJwt) {
    Write-Host "  ERROR: No JWT for claw-lead, cannot proceed" -ForegroundColor Red
    exit 1
}

$leadHeaders = @{ Authorization = "Bearer $leadJwt" }

foreach ($inst in $team) {
    if ($inst.name -eq "claw-lead") { continue }
    $peerAddr = "http://localhost:$($inst.port)"
    try {
        $body = @{ address = $peerAddr } | ConvertTo-Json
        $peer = Invoke-RestMethod -Uri "http://localhost:8087/v1/peers" -Method POST -Body $body -ContentType "application/json" -Headers $leadHeaders -ErrorAction Stop
        Write-Host "  + $($inst.name) ($peerAddr) -> peer registered" -ForegroundColor Green
    } catch {
        Write-Host "  + $($inst.name) - peer registration: $_" -ForegroundColor Yellow
    }
}

# ── Step 4: Create Squad ──
Write-Host "`n[Step 4] Creating Squad: StarClaw Dev Team..." -ForegroundColor Yellow

try {
    $squadBody = @{
        name = "StarClaw Dev Team"
        description = "Full-stack development team with 7 specialized AI agents"
        max_members = 10
        is_public = $false
        tags = @("dev", "team", "fullstack")
    } | ConvertTo-Json
    $squad = Invoke-RestMethod -Uri "http://localhost:8087/v1/squads" -Method POST -Body $squadBody -ContentType "application/json" -Headers $leadHeaders -ErrorAction Stop
    $squadId = $squad.squad.ID
    Write-Host "  Squad created: $squadId" -ForegroundColor Green
} catch {
    Write-Host "  Squad creation failed (may already exist): $_" -ForegroundColor Yellow
    # Try to find existing squad
    try {
        $squads = Invoke-RestMethod -Uri "http://localhost:8087/v1/squads" -Method GET -Headers $leadHeaders -ErrorAction Stop
        if ($squads.Count -gt 0) {
            $squadId = $squads[0].ID
            Write-Host "  Using existing squad: $squadId" -ForegroundColor Green
        }
    } catch {}
}

# ── Step 5: Invite members to Squad ──
Write-Host "`n[Step 5] Inviting team members to Squad..." -ForegroundColor Yellow

$specialties = @{
    "claw-backend"  = "backend,api,database,go,python"
    "claw-frontend" = "frontend,react,typescript,ui,css"
    "claw-qa"       = "testing,automation,quality,bug-tracking"
    "claw-pm"       = "product,requirements,user-stories,roadmap"
    "claw-design"   = "design,ui-ux,prototype,figma"
    "claw-devops"   = "ci-cd,deployment,monitoring,infrastructure"
}

foreach ($inst in $team) {
    if ($inst.name -eq "claw-lead") { continue }
    $nid = $nodeIds[$inst.name]
    if (-not $nid) { 
        Write-Host "  - $($inst.name) - skipped (no node ID)" -ForegroundColor Yellow
        continue
    }
    try {
        $inviteBody = @{
            node_id = $nid
            specialty = $specialties[$inst.name]
        } | ConvertTo-Json
        $member = Invoke-RestMethod -Uri "http://localhost:8087/v1/squads/$squadId/invite" -Method POST -Body $inviteBody -ContentType "application/json" -Headers $leadHeaders -ErrorAction Stop
        Write-Host "  + $($inst.name) ($($inst.role)) -> invited" -ForegroundColor Green
    } catch {
        Write-Host "  + $($inst.name) - invite: $_" -ForegroundColor Yellow
    }
}

# ── Summary ──
Write-Host "`n========================================" -ForegroundColor Cyan
Write-Host "  Team Setup Complete!" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""
Write-Host "  Captain (Overlord):  claw-lead   :8087" -ForegroundColor White
Write-Host "  Members:" -ForegroundColor White
Write-Host "    claw-backend   :8081  Backend Dev" -ForegroundColor Gray
Write-Host "    claw-frontend  :8082  Frontend Dev" -ForegroundColor Gray
Write-Host "    claw-qa        :8083  QA Engineer" -ForegroundColor Gray
Write-Host "    claw-pm        :8084  Product Manager" -ForegroundColor Gray
Write-Host "    claw-design    :8085  UI Designer" -ForegroundColor Gray
Write-Host "    claw-devops    :8086  DevOps" -ForegroundColor Gray
Write-Host ""
Write-Host "  Squad ID: $squadId" -ForegroundColor White
Write-Host "  Desktop:  http://localhost:7890" -ForegroundColor White
Write-Host "  Command:  Issue missions at http://localhost:8087" -ForegroundColor White
Write-Host ""
