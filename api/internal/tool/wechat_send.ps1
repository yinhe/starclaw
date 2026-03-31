Add-Type -AssemblyName System.Windows.Forms
$csCode = @"
using System;
using System.Text;
using System.Collections.Generic;
using System.Runtime.InteropServices;
using System.Diagnostics;
public class WeChatWin {
    public delegate bool EnumWindowsProc(IntPtr hWnd, IntPtr lParam);
    [DllImport("user32.dll")]
    public static extern bool EnumWindows(EnumWindowsProc lpEnumFunc, IntPtr lParam);
    [DllImport("user32.dll")]
    public static extern uint GetWindowThreadProcessId(IntPtr hWnd, out uint lpdwProcessId);
    [DllImport("kernel32.dll")]
    public static extern uint GetCurrentThreadId();
    [DllImport("user32.dll")]
    public static extern bool AttachThreadInput(uint idAttach, uint idAttachTo, bool fAttach);
    [DllImport("user32.dll")]
    public static extern bool SetForegroundWindow(IntPtr hWnd);
    [DllImport("user32.dll")]
    public static extern IntPtr GetForegroundWindow();
    [DllImport("user32.dll")]
    public static extern bool ShowWindow(IntPtr hWnd, int nCmdShow);
    [DllImport("user32.dll")]
    public static extern bool IsIconic(IntPtr hWnd);
    [DllImport("user32.dll")]
    public static extern bool IsWindowVisible(IntPtr hWnd);
    [DllImport("user32.dll", CharSet=CharSet.Unicode)]
    public static extern int GetWindowText(IntPtr hWnd, StringBuilder lpString, int nMaxCount);
    [DllImport("user32.dll")]
    public static extern int GetWindowTextLength(IntPtr hWnd);
    [DllImport("user32.dll")]
    public static extern bool GetWindowRect(IntPtr hWnd, out RECT lpRect);
    [DllImport("user32.dll")]
    public static extern bool SetCursorPos(int X, int Y);
    [DllImport("user32.dll")]
    public static extern void mouse_event(uint dwFlags, int dx, int dy, int dwData, IntPtr dwExtraInfo);
    [StructLayout(LayoutKind.Sequential)]
    public struct RECT { public int Left, Top, Right, Bottom; }
    public static string GetTitle(IntPtr hWnd) {
        if (hWnd == IntPtr.Zero) return "";
        int len = GetWindowTextLength(hWnd);
        if (len == 0) return "";
        var sb = new StringBuilder(len + 1);
        GetWindowText(hWnd, sb, sb.Capacity);
        return sb.ToString();
    }
    public static IntPtr FindWeChatWindow() {
        IntPtr result = IntPtr.Zero;
        var pids = new List<uint>();
        foreach (var name in new[] { "Weixin", "WeChat" }) {
            try { foreach (var p in Process.GetProcessesByName(name)) pids.Add((uint)p.Id); } catch {}
        }
        if (pids.Count == 0) return IntPtr.Zero;
        EnumWindows(delegate(IntPtr hWnd, IntPtr lParam) {
            if (!IsWindowVisible(hWnd)) return true;
            uint pid; GetWindowThreadProcessId(hWnd, out pid);
            if (!pids.Contains(pid)) return true;
            string title = GetTitle(hWnd);
            if (string.IsNullOrEmpty(title) || title == "AsHotplugCtrl") return true;
            RECT r; GetWindowRect(hWnd, out r);
            int w = r.Right - r.Left, h = r.Bottom - r.Top;
            if (w >= 500 && h >= 400) { result = hWnd; return false; }
            return true;
        }, IntPtr.Zero);
        return result;
    }
    [DllImport("user32.dll")] public static extern void keybd_event(byte bVk, byte bScan, uint dwFlags, UIntPtr dwExtraInfo);
    public static bool ReliableSetForeground(IntPtr hWnd) {
        if (IsIconic(hWnd)) { ShowWindow(hWnd, 9); System.Threading.Thread.Sleep(300); }
        IntPtr fgWnd = GetForegroundWindow();
        uint curTid = GetCurrentThreadId();
        uint dummy;
        uint fgTid = GetWindowThreadProcessId(fgWnd, out dummy);
        uint tgtTid = GetWindowThreadProcessId(hWnd, out dummy);
        if (curTid != fgTid) AttachThreadInput(curTid, fgTid, true);
        if (curTid != tgtTid) AttachThreadInput(curTid, tgtTid, true);
        // Simulate Alt key press to bypass Windows foreground lock
        keybd_event(0x12, 0, 0, UIntPtr.Zero);        // Alt down
        keybd_event(0x12, 0, 0x0002, UIntPtr.Zero);   // Alt up
        ShowWindow(hWnd, 5);
        bool ok = SetForegroundWindow(hWnd);
        if (curTid != tgtTid) AttachThreadInput(curTid, tgtTid, false);
        if (curTid != fgTid) AttachThreadInput(curTid, fgTid, false);
        return ok && GetForegroundWindow() == hWnd;
    }
}
"@
Add-Type -TypeDefinition $csCode -ReferencedAssemblies System.dll

$log = @()

# --- Layer 2: FocusGuard --- retry re-activation if focus lost to transient windows ---
function AssertFocus([string]$step) {
    $fg = [WeChatWin]::GetForegroundWindow()
    if ($fg -eq $script:hwnd) { return }
    # Focus lost — try to reclaim up to 3 times
    for ($retry = 0; $retry -lt 3; $retry++) {
        Start-Sleep -Milliseconds 300
        [WeChatWin]::ReliableSetForeground($script:hwnd) | Out-Null
        Start-Sleep -Milliseconds 200
        $fg = [WeChatWin]::GetForegroundWindow()
        if ($fg -eq $script:hwnd) { return }
    }
    $t = [WeChatWin]::GetTitle($fg)
    Write-Output "ERROR|FOCUS_LOST at $step (foreground=$t)"
    exit
}
function SafeClick($x, $y, [string]$step) {
    AssertFocus $step
    [WeChatWin]::SetCursorPos($x, $y)
    Start-Sleep -Milliseconds 80
    [WeChatWin]::mouse_event(0x0002, 0, 0, 0, [IntPtr]::Zero)
    Start-Sleep -Milliseconds 30
    [WeChatWin]::mouse_event(0x0004, 0, 0, 0, [IntPtr]::Zero)
}
function SafeKey([string]$keys, [string]$step) {
    AssertFocus $step
    [System.Windows.Forms.SendKeys]::SendWait($keys)
}

# --- Layer 0: Find WeChat window via EnumWindows ---
$hwnd = [WeChatWin]::FindWeChatWindow()
if ($hwnd -eq [IntPtr]::Zero) {
    Write-Output 'ERROR|WeChat window not found. Please open WeChat first.'
    exit
}
$log += ('found:' + [WeChatWin]::GetTitle($hwnd))

# --- Layer 1: Reliable foreground activation with retry ---
$activated = $false
foreach ($delay in @(0, 300, 600, 1000)) {
    if ($delay -gt 0) { Start-Sleep -Milliseconds $delay }
    if ([WeChatWin]::ReliableSetForeground($hwnd)) {
        $activated = $true
        break
    }
}
if (-not $activated) {
    Write-Output 'ERROR|failed to bring WeChat to foreground after retries'
    exit
}
Start-Sleep -Milliseconds 300

$rect = New-Object WeChatWin+RECT
[WeChatWin]::GetWindowRect($hwnd, [ref]$rect) | Out-Null
$wL = $rect.Left; $wT = $rect.Top
$wW = $rect.Right - $rect.Left; $wH = $rect.Bottom - $rect.Top
$log += "activated(${wW}x${wH})"

# --- Step A: Search and switch to target chat ---
$targetBytes = [System.Convert]::FromBase64String('{{TARGET_B64}}')
$targetText = [System.Text.Encoding]::UTF8.GetString($targetBytes)

# Use Ctrl+F to open search (works for global search from main window)
SafeKey '^f' 'open_search'
Start-Sleep -Milliseconds 500

[System.Windows.Forms.Clipboard]::SetText($targetText)
Start-Sleep -Milliseconds 100
SafeKey '^a' 'select_search'
Start-Sleep -Milliseconds 50
SafeKey '^v' 'paste_search'
Start-Sleep -Milliseconds 1500
$log += 'searched'

SafeKey '{ENTER}' 'select_result'
Start-Sleep -Milliseconds 1200
$log += 'opened_result'

# Do NOT press ESC here - selecting a result already closes the search panel.
# If search wasn't open, ESC would minimize WeChat to tray.

# --- Click input box to ensure focus (ESC may leave focus on chat list) ---
$inputX = $wL + [int]($wW * 0.50)
$inputY = $wT + [int]($wH * 0.85)
SafeClick $inputX $inputY 'focus_input'
Start-Sleep -Milliseconds 300
$log += 'focused_input'

# --- Step B: Paste message and send with Enter ---
$msgBytes = [System.Convert]::FromBase64String('{{MSG_B64}}')
$msgText = [System.Text.Encoding]::UTF8.GetString($msgBytes)
[System.Windows.Forms.Clipboard]::SetText($msgText)
Start-Sleep -Milliseconds 100
SafeKey '^v' 'paste_message'
Start-Sleep -Milliseconds 300
$log += 'pasted'

# --- Step C: Press Enter to send (WeChat default send key) ---
SafeKey '{ENTER}' 'send_enter'
Start-Sleep -Milliseconds 600
$log += 'sent'

# --- Step D: Verify message was sent ---
[System.Windows.Forms.Clipboard]::SetText('__verify__')
Start-Sleep -Milliseconds 50
SafeKey '^a' 'verify_select'
Start-Sleep -Milliseconds 80
SafeKey '^c' 'verify_copy'
Start-Sleep -Milliseconds 150
$afterText = ''
try { $afterText = [System.Windows.Forms.Clipboard]::GetText() } catch {}
if ($afterText -eq $msgText) {
    Write-Output 'ERROR|message still in input box after Enter; not sent'
    exit
}
$log += 'verified'

$logStr = $log -join '->'
Write-Output "OK|$logStr|$targetText|$($msgText.Length)"
