package tool

import (
	"context"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ── Windows UI Automation Layer ──
//
// Uses the built-in System.Windows.Automation .NET API (no external dependencies).
// Provides structured access to UI elements: buttons, text fields, menus, etc.
// This transforms "blind screenshot+guess" into "precise element interaction".

// UIElement represents a single element in the accessibility tree.
type UIElement struct {
	ID           int         `json:"id"`
	Type         string      `json:"type"`            // Button, TextBox, MenuItem, etc.
	Name         string      `json:"name"`            // Human-readable name
	AutomationID string      `json:"automation_id"`   // Developer-assigned ID (if any)
	Value        string      `json:"value,omitempty"` // Current text value
	IsEnabled    bool        `json:"enabled"`
	BoundsX      int         `json:"x"`
	BoundsY      int         `json:"y"`
	BoundsW      int         `json:"w"`
	BoundsH      int         `json:"h"`
	Children     []UIElement `json:"children,omitempty"`
}

// ── ui_tree: Get the accessibility tree of the foreground window ──

func (t *DesktopTool) uiTree(ctx context.Context, a desktopArgs) (string, error) {
	// PowerShell script that uses UI Automation to dump the element tree as JSON.
	// Recurses up to depth 4 to keep output manageable.
	maxDepth := 4
	if a.Seconds > 0 && a.Seconds <= 8 {
		maxDepth = a.Seconds // reuse seconds param as depth control
	}

	// Optional: target specific window by title
	windowFilter := ""
	if a.Title != "" {
		windowFilter = fmt.Sprintf(`
$allWindows = $root.FindAll([System.Windows.Automation.TreeScope]::Children, [System.Windows.Automation.Condition]::TrueCondition)
$targetWin = $null
foreach ($w in $allWindows) {
    if ($w.Current.Name -like '*%s*') { $targetWin = $w; break }
}
if ($targetWin -eq $null) {
    Write-Output '{"error":"window not found: %s"}'
    exit
}
$focusRoot = $targetWin
`, strings.ReplaceAll(a.Title, "'", "''"), a.Title)
	} else {
		windowFilter = `
Add-Type @"
using System;
using System.Runtime.InteropServices;
public class FGWin {
    [DllImport("user32.dll")] public static extern IntPtr GetForegroundWindow();
}
"@
$hwnd = [FGWin]::GetForegroundWindow()
$focusRoot = [System.Windows.Automation.AutomationElement]::FromHandle($hwnd)
`
	}

	psScript := fmt.Sprintf(`
Add-Type -AssemblyName UIAutomationClient
Add-Type -AssemblyName UIAutomationTypes

$root = [System.Windows.Automation.AutomationElement]::RootElement
%s

$script:nextId = 1
$maxDepth = %d

function Get-UITree($el, $depth) {
    if ($depth -gt $maxDepth) { return $null }
    $c = $el.Current
    $rect = $c.BoundingRectangle

    $node = @{
        id = $script:nextId++
        type = $c.ControlType.ProgrammaticName -replace 'ControlType\.',''
        name = if ($c.Name) { $c.Name } else { '' }
        automation_id = if ($c.AutomationId) { $c.AutomationId } else { '' }
        enabled = $c.IsEnabled
        x = [int]$rect.X
        y = [int]$rect.Y
        w = [int]$rect.Width
        h = [int]$rect.Height
    }

    # Get value for editable elements
    try {
        $vp = $el.GetCurrentPattern([System.Windows.Automation.ValuePattern]::Pattern)
        if ($vp) { $node.value = $vp.Current.Value }
    } catch {}

    # Get children (skip if too deep or element has too many children)
    $children = @()
    try {
        $kids = $el.FindAll([System.Windows.Automation.TreeScope]::Children, [System.Windows.Automation.Condition]::TrueCondition)
        if ($kids.Count -le 80) {
            foreach ($kid in $kids) {
                $child = Get-UITree $kid ($depth + 1)
                if ($child) { $children += $child }
            }
        }
    } catch {}

    if ($children.Count -gt 0) { $node.children = $children }
    return $node
}

$tree = Get-UITree $focusRoot 0
$windowName = $focusRoot.Current.Name

$result = @{
    window = $windowName
    tree = $tree
}

# Output as JSON (compact)
$json = $result | ConvertTo-Json -Depth 12 -Compress
Write-Output $json
`, windowFilter, maxDepth)

	out, err := runPowerShell(ctx, psScript)
	if err != nil {
		return "", fmt.Errorf("ui_tree failed: %w\n%.500s", err, out)
	}

	out = strings.TrimSpace(out)
	if out == "" {
		return "", fmt.Errorf("ui_tree returned empty output")
	}

	// Parse and re-format for LLM readability: flatten to a concise list
	var raw map[string]interface{}
	if err := json.Unmarshal([]byte(out), &raw); err != nil {
		// Return raw output if JSON parse fails
		if len(out) > 3000 {
			out = out[:3000] + "...(truncated)"
		}
		return toJSON(map[string]interface{}{
			"action": "ui_tree",
			"status": "success",
			"raw":    out,
		}), nil
	}

	// Flatten the tree into a concise element list for the LLM
	elements := flattenTree(raw["tree"], 0)
	windowName, _ := raw["window"].(string)

	// Truncate if too many elements
	if len(elements) > 100 {
		elements = elements[:100]
	}

	return toJSON(map[string]interface{}{
		"action":        "ui_tree",
		"status":        "success",
		"window":        windowName,
		"element_count": len(elements),
		"elements":      elements,
		"message":       fmt.Sprintf("获取到 \"%s\" 的 %d 个UI元素。使用 ui_click/ui_type 按 id 或 name 操作。", windowName, len(elements)),
	}), nil
}

// flattenTree converts nested tree to flat list with depth indicator
func flattenTree(node interface{}, depth int) []map[string]interface{} {
	if node == nil {
		return nil
	}
	m, ok := node.(map[string]interface{})
	if !ok {
		return nil
	}

	result := []map[string]interface{}{}

	name, _ := m["name"].(string)
	typeName, _ := m["type"].(string)
	automationID, _ := m["automation_id"].(string)
	enabled, _ := m["enabled"].(bool)
	value, _ := m["value"].(string)

	// Skip invisible/empty elements
	w, _ := m["w"].(float64)
	h, _ := m["h"].(float64)
	if w <= 0 || h <= 0 {
		// Still recurse children
		if children, ok := m["children"].([]interface{}); ok {
			for _, child := range children {
				result = append(result, flattenTree(child, depth+1)...)
			}
		}
		return result
	}

	// Skip generic containers with no name (Pane, Custom, Group)
	skipTypes := map[string]bool{"Pane": true, "Custom": true, "Group": true, "Separator": true, "Thumb": true, "ScrollBar": true}
	if name == "" && skipTypes[typeName] {
		if children, ok := m["children"].([]interface{}); ok {
			for _, child := range children {
				result = append(result, flattenTree(child, depth+1)...)
			}
		}
		return result
	}

	elem := map[string]interface{}{
		"id":   m["id"],
		"type": typeName,
		"name": name,
		"x":    int(m["x"].(float64)),
		"y":    int(m["y"].(float64)),
		"w":    int(w),
		"h":    int(h),
	}
	if automationID != "" {
		elem["aid"] = automationID
	}
	if value != "" {
		elem["value"] = value
	}
	if !enabled {
		elem["enabled"] = false
	}
	if depth > 0 {
		elem["depth"] = depth
	}

	result = append(result, elem)

	// Recurse children
	if children, ok := m["children"].([]interface{}); ok {
		for _, child := range children {
			result = append(result, flattenTree(child, depth+1)...)
		}
	}

	return result
}

// ── ui_click: Click a UI element by name or ID ──

func (t *DesktopTool) uiClick(ctx context.Context, a desktopArgs) (string, error) {
	if a.Title == "" && a.X == 0 && a.Y == 0 {
		return "", fmt.Errorf("ui_click requires 'title' (element name) or x,y coordinates")
	}

	// If name provided, find element via UIA and invoke/click
	if a.Title != "" {
		escapedName := strings.ReplaceAll(a.Title, "'", "''")
		psScript := fmt.Sprintf(`
Add-Type -AssemblyName UIAutomationClient
Add-Type -AssemblyName UIAutomationTypes
Add-Type @"
using System;
using System.Runtime.InteropServices;
public class ClickAPI {
    [DllImport("user32.dll")] public static extern IntPtr GetForegroundWindow();
    [DllImport("user32.dll")] public static extern bool SetCursorPos(int X, int Y);
    [DllImport("user32.dll")] public static extern void mouse_event(uint dwFlags, int dx, int dy, int dwData, IntPtr dwExtraInfo);
}
"@

$hwnd = [ClickAPI]::GetForegroundWindow()
$win = [System.Windows.Automation.AutomationElement]::FromHandle($hwnd)

# Search by Name
$cond = New-Object System.Windows.Automation.PropertyCondition([System.Windows.Automation.AutomationElement]::NameProperty, '%s')
$el = $win.FindFirst([System.Windows.Automation.TreeScope]::Descendants, $cond)

if ($el -eq $null) {
    # Try partial match: search all and filter
    $all = $win.FindAll([System.Windows.Automation.TreeScope]::Descendants, [System.Windows.Automation.Condition]::TrueCondition)
    foreach ($item in $all) {
        if ($item.Current.Name -like '*%s*' -and $item.Current.BoundingRectangle.Width -gt 0) {
            $el = $item
            break
        }
    }
}

if ($el -eq $null) {
    Write-Output "NOT_FOUND"
    exit
}

# Try InvokePattern first (native click)
$invoked = $false
try {
    $ip = $el.GetCurrentPattern([System.Windows.Automation.InvokePattern]::Pattern)
    $ip.Invoke()
    $invoked = $true
} catch {}

# Fallback: physical click at element center
if (-not $invoked) {
    $rect = $el.Current.BoundingRectangle
    $cx = [int]($rect.X + $rect.Width / 2)
    $cy = [int]($rect.Y + $rect.Height / 2)
    [ClickAPI]::SetCursorPos($cx, $cy)
    Start-Sleep -Milliseconds 50
    [ClickAPI]::mouse_event(0x0002, 0, 0, 0, [IntPtr]::Zero)
    Start-Sleep -Milliseconds 30
    [ClickAPI]::mouse_event(0x0004, 0, 0, 0, [IntPtr]::Zero)
}

$r = $el.Current.BoundingRectangle
$n = $el.Current.Name
$t = $el.Current.ControlType.ProgrammaticName -replace 'ControlType\.',''
Write-Output "OK|$n|$t|$([int]$r.X)|$([int]$r.Y)|$([int]$r.Width)|$([int]$r.Height)|$invoked"
`, escapedName, escapedName)

		out, err := runPowerShell(ctx, psScript)
		if err != nil {
			return "", fmt.Errorf("ui_click failed: %w\n%.500s", err, out)
		}
		out = strings.TrimSpace(out)

		if out == "NOT_FOUND" {
			return toJSON(map[string]interface{}{
				"action":  "ui_click",
				"status":  "not_found",
				"name":    a.Title,
				"message": fmt.Sprintf("未找到名为 \"%s\" 的元素。请先用 ui_tree 查看可用元素。", a.Title),
			}), nil
		}

		parts := strings.SplitN(out, "|", 8)
		if len(parts) >= 7 {
			x, _ := strconv.Atoi(parts[3])
			y, _ := strconv.Atoi(parts[4])
			w, _ := strconv.Atoi(parts[5])
			h, _ := strconv.Atoi(parts[6])
			method := "invoke"
			if len(parts) >= 8 && parts[7] == "False" {
				method = "mouse_click"
			}
			return toJSON(map[string]interface{}{
				"action": "ui_click",
				"status": "success",
				"name":   parts[1],
				"type":   parts[2],
				"x":      x, "y": y, "w": w, "h": h,
				"method":  method,
				"message": fmt.Sprintf("已点击 %s \"%s\" (方式: %s)", parts[2], parts[1], method),
			}), nil
		}

		return toJSON(map[string]interface{}{
			"action":  "ui_click",
			"status":  "success",
			"name":    a.Title,
			"message": fmt.Sprintf("已点击 \"%s\"", a.Title),
		}), nil
	}

	// Fallback: coordinate-based click (same as mouse_click)
	return t.mouseClick(ctx, a)
}

// ── ui_type: Type text into a UI element by name ──

func (t *DesktopTool) uiType(ctx context.Context, a desktopArgs) (string, error) {
	if a.Title == "" {
		return "", fmt.Errorf("ui_type requires 'title' (element name or automation_id of the input field)")
	}
	if a.Text == "" {
		return "", fmt.Errorf("ui_type requires 'text' (content to type)")
	}

	escapedName := strings.ReplaceAll(a.Title, "'", "''")
	textB64 := encodeBase64(a.Text)

	psScript := fmt.Sprintf(`
Add-Type -AssemblyName UIAutomationClient
Add-Type -AssemblyName UIAutomationTypes
Add-Type -AssemblyName System.Windows.Forms
Add-Type @"
using System;
using System.Runtime.InteropServices;
public class TypeAPI {
    [DllImport("user32.dll")] public static extern IntPtr GetForegroundWindow();
}
"@

$hwnd = [TypeAPI]::GetForegroundWindow()
$win = [System.Windows.Automation.AutomationElement]::FromHandle($hwnd)

# Find by Name
$cond = New-Object System.Windows.Automation.PropertyCondition([System.Windows.Automation.AutomationElement]::NameProperty, '%s')
$el = $win.FindFirst([System.Windows.Automation.TreeScope]::Descendants, $cond)

# Fallback: partial name match on editable elements
if ($el -eq $null) {
    $all = $win.FindAll([System.Windows.Automation.TreeScope]::Descendants, [System.Windows.Automation.Condition]::TrueCondition)
    foreach ($item in $all) {
        if ($item.Current.Name -like '*%s*' -and $item.Current.IsEnabled) {
            $el = $item
            break
        }
    }
}

if ($el -eq $null) {
    Write-Output "NOT_FOUND"
    exit
}

$bytes = [System.Convert]::FromBase64String('%s')
$text = [System.Text.Encoding]::UTF8.GetString($bytes)

# Try ValuePattern (direct text set — most reliable)
$done = $false
try {
    $vp = $el.GetCurrentPattern([System.Windows.Automation.ValuePattern]::Pattern)
    $vp.SetValue($text)
    $done = $true
} catch {}

# Fallback: focus element + clipboard paste
if (-not $done) {
    try { $el.SetFocus() } catch {}
    Start-Sleep -Milliseconds 100
    [System.Windows.Forms.Clipboard]::SetText($text)
    Start-Sleep -Milliseconds 50
    [System.Windows.Forms.SendKeys]::SendWait('^a')
    Start-Sleep -Milliseconds 50
    [System.Windows.Forms.SendKeys]::SendWait('^v')
    $done = $true
}

$n = $el.Current.Name
$t = $el.Current.ControlType.ProgrammaticName -replace 'ControlType\.',''
Write-Output "OK|$n|$t|$($text.Length)"
`, escapedName, escapedName, textB64)

	out, err := runPowerShell(ctx, psScript)
	if err != nil {
		return "", fmt.Errorf("ui_type failed: %w\n%.500s", err, out)
	}
	out = strings.TrimSpace(out)

	if out == "NOT_FOUND" {
		return toJSON(map[string]interface{}{
			"action":  "ui_type",
			"status":  "not_found",
			"name":    a.Title,
			"message": fmt.Sprintf("未找到名为 \"%s\" 的输入框。请先用 ui_tree 查看可用元素。", a.Title),
		}), nil
	}

	parts := strings.SplitN(out, "|", 4)
	elemName := a.Title
	elemType := "TextBox"
	if len(parts) >= 3 {
		elemName = parts[1]
		elemType = parts[2]
	}

	preview := a.Text
	if len(preview) > 50 {
		preview = preview[:50] + "..."
	}
	return toJSON(map[string]interface{}{
		"action":  "ui_type",
		"status":  "success",
		"name":    elemName,
		"type":    elemType,
		"text":    preview,
		"message": fmt.Sprintf("已在 %s \"%s\" 中输入: \"%s\"", elemType, elemName, preview),
	}), nil
}

// ── ui_select: Select a value in a combo box / list ──

func (t *DesktopTool) uiSelect(ctx context.Context, a desktopArgs) (string, error) {
	if a.Title == "" || a.Text == "" {
		return "", fmt.Errorf("ui_select requires 'title' (element name) and 'text' (value to select)")
	}

	escapedName := strings.ReplaceAll(a.Title, "'", "''")
	escapedValue := strings.ReplaceAll(a.Text, "'", "''")

	psScript := fmt.Sprintf(`
Add-Type -AssemblyName UIAutomationClient
Add-Type -AssemblyName UIAutomationTypes
Add-Type @"
using System;
using System.Runtime.InteropServices;
public class SelAPI {
    [DllImport("user32.dll")] public static extern IntPtr GetForegroundWindow();
}
"@

$hwnd = [SelAPI]::GetForegroundWindow()
$win = [System.Windows.Automation.AutomationElement]::FromHandle($hwnd)
$cond = New-Object System.Windows.Automation.PropertyCondition([System.Windows.Automation.AutomationElement]::NameProperty, '%s')
$el = $win.FindFirst([System.Windows.Automation.TreeScope]::Descendants, $cond)

if ($el -eq $null) {
    Write-Output "NOT_FOUND"
    exit
}

# Try SelectionPattern → find item by name
try {
    $ep = $el.GetCurrentPattern([System.Windows.Automation.ExpandCollapsePattern]::Pattern)
    $ep.Expand()
    Start-Sleep -Milliseconds 200
} catch {}

$itemCond = New-Object System.Windows.Automation.PropertyCondition([System.Windows.Automation.AutomationElement]::NameProperty, '%s')
$item = $el.FindFirst([System.Windows.Automation.TreeScope]::Descendants, $itemCond)

if ($item) {
    try {
        $si = $item.GetCurrentPattern([System.Windows.Automation.SelectionItemPattern]::Pattern)
        $si.Select()
        Write-Output "OK|selected|$('%s')"
        exit
    } catch {}
    try {
        $ip = $item.GetCurrentPattern([System.Windows.Automation.InvokePattern]::Pattern)
        $ip.Invoke()
        Write-Output "OK|invoked|$('%s')"
        exit
    } catch {}
}

Write-Output "ITEM_NOT_FOUND"
`, escapedName, escapedValue, escapedValue, escapedValue)

	out, err := runPowerShell(ctx, psScript)
	if err != nil {
		return "", fmt.Errorf("ui_select failed: %w\n%.500s", err, out)
	}
	out = strings.TrimSpace(out)

	if out == "NOT_FOUND" {
		return toJSON(map[string]interface{}{
			"action": "ui_select", "status": "not_found", "name": a.Title,
			"message": fmt.Sprintf("未找到 \"%s\"", a.Title),
		}), nil
	}
	if out == "ITEM_NOT_FOUND" {
		return toJSON(map[string]interface{}{
			"action": "ui_select", "status": "item_not_found", "name": a.Title, "value": a.Text,
			"message": fmt.Sprintf("找到 \"%s\" 但未找到选项 \"%s\"", a.Title, a.Text),
		}), nil
	}

	return toJSON(map[string]interface{}{
		"action": "ui_select", "status": "success", "name": a.Title, "value": a.Text,
		"message": fmt.Sprintf("已在 \"%s\" 中选择 \"%s\"", a.Title, a.Text),
	}), nil
}

// ── ui_scroll: Scroll within an element or the active window ──

func (t *DesktopTool) uiScroll(ctx context.Context, a desktopArgs) (string, error) {
	direction := a.Button // reuse button field: "up", "down", "left", "right"
	if direction == "" {
		direction = "down"
	}
	amount := a.Seconds // reuse seconds as scroll amount (1-10)
	if amount <= 0 {
		amount = 3
	}

	// Use mouse wheel via user32.dll
	wheelDelta := 120 * amount
	if direction == "down" || direction == "right" {
		wheelDelta = -wheelDelta
	}

	flag := "0x0800" // MOUSEEVENTF_WHEEL
	if direction == "left" || direction == "right" {
		flag = "0x01000" // MOUSEEVENTF_HWHEEL
	}

	psScript := fmt.Sprintf(`
Add-Type @"
using System;
using System.Runtime.InteropServices;
public class ScrollAPI {
    [DllImport("user32.dll")] public static extern void mouse_event(uint dwFlags, int dx, int dy, int dwData, IntPtr dwExtraInfo);
}
"@
[ScrollAPI]::mouse_event(%s, 0, 0, %d, [IntPtr]::Zero)
Write-Output "scrolled %s %d"
`, flag, wheelDelta, direction, amount)

	out, err := runPowerShell(ctx, psScript)
	if err != nil {
		return "", fmt.Errorf("ui_scroll failed: %w\n%.300s", err, out)
	}

	return toJSON(map[string]interface{}{
		"action":    "ui_scroll",
		"status":    "success",
		"direction": direction,
		"amount":    amount,
		"message":   fmt.Sprintf("已向%s滚动 %d 格", direction, amount),
	}), nil
}

// ── ui_wait: Wait for a UI element to appear ──

func (t *DesktopTool) uiWait(ctx context.Context, a desktopArgs) (string, error) {
	if a.Title == "" {
		return "", fmt.Errorf("ui_wait requires 'title' (element name to wait for)")
	}
	timeoutSec := a.Seconds
	if timeoutSec <= 0 {
		timeoutSec = 10
	}
	if timeoutSec > 30 {
		timeoutSec = 30
	}

	escapedName := strings.ReplaceAll(a.Title, "'", "''")
	deadline := time.Now().Add(time.Duration(timeoutSec) * time.Second)

	for time.Now().Before(deadline) {
		psScript := fmt.Sprintf(`
Add-Type -AssemblyName UIAutomationClient
Add-Type -AssemblyName UIAutomationTypes
Add-Type @"
using System;
using System.Runtime.InteropServices;
public class WaitAPI {
    [DllImport("user32.dll")] public static extern IntPtr GetForegroundWindow();
}
"@
$hwnd = [WaitAPI]::GetForegroundWindow()
$win = [System.Windows.Automation.AutomationElement]::FromHandle($hwnd)
$all = $win.FindAll([System.Windows.Automation.TreeScope]::Descendants, [System.Windows.Automation.Condition]::TrueCondition)
foreach ($item in $all) {
    if ($item.Current.Name -like '*%s*' -and $item.Current.BoundingRectangle.Width -gt 0) {
        $r = $item.Current.BoundingRectangle
        $n = $item.Current.Name
        $t = $item.Current.ControlType.ProgrammaticName -replace 'ControlType\.',''
        Write-Output "FOUND|$n|$t|$([int]$r.X)|$([int]$r.Y)"
        exit
    }
}
Write-Output "WAITING"
`, escapedName)

		out, err := runPowerShell(ctx, psScript)
		if err == nil {
			out = strings.TrimSpace(out)
			if strings.HasPrefix(out, "FOUND|") {
				parts := strings.SplitN(out, "|", 5)
				name := a.Title
				elemType := ""
				if len(parts) >= 3 {
					name = parts[1]
					elemType = parts[2]
				}
				elapsed := time.Since(deadline.Add(-time.Duration(timeoutSec) * time.Second))
				return toJSON(map[string]interface{}{
					"action":     "ui_wait",
					"status":     "found",
					"name":       name,
					"type":       elemType,
					"elapsed_ms": elapsed.Milliseconds(),
					"message":    fmt.Sprintf("元素 \"%s\" 已出现 (%.1f秒)", name, elapsed.Seconds()),
				}), nil
			}
		}
		time.Sleep(500 * time.Millisecond)
	}

	return toJSON(map[string]interface{}{
		"action":  "ui_wait",
		"status":  "timeout",
		"name":    a.Title,
		"timeout": timeoutSec,
		"message": fmt.Sprintf("等待 \"%s\" 超时 (%d秒)", a.Title, timeoutSec),
	}), nil
}

//go:embed wechat_send.ps1
var wechatSendPS1 string

// ── wechat_send: Robust WeChat message sending ──
//
// Uses EnumWindows for window discovery, AttachThreadInput for reliable activation,
// and FocusGuard to abort immediately if focus is ever lost.
// See docs/WECHAT_SEND_DESIGN.md for design rationale.

func (t *DesktopTool) wechatSend(ctx context.Context, a desktopArgs) (string, error) {
	target := a.Title // group or contact name
	message := a.Text // message to send
	if target == "" {
		return "", fmt.Errorf("wechat_send requires 'title' (group or contact name)")
	}
	if message == "" {
		return "", fmt.Errorf("wechat_send requires 'text' (message content)")
	}

	targetB64 := encodeBase64(target)
	msgB64 := encodeBase64(message)

	// Robust PowerShell script: EnumWindows → AttachThreadInput → FocusGuard → send
	// See docs/WECHAT_SEND_DESIGN.md
	psScript := wechatSendPS1
	psScript = strings.Replace(psScript, "{{TARGET_B64}}", targetB64, 1)
	psScript = strings.Replace(psScript, "{{MSG_B64}}", msgB64, 1)

	out, err := runPowerShell(ctx, psScript)
	if err != nil {
		return "", fmt.Errorf("wechat_send failed: %w\n%.800s", err, out)
	}
	out = strings.TrimSpace(out)

	if strings.HasPrefix(out, "ERROR|") {
		errMsg := strings.TrimPrefix(out, "ERROR|")
		return toJSON(map[string]interface{}{
			"action":  "wechat_send",
			"status":  "error",
			"target":  target,
			"message": errMsg,
		}), nil
	}

	parts := strings.SplitN(out, "|", 4)
	logStr := ""
	clickedName := target
	msgLen := 0
	if len(parts) >= 2 {
		logStr = parts[1]
	}
	if len(parts) >= 3 {
		clickedName = parts[2]
	}
	if len(parts) >= 4 {
		msgLen, _ = strconv.Atoi(parts[3])
	}

	preview := message
	if len(preview) > 80 {
		preview = preview[:80] + "..."
	}

	return toJSON(map[string]interface{}{
		"action":       "wechat_send",
		"status":       "success",
		"target":       target,
		"clicked_name": clickedName,
		"msg_length":   msgLen,
		"steps":        logStr,
		"message":      fmt.Sprintf("已向 \"%s\" 发送消息: \"%s\"", clickedName, preview),
	}), nil
}

// ── Helper ──

func encodeBase64(s string) string {
	return base64.StdEncoding.EncodeToString([]byte(s))
}
