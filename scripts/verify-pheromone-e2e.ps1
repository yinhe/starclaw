param(
    [string]$ForgeBaseUrl = "http://127.0.0.1:8099",
    [string]$PheromoneBaseUrl = "http://127.0.0.1:8100",
    [string]$NodeId = "admin",
    [string]$Password = "changeme123",
    [string]$Subject = "pheromone.events.deploy.forge",
    [int]$PollRetries = 15,
    [int]$PollIntervalSeconds = 1
)

$ErrorActionPreference = "Stop"

function Assert-StatusCode {
    param(
        [string]$Name,
        [int]$Expected,
        [int]$Actual
    )

    if ($Actual -ne $Expected) {
        throw "$Name failed: expected status $Expected, got $Actual"
    }
}

Write-Host "[1/4] Health checks..."
$forgeHealth = Invoke-WebRequest -UseBasicParsing -Uri "$ForgeBaseUrl/health"
$pheromoneHealth = Invoke-WebRequest -UseBasicParsing -Uri "$PheromoneBaseUrl/health"
Assert-StatusCode -Name "Forge health" -Expected 200 -Actual $forgeHealth.StatusCode
Assert-StatusCode -Name "Pheromone health" -Expected 200 -Actual $pheromoneHealth.StatusCode

$ts = [DateTimeOffset]::UtcNow.ToUnixTimeSeconds()
$commit = "e2e-$ts"

Write-Host "[2/4] Publish event: $Subject ($commit)..."
$payload = @{ 
    service = "forge"
    branch = "master"
    commit = $commit
    actor = "verify-pheromone-e2e"
    source = "script"
    ts = $ts
}
$publishBody = @{ subject = $Subject; payload = $payload } | ConvertTo-Json -Depth 8
$publishResp = Invoke-WebRequest -UseBasicParsing -Method Post -Uri "$PheromoneBaseUrl/api/events" -ContentType "application/json" -Body $publishBody
Assert-StatusCode -Name "Pheromone publish" -Expected 202 -Actual $publishResp.StatusCode

Write-Host "[3/4] Verify event in Pheromone recent list..."
$recent = Invoke-RestMethod -UseBasicParsing -Uri "$PheromoneBaseUrl/api/events/recent?limit=50"
$recentHit = $recent.events | Where-Object { $_.subject -eq $Subject -and $_.payload.commit -eq $commit } | Select-Object -First 1
if ($null -eq $recentHit) {
    throw "Published event not found in Pheromone recent list (commit=$commit)"
}

Write-Host "[4/4] Verify event ingested by Forge activity..."
$loginBody = @{ node_id = $NodeId; password = $Password } | ConvertTo-Json
$loginResp = Invoke-RestMethod -UseBasicParsing -Method Post -Uri "$ForgeBaseUrl/api/auth/login" -ContentType "application/json" -Body $loginBody
$token = $loginResp.token
if ([string]::IsNullOrWhiteSpace($token)) {
    throw "Forge login succeeded but token is empty"
}

$found = $false
for ($i = 0; $i -lt $PollRetries; $i++) {
    $activity = Invoke-RestMethod -UseBasicParsing -Headers @{ Authorization = "Bearer $token" } -Uri "$ForgeBaseUrl/api/dashboard/activity?source=pheromone&limit=100"
    $activityJson = $activity | ConvertTo-Json -Depth 12
    if ($activityJson -match [regex]::Escape($commit)) {
        $found = $true
        break
    }
    Start-Sleep -Seconds $PollIntervalSeconds
}

if (-not $found) {
    throw "Forge activity did not contain commit=$commit after $PollRetries retries"
}

Write-Host ""
Write-Host "✅ Pheromone E2E verification passed"
Write-Host "   commit : $commit"
Write-Host "   subject: $Subject"
Write-Host "   forge  : $ForgeBaseUrl"
Write-Host "   api    : $PheromoneBaseUrl"
