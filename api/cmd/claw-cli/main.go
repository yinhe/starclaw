package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/yinhe/starclaw/internal/agent"
)

const usage = `StarClaw Agent CLI

Usage:
  claw-cli <command> [options]

Commands:
  list          List all discovered agents from agents/ directory
  info <id>     Show detailed info for a specific agent
  create <id>   Scaffold a new agent directory from template
  validate      Validate all manifests in agents/ directory
  bridges       Show bridge configuration summary

Options:
  -d, --dir <path>   Agents directory (default: auto-detect or CLAW_AGENTS_DIR)
  -h, --help         Show this help
  --json             Output in JSON format
`

func main() {
	args := os.Args[1:]
	if len(args) == 0 {
		fmt.Print(usage)
		os.Exit(0)
	}

	// Parse flags
	agentsDir := ""
	jsonOutput := false
	var positional []string

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-d", "--dir":
			if i+1 < len(args) {
				agentsDir = args[i+1]
				i++
			}
		case "--json":
			jsonOutput = true
		case "-h", "--help":
			fmt.Print(usage)
			os.Exit(0)
		default:
			positional = append(positional, args[i])
		}
	}

	if agentsDir == "" {
		if v := os.Getenv("CLAW_AGENTS_DIR"); v != "" {
			agentsDir = v
		} else {
			// Try relative paths
			for _, candidate := range []string{"agents", "../agents", "../../agents", filepath.Join(filepath.Dir(os.Args[0]), "..", "..", "agents")} {
				if info, err := os.Stat(candidate); err == nil && info.IsDir() {
					agentsDir = candidate
					break
				}
			}
		}
	}

	if agentsDir == "" {
		log.Fatal("Cannot find agents/ directory. Use -d flag or set CLAW_AGENTS_DIR.")
	}

	cmd := positional[0]
	cmdArgs := positional[1:]

	switch cmd {
	case "list":
		cmdList(agentsDir, jsonOutput)
	case "info":
		if len(cmdArgs) == 0 {
			log.Fatal("Usage: claw-cli info <agent_id>")
		}
		cmdInfo(agentsDir, cmdArgs[0], jsonOutput)
	case "create":
		if len(cmdArgs) == 0 {
			log.Fatal("Usage: claw-cli create <agent_id>")
		}
		cmdCreate(agentsDir, cmdArgs[0])
	case "validate":
		cmdValidate(agentsDir)
	case "bridges":
		cmdBridges(agentsDir, jsonOutput)
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", cmd)
		fmt.Print(usage)
		os.Exit(1)
	}
}

// ── list ─────────────────────────────────────────────

func cmdList(dir string, jsonOut bool) {
	manifests, err := agent.ScanAgentsDir(dir)
	if err != nil {
		log.Fatalf("Scan failed: %v", err)
	}

	if jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		items := make([]map[string]interface{}, 0, len(manifests))
		for _, m := range manifests {
			items = append(items, map[string]interface{}{
				"id":          m.ID,
				"type":        m.Type,
				"name_zh":     m.Name["zh"],
				"name_en":     m.Name["en"],
				"category":    m.Category,
				"version":     m.Version,
				"status":      m.Status,
				"is_builtin":  m.IsBuiltin,
				"has_bridge":  m.Bridge != nil,
				"tools_count": len(m.Tools.Shared) + len(m.Tools.Own),
			})
		}
		enc.Encode(map[string]interface{}{"agents": items, "count": len(items)})
		return
	}

	fmt.Printf("📦 Discovered %d agents in %s\n\n", len(manifests), dir)

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tTYPE\tNAME\tCATEGORY\tVER\tSTATUS\tBRIDGE")
	fmt.Fprintln(w, "──\t────\t────\t────────\t───\t──────\t──────")
	for _, m := range manifests {
		bridge := "—"
		if m.Bridge != nil {
			bridge = m.Bridge.Type
		}
		name := m.Name["zh"]
		if name == "" {
			name = m.Name["en"]
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			m.ID, m.Type, truncate(name, 20), m.Category, m.Version, m.Status, bridge)
	}
	w.Flush()
}

// ── info ─────────────────────────────────────────────

func cmdInfo(dir, id string, jsonOut bool) {
	manifests, err := agent.ScanAgentsDir(dir)
	if err != nil {
		log.Fatalf("Scan failed: %v", err)
	}

	var found *agent.AgentManifest
	for i := range manifests {
		if manifests[i].ID == id {
			found = &manifests[i]
			break
		}
	}
	if found == nil {
		log.Fatalf("Agent %q not found", id)
	}

	if jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.Encode(found)
		return
	}

	m := found
	fmt.Printf("🤖 Agent: %s\n", m.ID)
	fmt.Printf("   Type:     %s\n", m.Type)
	fmt.Printf("   Name:     %s / %s\n", m.Name["zh"], m.Name["en"])
	fmt.Printf("   Desc:     %s\n", truncate(m.Description["zh"], 60))
	fmt.Printf("   Category: %s\n", m.Category)
	fmt.Printf("   Version:  %s (%s)\n", m.Version, m.Status)
	fmt.Printf("   Built-in: %v\n", m.IsBuiltin)

	if m.Author.ID != "" {
		fmt.Printf("   Author:   %s (%s)\n", m.Author.Name["zh"], m.Author.ID)
	}
	if m.Model.Name != "" {
		fmt.Printf("   Model:    %s (temp=%.1f, max=%d)\n", m.Model.Name, m.Model.Temperature, m.Model.MaxTokens)
	}
	if len(m.Tools.Shared) > 0 {
		fmt.Printf("   Tools:    %s\n", strings.Join(m.Tools.Shared, ", "))
	}
	if len(m.Tools.Own) > 0 {
		fmt.Printf("   Own Tools: %d\n", len(m.Tools.Own))
	}
	if len(m.Skills) > 0 {
		fmt.Printf("   Skills:   %d\n", len(m.Skills))
	}
	if len(m.Glands) > 0 {
		fmt.Printf("   Glands:   %d\n", len(m.Glands))
	}
	if m.Bridge != nil {
		fmt.Printf("   Bridge:   %s (%s) port=%d auto_start=%v\n",
			m.Bridge.Type, m.Bridge.Entry, m.Bridge.Port, m.Bridge.AutoStart)
	}
	if m.PromptText != "" {
		lines := strings.Count(m.PromptText, "\n") + 1
		fmt.Printf("   Prompt:   %d lines\n", lines)
	}
}

// ── create ───────────────────────────────────────────

func cmdCreate(dir, id string) {
	targetDir := filepath.Join(dir, id)

	if _, err := os.Stat(targetDir); err == nil {
		log.Fatalf("Directory %s already exists", targetDir)
	}

	// Copy from _template
	templateDir := filepath.Join(dir, "_template")
	if _, err := os.Stat(templateDir); os.IsNotExist(err) {
		log.Fatalf("Template directory not found: %s", templateDir)
	}

	if err := os.MkdirAll(targetDir, 0755); err != nil {
		log.Fatalf("mkdir failed: %v", err)
	}

	// Copy template files
	for _, fname := range []string{"manifest.yaml", "prompt.md", "prompt.en.md"} {
		src := filepath.Join(templateDir, fname)
		dst := filepath.Join(targetDir, fname)

		data, err := os.ReadFile(src)
		if err != nil {
			log.Printf("Warning: cannot read %s: %v", src, err)
			continue
		}

		// Replace template placeholders with agent ID
		content := strings.ReplaceAll(string(data), "my_agent", id)
		content = strings.ReplaceAll(content, "my-agent", id)

		if err := os.WriteFile(dst, []byte(content), 0644); err != nil {
			log.Fatalf("write %s failed: %v", dst, err)
		}
	}

	fmt.Printf("✅ Created agent scaffold: %s\n", targetDir)
	fmt.Printf("   manifest.yaml — edit metadata, tools, model\n")
	fmt.Printf("   prompt.md     — write your Chinese prompt\n")
	fmt.Printf("   prompt.en.md  — write your English prompt\n")
	fmt.Printf("\nNext: edit the manifest, then restart Claw to auto-discover.\n")
}

// ── validate ─────────────────────────────────────────

func cmdValidate(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		log.Fatalf("Cannot read %s: %v", dir, err)
	}

	total, valid, invalid := 0, 0, 0

	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), "_") || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		manifestPath := filepath.Join(dir, e.Name(), "manifest.yaml")
		if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
			continue
		}

		total++
		manifests, err := agent.ScanAgentsDir(filepath.Join(dir, e.Name()))
		if err != nil || len(manifests) == 0 {
			fmt.Printf("  ❌ %s — parse error: %v\n", e.Name(), err)
			invalid++
			continue
		}

		m := manifests[0]
		issues := []string{}
		if m.ID == "" {
			issues = append(issues, "missing id")
		}
		if m.Name["zh"] == "" && m.Name["en"] == "" {
			issues = append(issues, "missing name")
		}
		if m.PromptText == "" {
			issues = append(issues, "empty prompt")
		}

		if len(issues) > 0 {
			fmt.Printf("  ⚠️  %s — %s\n", e.Name(), strings.Join(issues, ", "))
			invalid++
		} else {
			fmt.Printf("  ✅ %s\n", e.Name())
			valid++
		}
	}

	fmt.Printf("\n📋 Validated %d agents: %d valid, %d with issues\n", total, valid, invalid)
}

// ── bridges ──────────────────────────────────────────

func cmdBridges(dir string, jsonOut bool) {
	manifests, err := agent.ScanAgentsDir(dir)
	if err != nil {
		log.Fatalf("Scan failed: %v", err)
	}

	type bridgeInfo struct {
		AgentID   string `json:"agent_id"`
		Type      string `json:"type"`
		Entry     string `json:"entry"`
		Port      int    `json:"port"`
		AutoStart bool   `json:"auto_start"`
	}

	var bridges []bridgeInfo
	for _, m := range manifests {
		if m.Bridge != nil {
			bridges = append(bridges, bridgeInfo{
				AgentID:   m.ID,
				Type:      m.Bridge.Type,
				Entry:     m.Bridge.Entry,
				Port:      m.Bridge.Port,
				AutoStart: m.Bridge.AutoStart,
			})
		}
	}

	if jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.Encode(map[string]interface{}{"bridges": bridges, "count": len(bridges)})
		return
	}

	if len(bridges) == 0 {
		fmt.Println("No agents with bridge configuration found.")
		return
	}

	fmt.Printf("🔌 Found %d agents with bridges:\n\n", len(bridges))
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "AGENT\tTYPE\tENTRY\tPORT\tAUTO_START")
	fmt.Fprintln(w, "─────\t────\t─────\t────\t──────────")
	for _, b := range bridges {
		fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%v\n", b.AgentID, b.Type, b.Entry, b.Port, b.AutoStart)
	}
	w.Flush()
}

// ── helpers ──────────────────────────────────────────

func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen-1]) + "…"
}
