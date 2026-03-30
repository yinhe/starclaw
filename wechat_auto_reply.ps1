# wechat_auto_reply.ps1 v5 — Vision + Context Memory
# Badge detect → enter chat → screenshot → Claw AI vision → contextual reply → send
param([int]$PollSeconds = 3, [int]$CooldownSeconds = 120)

Add-Type -AssemblyName System.Windows.Forms
Add-Type -AssemblyName System.Drawing

$MCP_URL = 'http://127.0.0.1:9101'
$API_URL = 'http://127.0.0.1:8081'
$AGENT_ID = '44f9781b-a1cf-41b2-90a3-7c322bd2ffc9'
$JWT_SECRET = 'sc-1774595836348357800-35772'
$USER_ID = '75309ac7-9eb5-46a1-a2a4-125898d649da'

Add-Type @"
using System;
using System.Runtime.InteropServices;
using System.Threading;
public class Wx5 {
    [DllImport("user32.dll")] public static extern IntPtr GetForegroundWindow();
    [DllImport("user32.dll", CharSet=CharSet.Auto)] public static extern int GetWindowText(IntPtr h, System.Text.StringBuilder sb, int n);
    [DllImport("user32.dll")] public static extern bool GetWindowRect(IntPtr h, out RECT r);
    [DllImport("user32.dll")] public static extern void SetCursorPos(int x, int y);
    [DllImport("user32.dll")] public static extern void mouse_event(uint f, uint dx, uint dy, uint d, IntPtr e);
    public struct RECT { public int Left, Top, Right, Bottom; }
    public static string FgTitle() { var sb = new System.Text.StringBuilder(256); GetWindowText(GetForegroundWindow(), sb, 256); return sb.ToString(); }
    public static void Click(int x, int y) {
        SetCursorPos(x, y); Thread.Sleep(80);
        mouse_event(0x0002, 0, 0, 0, IntPtr.Zero); Thread.Sleep(30);
        mouse_event(0x0004, 0, 0, 0, IntPtr.Zero);
    }
}
"@

function Activate-WeChat {
    try {
        $body = '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"open_app","arguments":{"target":"\u5fae\u4fe1"}}}'
        Invoke-RestMethod $MCP_URL -Method Post -ContentType 'application/json' -Body ([System.Text.Encoding]::UTF8.GetBytes($body)) -TimeoutSec 10 | Out-Null
        Start-Sleep -Milliseconds 800
        return $true
    } catch { return $false }
}

function Get-WxRect {
    $fg = [Wx5]::GetForegroundWindow()
    $r = New-Object Wx5+RECT
    [Wx5]::GetWindowRect($fg, [ref]$r) | Out-Null
    return $r
}

function Get-Token {
    $h = '{"alg":"HS256","typ":"JWT"}'
    $exp = [int][double]::Parse((Get-Date -Date ((Get-Date).AddHours(720)).ToUniversalTime() -UFormat %s))
    $iat = [int][double]::Parse((Get-Date -Date (Get-Date).ToUniversalTime() -UFormat %s))
    $p = '{"sub":"' + $USER_ID + '","username":"yinhe","role":"admin","exp":' + $exp + ',"iat":' + $iat + '}'
    function B64U($bytes) { [Convert]::ToBase64String($bytes).TrimEnd('=').Replace('+','-').Replace('/','_') }
    $h64 = B64U([System.Text.Encoding]::UTF8.GetBytes($h))
    $p64 = B64U([System.Text.Encoding]::UTF8.GetBytes($p))
    $hmac = New-Object System.Security.Cryptography.HMACSHA256
    $hmac.Key = [System.Text.Encoding]::UTF8.GetBytes($JWT_SECRET)
    $sig = B64U($hmac.ComputeHash([System.Text.Encoding]::UTF8.GetBytes("$h64.$p64")))
    return "$h64.$p64.$sig"
}

# Screenshot chat area and return base64 JPEG data URL
function Get-ChatScreenshot {
    $wr = Get-WxRect
    $wW = $wr.Right - $wr.Left; $wH = $wr.Bottom - $wr.Top
    # Chat area: right 65% of window, top 75% (exclude input box)
    $cx = $wr.Left + [int]($wW * 0.35)
    $cy = $wr.Top
    $cw = [int]($wW * 0.65)
    $ch = [int]($wH * 0.75)
    if ($cw -lt 50 -or $ch -lt 50) { return $null }

    $bmp = New-Object System.Drawing.Bitmap($cw, $ch)
    $g = [System.Drawing.Graphics]::FromImage($bmp)
    $g.CopyFromScreen($cx, $cy, 0, 0, (New-Object System.Drawing.Size($cw, $ch)))
    $g.Dispose()

    # Resize to max 500px wide for small file size
    $scale = [Math]::Min(1.0, 500.0 / $cw)
    $nw = [int]($cw * $scale); $nh = [int]($ch * $scale)
    $small = New-Object System.Drawing.Bitmap($nw, $nh)
    $g2 = [System.Drawing.Graphics]::FromImage($small)
    $g2.InterpolationMode = [System.Drawing.Drawing2D.InterpolationMode]::HighQualityBicubic
    $g2.DrawImage($bmp, 0, 0, $nw, $nh)
    $g2.Dispose()
    $bmp.Dispose()

    # Encode as JPEG quality 50
    $jpegCodec = [System.Drawing.Imaging.ImageCodecInfo]::GetImageEncoders() | Where-Object { $_.MimeType -eq 'image/jpeg' }
    $encParams = New-Object System.Drawing.Imaging.EncoderParameters(1)
    $encParams.Param[0] = New-Object System.Drawing.Imaging.EncoderParameter([System.Drawing.Imaging.Encoder]::Quality, 50L)
    $ms = New-Object System.IO.MemoryStream
    $small.Save($ms, $jpegCodec, $encParams)
    $small.Dispose()
    $bytes = $ms.ToArray(); $ms.Dispose()
    $b64 = [Convert]::ToBase64String($bytes)
    return "data:image/jpeg;base64,$b64"
}

# Call Claw chat API with image for contextual reply
function Get-ContextualReply([string]$imageDataUrl, [string]$convId) {
    $token = Get-Token
    $prompt = [Convert]::ToBase64String([System.Text.Encoding]::UTF8.GetBytes(
        '这是我微信聊天的截图。请仔细阅读对话内容，理解上下文，然后生成一条自然的中文回复。要求：1)根据对方说的话来回复，不要答非所问 2)语气随意像朋友聊天 3)如果对方发了语音消息(显示为秒数)，就说让对方打字 4)回复1-2句话，简短自然 5)绝对不要说自己是AI。只输出回复内容，不要其他解释。'
    ))
    $promptText = [System.Text.Encoding]::UTF8.GetString([Convert]::FromBase64String($prompt))

    $bodyObj = @{
        agent_id = $AGENT_ID
        message = $promptText
        images = @($imageDataUrl)
        stream = $false
    }
    if ($convId) { $bodyObj['conversation_id'] = $convId }

    $bodyJson = $bodyObj | ConvertTo-Json -Depth 3 -Compress
    $bodyBytes = [System.Text.Encoding]::UTF8.GetBytes($bodyJson)

    try {
        $headers = @{Authorization = "Bearer $token"; 'Content-Type' = 'application/json'}
        $r = Invoke-RestMethod "$API_URL/v1/chat/completions" -Method Post -Headers $headers -Body $bodyBytes -TimeoutSec 30
        $reply = $r.message.content
        $newConvId = $r.conversation_id
        if ($reply) {
            # Clean up: remove quotes, markdown, etc
            $reply = $reply.Trim().Trim('"').Trim()
            if ($reply.Length -gt 200) { $reply = $reply.Substring(0, 200) }
            return @{reply=$reply; conv_id=$newConvId}
        }
    } catch {
        Write-Host "  [AI] error: $_"
    }
    $fbb64 = @('5Zyo5ZGi5Zyo5ZGi77yB','5pS25Yiw5pS25YiwIQ==','5ZOI5ZOI5oCO5LmI5LqG77yf')
    $fb = [System.Text.Encoding]::UTF8.GetString([Convert]::FromBase64String($fbb64[(Get-Random -Maximum $fbb64.Count)]))
    return @{reply=$fb; conv_id=$convId}
}

# State
$repliedRows = @{}
$convIds = @{}  # row -> conversation_id for multi-turn memory

$ts = Get-Date -Format 'HH:mm:ss'
Write-Host "[$ts] v5: Vision + Context Memory"
Write-Host "[$ts] Poll=${PollSeconds}s Cool=${CooldownSeconds}s"
Write-Host "[$ts] Waiting for badges..."

while ($true) {
    try {
        if (-not (Activate-WeChat)) { Start-Sleep -Seconds $PollSeconds; continue }

        $wr = Get-WxRect
        $wW = $wr.Right - $wr.Left; $wH = $wr.Bottom - $wr.Top
        if ($wW -lt 500 -or $wH -lt 400) { Start-Sleep -Seconds $PollSeconds; continue }
        $sW = [int]($wW * 0.35)

        # Screenshot sidebar for badge detection
        $bmp = New-Object System.Drawing.Bitmap($sW, $wH)
        $g = [System.Drawing.Graphics]::FromImage($bmp)
        $g.CopyFromScreen($wr.Left, $wr.Top, 0, 0, (New-Object System.Drawing.Size($sW, $wH)))
        $g.Dispose()

        $entryH = 70; $startY = [int]($wH * 0.12)
        $sx1 = [int]($sW * 0.35); $sx2 = [int]($sW * 0.65)

        for ($row = 0; $row -lt 8; $row++) {
            $y1 = $startY + ($row * $entryH)
            $y2 = [Math]::Min($y1 + $entryH, $wH)
            if ($y2 -gt $wH) { break }

            if ($repliedRows["r$row"]) {
                if (((Get-Date) - $repliedRows["r$row"]).TotalSeconds -lt $CooldownSeconds) { continue }
            }

            $reds = 0
            for ($y = $y1; $y -lt $y2; $y += 3) {
                for ($x = $sx1; $x -lt $sx2; $x += 3) {
                    $px = $bmp.GetPixel($x, $y)
                    if ($px.R -gt 200 -and $px.G -lt 100 -and $px.B -lt 100) { $reds++ }
                }
            }
            if ($reds -lt 3) { continue }

            $ts = Get-Date -Format 'HH:mm:ss'
            Write-Host "[$ts] BADGE row$row (reds=$reds)"

            # Click the chat entry
            $cx = $wr.Left + [int]($sW * 0.5)
            $cy = $wr.Top + $y1 + [int]($entryH / 2)
            [Wx5]::Click($cx, $cy)
            Start-Sleep -Milliseconds 1000

            # Screenshot the chat area for AI vision
            $imgUrl = Get-ChatScreenshot
            if (-not $imgUrl) {
                Write-Host "  screenshot failed"
                continue
            }
            $imgSize = [int]($imgUrl.Length / 1024)
            Write-Host "  screenshot ${imgSize}KB, asking AI..."

            # Get contextual reply from Claw AI with vision
            $result = Get-ContextualReply $imgUrl $convIds["r$row"]
            $reply = $result.reply
            $convIds["r$row"] = $result.conv_id
            Write-Host "  AI: $reply"

            # Click input box and send
            $ix = $wr.Left + [int]($wW * 0.50)
            $iy = $wr.Top + [int]($wH * 0.85)
            [Wx5]::Click($ix, $iy)
            Start-Sleep -Milliseconds 300

            [System.Windows.Forms.Clipboard]::SetText($reply)
            Start-Sleep -Milliseconds 100
            [System.Windows.Forms.SendKeys]::SendWait('^v')
            Start-Sleep -Milliseconds 300
            [System.Windows.Forms.SendKeys]::SendWait('{ENTER}')
            Start-Sleep -Milliseconds 500

            Write-Host "[$ts] SENT row$row"
            $repliedRows["r$row"] = Get-Date
            Start-Sleep -Milliseconds 300
        }

        $bmp.Dispose()

    } catch {
        $ts = Get-Date -Format 'HH:mm:ss'
        Write-Host "[$ts] ERR: $_"
    }
    Start-Sleep -Seconds $PollSeconds
}