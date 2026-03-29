Add-Type -AssemblyName System.Windows.Forms
Add-Type @"
using System;
using System.Runtime.InteropServices;
public class WXSend {
    [DllImport("user32.dll")] public static extern bool SetForegroundWindow(IntPtr h);
    [DllImport("user32.dll")] public static extern bool ShowWindow(IntPtr h, int n);
    [DllImport("user32.dll")] public static extern bool SetCursorPos(int X, int Y);
    [DllImport("user32.dll")] public static extern void mouse_event(uint f, int dx, int dy, int d, IntPtr e);
    [DllImport("user32.dll")] public static extern bool GetWindowRect(IntPtr h, out RECT r);
    [DllImport("user32.dll")] public static extern bool IsIconic(IntPtr h);
    [StructLayout(LayoutKind.Sequential)]
    public struct RECT { public int Left, Top, Right, Bottom; }
}
"@

function ClickAt($x, $y) {
    [WXSend]::SetCursorPos($x, $y)
    Start-Sleep -Milliseconds 80
    [WXSend]::mouse_event(0x0002, 0, 0, 0, [IntPtr]::Zero)
    Start-Sleep -Milliseconds 30
    [WXSend]::mouse_event(0x0004, 0, 0, 0, [IntPtr]::Zero)
}

$msgB64 = $args[0]
if (-not $msgB64) {
    throw 'msgB64 argument is required'
}
$h = [IntPtr]36771926
if ([WXSend]::IsIconic($h)) {
    [WXSend]::ShowWindow($h, 9)
    Start-Sleep -Milliseconds 300
}
[WXSend]::ShowWindow($h, 5)
[WXSend]::SetForegroundWindow($h)
Start-Sleep -Milliseconds 400

$rect = New-Object WXSend+RECT
[WXSend]::GetWindowRect($h, [ref]$rect) | Out-Null
$inputX = $rect.Left + [int](($rect.Right - $rect.Left) * 0.60)
$inputY = $rect.Top + ($rect.Bottom - $rect.Top) - [int](($rect.Bottom - $rect.Top) * 0.05)
ClickAt $inputX $inputY
Start-Sleep -Milliseconds 300

$bytes = [System.Convert]::FromBase64String($msgB64)
$text = [System.Text.Encoding]::UTF8.GetString($bytes)
[System.Windows.Forms.Clipboard]::SetText($text)
Start-Sleep -Milliseconds 100
[System.Windows.Forms.SendKeys]::SendWait('^a')
Start-Sleep -Milliseconds 50
[System.Windows.Forms.SendKeys]::SendWait('^v')
Start-Sleep -Milliseconds 200
[System.Windows.Forms.SendKeys]::SendWait('{ENTER}')
Write-Output 'sent'
