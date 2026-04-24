param(
    [string]$ComposeFile = 'E:\claw-demo\docker-compose.prod.yml',
    [string]$EnvFile = 'E:\claw-demo\.env',
    [string]$BaseUrl = 'http://127.0.0.1:18080',
    [string]$TrustedUrlJson = 'e:\starclaw\docs\swarm-universe\production\ep04\clips_v2\_trusted_urls.json',
    [string]$TrustedKey = 'zerg'
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
try { $PSNativeCommandUseErrorActionPreference = $false } catch {}

function Write-Section([string]$Title) {
    Write-Host ''
    Write-Host ("=== " + $Title + " ===")
}

function Invoke-DockerLogged {
    param(
        [string]$LogPath,
        [string]$Command
    )

    $cmdLine = $Command + ' > "' + $LogPath + '" 2>&1'
    & cmd.exe /d /c $cmdLine
    $exitCode = $LASTEXITCODE
    if (Test-Path $LogPath) {
        Get-Content $LogPath -Tail 40 | ForEach-Object { $_ }
    }
    if ($exitCode -ne 0) {
        throw "docker command failed with exit code $exitCode"
    }
}

function Read-WebExceptionBody($ErrorRecord) {
    $raw = ''
    if ($ErrorRecord.ErrorDetails -and $ErrorRecord.ErrorDetails.Message) {
        $raw = $ErrorRecord.ErrorDetails.Message
    }
    if (-not $raw -and $ErrorRecord.Exception -and $ErrorRecord.Exception.Response) {
        try {
            $stream = $ErrorRecord.Exception.Response.GetResponseStream()
            if ($stream) {
                $reader = New-Object System.IO.StreamReader($stream)
                $raw = $reader.ReadToEnd()
                $reader.Close()
            }
        } catch {
        }
    }
    return $raw
}

function Show-Json([string]$Label, [string]$Raw) {
    if (-not $Raw) {
        return $null
    }
    try {
        $obj = $Raw | ConvertFrom-Json -ErrorAction Stop
        Write-Section $Label
        Write-Host ($obj | ConvertTo-Json -Depth 12)
        return $obj
    } catch {
        Write-Host $Raw
        return $null
    }
}

$buildLog = Join-Path $env:TEMP 'diagnose-resign-build.log'
$upLog = Join-Path $env:TEMP 'diagnose-resign-up.log'
$dockerLogsPath = Join-Path $env:TEMP 'diagnose-resign-docker.log'
$apiReturnedError = $false

Write-Section 'Build API'
Invoke-DockerLogged -LogPath $buildLog -Command ('docker compose -f "{0}" build api' -f $ComposeFile)

Write-Section 'Recreate API'
Invoke-DockerLogged -LogPath $upLog -Command ('docker compose --env-file "{0}" -f "{1}" up -d --force-recreate api' -f $EnvFile, $ComposeFile)

Start-Sleep -Seconds 5

Write-Section 'Verify Container Env'
& docker exec claw-demo-api sh -c 'echo AK=$VOLC_TOS_AK; echo SK=$VOLC_TOS_SK_B64'
if ($LASTEXITCODE -ne 0) {
    throw "docker exec failed with exit code $LASTEXITCODE"
}

Write-Section 'Prepare Request'
$login = Invoke-RestMethod -Method Post -Uri ($BaseUrl + '/v1/auth/login') -ContentType 'application/json' -Body '{"email":"tos@example.com","password":"Tostest123!"}'
$token = if ($login.token) { $login.token } elseif ($login.access_token) { $login.access_token } else { '' }
if (-not $token) {
    throw 'login succeeded but token/access_token missing'
}
$trusted = Get-Content $TrustedUrlJson -Raw | ConvertFrom-Json
$staleURL = $trusted.trusted_urls.$TrustedKey
if (-not $staleURL) {
    throw "trusted_urls.$TrustedKey missing in $TrustedUrlJson"
}
$payload = @{ tos_url = $staleURL; expires_sec = 604800 } | ConvertTo-Json -Compress
Write-Host ('token? ' + [bool]$token)
Write-Host ('stale url head: ' + $staleURL.Substring(0, [Math]::Min(160, $staleURL.Length)))
Write-Host ('payload: ' + $payload)

Write-Section 'POST /v1/cdn/resign-tos'
try {
    $resp = Invoke-WebRequest -Method Post -Uri ($BaseUrl + '/v1/cdn/resign-tos') -ContentType 'application/json' -Body $payload -Headers @{ Authorization = ('Bearer ' + $token) } -ErrorAction Stop
    Write-Host ('HTTP ' + [int]$resp.StatusCode)
    Write-Host '--- raw success body ---'
    Write-Host $resp.Content
    Show-Json -Label 'Parsed Success JSON' -Raw $resp.Content | Out-Null
} catch {
    $apiReturnedError = $true
    $status = -1
    if ($_.Exception -and $_.Exception.Response) {
        try { $status = [int]$_.Exception.Response.StatusCode } catch {}
    }
    Write-Host ('HTTP ' + $status)
    Write-Host '--- raw error body ---'
    $raw = Read-WebExceptionBody -ErrorRecord $_
    Write-Host $raw
    $body = Show-Json -Label 'Parsed Error JSON' -Raw $raw
    if ($null -ne $body -and $null -ne $body.PSObject.Properties['remote']) {
        Write-Section 'Remote Diagnostic'
        $body.remote | ConvertTo-Json -Depth 12
    }
}

Write-Section 'Recent API Logs'
$logLines = @()
Invoke-DockerLogged -LogPath $dockerLogsPath -Command 'docker logs --tail 80 claw-demo-api'
if (Test-Path $dockerLogsPath) {
    $logLines = Get-Content $dockerLogsPath
}
if ($logLines) {
    $matched = $logLines | Select-String -Pattern 'resign-tos|SignatureDoesNotMatch|AccessDenied|request-id|panic'
    if ($matched) {
        $matched | ForEach-Object { $_.Line }
    } else {
        $logLines | Select-Object -Last 20 | ForEach-Object { $_ }
    }
}

if ($apiReturnedError) {
    Write-Section 'Result'
    Write-Host 'Resign diagnostic completed with a non-2xx API response. See Parsed Error JSON above.'
}
