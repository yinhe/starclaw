# wechat_auto_chat.ps1 — Background loop: continuously scan badges, AI vision reply, send
# Injected by Go: {{API_URL}}, {{JWT_TOKEN}}, {{AGENT_ID}}, {{MCP_URL}}, {{POLL_SEC}}, {{PID_FILE}}
# Runs until killed or PID_FILE is deleted.

Add-Type -AssemblyName System.Windows.Forms
Add-Type -AssemblyName System.Drawing

$API_URL = '{{API_URL}}'
$JWT_TOKEN = '{{JWT_TOKEN}}'
$AGENT_ID = '{{AGENT_ID}}'
$MCP_URL = '{{MCP_URL}}'
$POLL_SEC = {{POLL_SEC}}
$PID_FILE = '{{PID_FILE}}'
$COOLDOWN_SEC = 120

# Write PID for stop tracking
$PID = [System.Diagnostics.Process]::GetCurrentProcess().Id
[System.IO.File]::WriteAllText($PID_FILE, "$PID")

Add-Type @"
using System;
using System.Runtime.InteropServices;
using System.Threading;
public class WxAC {
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
    $fg = [WxAC]::GetForegroundWindow()
    $r = New-Object WxAC+RECT
    [WxAC]::GetWindowRect($fg, [ref]$r) | Out-Null
    return $r
}

function Get-ChatScreenshot {
    $wr = Get-WxRect
    $wW = $wr.Right - $wr.Left; $wH = $wr.Bottom - $wr.Top
    $cx = $wr.Left + [int]($wW * 0.35)
    $cy = $wr.Top
    $cw = [int]($wW * 0.65)
    $ch = [int]($wH * 0.75)
    if ($cw -lt 50 -or $ch -lt 50) { return $null }

    $bmp = New-Object System.Drawing.Bitmap($cw, $ch)
    $g = [System.Drawing.Graphics]::FromImage($bmp)
    $g.CopyFromScreen($cx, $cy, 0, 0, (New-Object System.Drawing.Size($cw, $ch)))
    $g.Dispose()

    $scale = [Math]::Min(1.0, 500.0 / $cw)
    $nw = [int]($cw * $scale); $nh = [int]($ch * $scale)
    $small = New-Object System.Drawing.Bitmap($nw, $nh)
    $g2 = [System.Drawing.Graphics]::FromImage($small)
    $g2.InterpolationMode = [System.Drawing.Drawing2D.InterpolationMode]::HighQualityBicubic
    $g2.DrawImage($bmp, 0, 0, $nw, $nh)
    $g2.Dispose()
    $bmp.Dispose()

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

function Get-ContextualReply([string]$imageDataUrl, [string]$convId) {
    $promptB64 = [Convert]::ToBase64String([System.Text.Encoding]::UTF8.GetBytes(
        '这是我微信聊天的截图。请仔细阅读对话内容，理解上下文，然后生成一条自然的中文回复。要求：1)根据对方说的话来回复，不要答非所问 2)语气随意像朋友聊天 3)如果对方发了语音消息(显示为秒数)，就说让对方打字 4)回复1-2句话，简短自然 5)绝对不要说自己是AI。只输出回复内容，不要其他解释。'
    ))
    $promptText = [System.Text.Encoding]::UTF8.GetString([Convert]::FromBase64String($promptB64))

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
        $headers = @{Authorization = "Bearer $JWT_TOKEN"; 'Content-Type' = 'application/json'}
        $r = Invoke-RestMethod "$API_URL/v1/chat/completions" -Method Post -Headers $headers -Body $bodyBytes -TimeoutSec 30
        $reply = $r.message.content
        $newConvId = $r.conversation_id
        if ($reply) {
            $reply = $reply.Trim().Trim('"').Trim()
            if ($reply.Length -gt 200) { $reply = $reply.Substring(0, 200) }
            return @{reply=$reply; conv_id=$newConvId}
        }
    } catch {}
    $fbb64 = @('5Zyo5ZGi5Zyo5ZGi77yB','5pS25Yiw5pS25YiwIQ==','5ZOI5ZOI5oCO5LmI5LqG77yf')
    $fb = [System.Text.Encoding]::UTF8.GetString([Convert]::FromBase64String($fbb64[(Get-Random -Maximum $fbb64.Count)]))
    return @{reply=$fb; conv_id=$convId}
}

# ── Main Loop ──
$repliedRows = @{}
$convIds = @{}

while (Test-Path $PID_FILE) {
    try {
        if (-not (Activate-WeChat)) { Start-Sleep -Seconds $POLL_SEC; continue }

        $wr = Get-WxRect
        $wW = $wr.Right - $wr.Left; $wH = $wr.Bottom - $wr.Top
        if ($wW -lt 500 -or $wH -lt 400) { Start-Sleep -Seconds $POLL_SEC; continue }
        $sW = [int]($wW * 0.35)

        $bmp = New-Object System.Drawing.Bitmap($sW, $wH)
        $g = [System.Drawing.Graphics]::FromImage($bmp)
        $g.CopyFromScreen($wr.Left, $wr.Top, 0, 0, (New-Object System.Drawing.Size($sW, $wH)))
        $g.Dispose()

        $entryH = 70; $startY = [int]($wH * 0.12)
        $sx1 = [int]($sW * 0.35); $sx2 = [int]($sW * 0.65)

        for ($row = 0; $row -lt 10; $row++) {
            # Check if still running
            if (-not (Test-Path $PID_FILE)) { break }

            $y1 = $startY + ($row * $entryH)
            $y2 = [Math]::Min($y1 + $entryH, $wH)
            if ($y2 -gt $wH) { break }

            if ($repliedRows["r$row"]) {
                if (((Get-Date) - $repliedRows["r$row"]).TotalSeconds -lt $COOLDOWN_SEC) { continue }
            }

            $reds = 0
            for ($y = $y1; $y -lt $y2; $y += 3) {
                for ($x = $sx1; $x -lt $sx2; $x += 3) {
                    if ($x -ge $sW -or $y -ge $wH) { continue }
                    $px = $bmp.GetPixel($x, $y)
                    if ($px.R -gt 200 -and $px.G -lt 100 -and $px.B -lt 100) { $reds++ }
                }
            }
            if ($reds -lt 3) { continue }

            # Click the chat entry
            $cx = $wr.Left + [int]($sW * 0.5)
            $cy = $wr.Top + $y1 + [int]($entryH / 2)
            [WxAC]::Click($cx, $cy)
            Start-Sleep -Milliseconds 1000

            # Screenshot chat area
            $imgUrl = Get-ChatScreenshot
            if (-not $imgUrl) { continue }

            # AI vision reply
            $result = Get-ContextualReply $imgUrl $convIds["r$row"]
            $reply = $result.reply
            $convIds["r$row"] = $result.conv_id

            # Send reply
            $wr2 = Get-WxRect
            $wW2 = $wr2.Right - $wr2.Left; $wH2 = $wr2.Bottom - $wr2.Top
            $ix = $wr2.Left + [int]($wW2 * 0.50)
            $iy = $wr2.Top + [int]($wH2 * 0.85)
            [WxAC]::Click($ix, $iy)
            Start-Sleep -Milliseconds 300

            [System.Windows.Forms.Clipboard]::SetText($reply)
            Start-Sleep -Milliseconds 100
            [System.Windows.Forms.SendKeys]::SendWait('^v')
            Start-Sleep -Milliseconds 300
            [System.Windows.Forms.SendKeys]::SendWait('{ENTER}')
            Start-Sleep -Milliseconds 500

            $repliedRows["r$row"] = Get-Date
            Start-Sleep -Milliseconds 300
        }

        $bmp.Dispose()
    } catch {}
    Start-Sleep -Seconds $POLL_SEC
}
