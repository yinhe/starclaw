package workflow

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/yinhe/starclaw/internal/model"
)

// ════════════════════════════════════════════════════════════
//  Project Sync — scan a drama project directory and generate
//  a visual production workflow with all assets.
// ════════════════════════════════════════════════════════════

type syncProjectRequest struct {
	ProjectName string `json:"project_name" binding:"required"`
	AgentID     string `json:"agent_id"`
}

// SyncProject scans a drama project directory under /app/docs/{project_name}
// and creates/updates a workflow with character, episode, and asset nodes.
func (h *WorkflowHandler) SyncProject(c *gin.Context) {
	userID := c.GetString("user_id")

	var req syncProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	projectDir := filepath.Join("/app/docs", req.ProjectName)
	if _, err := os.Stat(projectDir); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("project directory not found: %s", req.ProjectName)})
		return
	}

	project := scanProject(req.ProjectName, projectDir)
	definition := generateWorkflowDefinition(project)
	defJSON, _ := json.Marshal(definition)

	workflowName := project.Title + " · 生产看板"

	var existing model.Workflow
	query := h.db.Where("name = ? AND user_id = ?", workflowName, userID)
	if req.AgentID != "" {
		query = query.Where("agent_id = ?", req.AgentID)
	}

	if err := query.First(&existing).Error; err != nil {
		wf := model.Workflow{
			ID:          uuid.New().String(),
			UserID:      userID,
			AgentID:     req.AgentID,
			Name:        workflowName,
			Description: fmt.Sprintf("从 %s 项目目录自动同步", req.ProjectName),
			Definition:  string(defJSON),
			IsPublic:    true,
		}
		h.db.Create(&wf)
		log.Printf("[ProjectSync] Created workflow: %s (%d nodes, %d edges)", workflowName, len(definition.Nodes), len(definition.Edges))
		c.JSON(http.StatusOK, gin.H{"workflow": wf, "created": true, "stats": project.Stats})
	} else {
		h.db.Model(&existing).Updates(map[string]interface{}{
			"definition":  string(defJSON),
			"description": fmt.Sprintf("从 %s 项目目录自动同步", req.ProjectName),
		})
		existing.Definition = string(defJSON)
		log.Printf("[ProjectSync] Updated workflow: %s (%d nodes, %d edges)", workflowName, len(definition.Nodes), len(definition.Edges))
		c.JSON(http.StatusOK, gin.H{"workflow": existing, "created": false, "stats": project.Stats})
	}
}

// ListProjects returns available project directories under /app/docs/
func (h *WorkflowHandler) ListProjects(c *gin.Context) {
	docsDir := "/app/docs"
	entries, err := os.ReadDir(docsDir)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"projects": []any{}})
		return
	}

	var projects []map[string]any
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		hasBible := false
		hasDrama := false
		if _, err := os.Stat(filepath.Join(docsDir, e.Name(), "bible.md")); err == nil {
			hasBible = true
		}
		if _, err := os.Stat(filepath.Join(docsDir, e.Name(), "drama")); err == nil {
			hasDrama = true
		}
		if !hasBible && !hasDrama {
			continue
		}
		projects = append(projects, map[string]any{
			"name":      e.Name(),
			"has_bible": hasBible,
			"has_drama": hasDrama,
		})
	}

	c.JSON(http.StatusOK, gin.H{"projects": projects})
}

// ── Project Scanner ─────────────────────────────────────────

type scannedProject struct {
	Name       string
	Title      string
	Characters []scannedCharacter
	Episodes   []scannedEpisode
	Stats      map[string]int
}

type scannedCharacter struct {
	Name        string
	Tag         string
	Description string
	ImagePath   string
}

type scannedEpisode struct {
	Code       string
	Title      string
	ScriptFile string
	PromptFile string
	ClipCount  int
	ClipPaths  []string
}

func scanProject(projectName, projectDir string) scannedProject {
	p := scannedProject{
		Name:  projectName,
		Title: projectName,
		Stats: map[string]int{},
	}

	if readme, err := os.ReadFile(filepath.Join(projectDir, "README.md")); err == nil {
		for _, line := range strings.Split(string(readme), "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "# ") {
				p.Title = strings.TrimPrefix(line, "# ")
				break
			}
		}
	}

	p.Characters = scanCharacters(projectDir, projectName)
	p.Stats["characters"] = len(p.Characters)

	p.Episodes = scanEpisodes(projectDir, projectName)
	p.Stats["episodes"] = len(p.Episodes)

	totalClips := 0
	for _, ep := range p.Episodes {
		totalClips += ep.ClipCount
	}
	p.Stats["clips"] = totalClips

	imgCount := 0
	filepath.Walk(filepath.Join(projectDir, "production", "characters"), func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext == ".png" || ext == ".jpg" || ext == ".jpeg" || ext == ".webp" {
			imgCount++
		}
		return nil
	})
	p.Stats["images"] = imgCount

	return p
}

var charTagRe = regexp.MustCompile(`\[图(\d+)\]`)
var charSectionRe = regexp.MustCompile(`^###\s+(.+)`)

func scanCharacters(projectDir, projectName string) []scannedCharacter {
	f, err := os.Open(filepath.Join(projectDir, "bible.md"))
	if err != nil {
		return nil
	}
	defer f.Close()

	var chars []scannedCharacter
	var current *scannedCharacter
	scanner := bufio.NewScanner(f)
	inTagBlock := false

	for scanner.Scan() {
		line := scanner.Text()

		if m := charSectionRe.FindStringSubmatch(line); m != nil {
			if current != nil {
				chars = append(chars, *current)
			}
			name := m[1]
			if idx := strings.IndexAny(name, "（("); idx > 0 {
				name = strings.TrimSpace(name[:idx])
			}
			current = &scannedCharacter{Name: name}
			inTagBlock = false
			continue
		}

		if current == nil {
			continue
		}

		if charTagRe.MatchString(line) {
			current.Tag = charTagRe.FindString(line)
			inTagBlock = true
			if idx := strings.Index(line, "]"); idx >= 0 {
				rest := strings.TrimSpace(line[idx+1:])
				rest = strings.TrimPrefix(rest, current.Name+"：")
				rest = strings.TrimPrefix(rest, current.Name+":")
				if len(rest) > 5 {
					current.Description = rest
				}
			}
			continue
		}

		if inTagBlock && current.Description == "" && len(strings.TrimSpace(line)) > 10 {
			desc := strings.TrimSpace(line)
			desc = strings.TrimPrefix(desc, current.Name+"：")
			if len(desc) > 5 {
				current.Description = desc
			}
		}
	}
	if current != nil {
		chars = append(chars, *current)
	}

	for i := range chars {
		chars[i].ImagePath = findCharacterImage(projectDir, projectName, chars[i].Name)
	}

	return chars
}

func findCharacterImage(projectDir, projectName, charName string) string {
	// Try to resolve character key from project manifest.json first
	dirName := resolveCharDirFromManifest(projectDir, charName)
	if dirName == "" {
		// Fallback: lowercase the character name, replace spaces with underscores
		dirName = strings.ToLower(strings.ReplaceAll(strings.TrimSpace(charName), " ", "_"))
	}

	charDir := filepath.Join(projectDir, "production", "characters", dirName)
	if _, err := os.Stat(charDir); os.IsNotExist(err) {
		return ""
	}

	preferences := []string{"unified_sheet_v", "unified_sheet", "turnaround", "character_sheet", "combined_ref", "_ref_v"}
	var allImages []string

	filepath.Walk(charDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext == ".png" || ext == ".jpg" {
			allImages = append(allImages, filepath.Base(path))
		}
		return nil
	})

	if len(allImages) == 0 {
		return ""
	}

	sort.Sort(sort.Reverse(sort.StringSlice(allImages)))

	for _, pref := range preferences {
		for _, img := range allImages {
			if strings.Contains(img, pref) {
				return fmt.Sprintf("/v1/projects/%s/production/characters/%s/%s", projectName, dirName, img)
			}
		}
	}

	return fmt.Sprintf("/v1/projects/%s/production/characters/%s/%s", projectName, dirName, allImages[0])
}

// resolveCharDirFromManifest reads manifest.json in the project and looks up
// a character directory name by its Chinese label. Returns "" if not found.
func resolveCharDirFromManifest(projectDir, charLabel string) string {
	data, err := os.ReadFile(filepath.Join(projectDir, "manifest.json"))
	if err != nil {
		return ""
	}
	var manifest struct {
		Entities struct {
			Characters map[string]struct {
				Label string `json:"label"`
			} `json:"characters"`
		} `json:"entities"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		return ""
	}
	for key, ch := range manifest.Entities.Characters {
		if strings.EqualFold(ch.Label, charLabel) {
			return key
		}
	}
	// Also try scanning character subdirectories directly
	charsDir := filepath.Join(projectDir, "production", "characters")
	entries, err := os.ReadDir(charsDir)
	if err != nil {
		return ""
	}
	lower := strings.ToLower(charLabel)
	for _, e := range entries {
		if e.IsDir() && strings.Contains(strings.ToLower(e.Name()), lower) {
			return e.Name()
		}
	}
	return ""
}

func scanEpisodes(projectDir, projectName string) []scannedEpisode {
	dramaDir := filepath.Join(projectDir, "drama")
	entries, err := os.ReadDir(dramaDir)
	if err != nil {
		return nil
	}

	epMap := make(map[string]*scannedEpisode)
	epCodeRe := regexp.MustCompile(`EP(\d+)`)

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		m := epCodeRe.FindStringSubmatch(strings.ToUpper(e.Name()))
		if m == nil {
			continue
		}
		code := "EP" + m[1]

		ep, exists := epMap[code]
		if !exists {
			ep = &scannedEpisode{Code: code}
			epMap[code] = ep
		}

		if strings.Contains(strings.ToUpper(e.Name()), "PROMPTS") {
			ep.PromptFile = "drama/" + e.Name()
		} else {
			ep.ScriptFile = "drama/" + e.Name()
			parts := strings.SplitN(strings.TrimSuffix(e.Name(), ".md"), "_", 3)
			if len(parts) >= 2 {
				ep.Title = strings.Join(parts[1:], "_")
			}
		}
	}

	// Scan clips
	for code, ep := range epMap {
		epLower := strings.ToLower(code)
		prodDir := filepath.Join(projectDir, "production", epLower)
		if _, err := os.Stat(prodDir); os.IsNotExist(err) {
			continue
		}
		filepath.Walk(prodDir, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			if strings.HasSuffix(path, ".mp4") || strings.HasSuffix(path, ".webm") {
				relPath, _ := filepath.Rel(projectDir, path)
				relPath = strings.ReplaceAll(relPath, "\\", "/")
				ep.ClipPaths = append(ep.ClipPaths, relPath)
				ep.ClipCount++
			}
			return nil
		})
	}

	var episodes []scannedEpisode
	for _, ep := range epMap {
		episodes = append(episodes, *ep)
	}
	sort.Slice(episodes, func(i, j int) bool { return episodes[i].Code < episodes[j].Code })
	return episodes
}

// ── Workflow Generator ──────────────────────────────────────

type wfDef struct {
	Nodes []wfNode `json:"nodes"`
	Edges []wfEdge `json:"edges"`
}

type wfNode struct {
	ID       string         `json:"id"`
	Type     string         `json:"type"`
	Position wfPos          `json:"position"`
	Data     map[string]any `json:"data"`
}

type wfEdge struct {
	ID     string         `json:"id"`
	Source string         `json:"source"`
	Target string         `json:"target"`
	Style  map[string]any `json:"style,omitempty"`
	Data   map[string]any `json:"data,omitempty"`
}

type wfPos struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

func generateWorkflowDefinition(p scannedProject) wfDef {
	var nodes []wfNode
	var edges []wfEdge

	// Row 0: Project start
	nodes = append(nodes, wfNode{
		ID: "start", Type: "start",
		Position: wfPos{X: 400, Y: 20},
		Data:     map[string]any{"label": p.Title},
	})

	// Row 1: Characters (y=120)
	charSpacing := 190.0
	charStartX := 400 - (float64(len(p.Characters)-1)*charSpacing)/2
	if len(p.Characters) == 0 {
		charStartX = 400
	}

	bibleID := "bible"
	nodes = append(nodes, wfNode{
		ID: bibleID, Type: "llm",
		Position: wfPos{X: 400, Y: 300},
		Data:     map[string]any{"label": "bible.md", "description": fmt.Sprintf("%d 角色 · 世界观 · [图N] 标签", len(p.Characters))},
	})
	edges = append(edges, wfEdge{ID: "e-start-bible", Source: "start", Target: bibleID})

	for i, ch := range p.Characters {
		id := fmt.Sprintf("char-%d", i+1)
		label := ch.Name
		if ch.Tag != "" {
			label = ch.Tag + " " + ch.Name
		}
		desc := ch.Description
		if len(desc) > 40 {
			desc = desc[:40] + "…"
		}

		nodes = append(nodes, wfNode{
			ID: id, Type: "media",
			Position: wfPos{X: charStartX + float64(i)*charSpacing, Y: 120},
			Data: map[string]any{
				"label":       label,
				"description": desc,
				"category":    "character",
				"imageUrl":    ch.ImagePath,
			},
		})
		edges = append(edges, wfEdge{ID: fmt.Sprintf("e-c%d-bible", i+1), Source: id, Target: bibleID})
	}

	// Row 2: Episodes (y=440)
	epSpacing := 190.0
	epStartX := 400 - (float64(len(p.Episodes)-1)*epSpacing)/2
	if len(p.Episodes) == 0 {
		epStartX = 400
	}

	for i, ep := range p.Episodes {
		id := fmt.Sprintf("ep-%s", strings.ToLower(ep.Code))
		title := ep.Code
		if ep.Title != "" {
			title = ep.Code + " " + ep.Title
		}
		desc := ""
		if ep.PromptFile != "" {
			desc += "PROMPTS "
		}
		if ep.ScriptFile != "" {
			desc += "剧本 "
		}
		if ep.ClipCount > 0 {
			desc += fmt.Sprintf("· %d clips", ep.ClipCount)
		}

		data := map[string]any{
			"label":       title,
			"description": strings.TrimSpace(desc),
			"category":    "scene",
		}
		if ep.ScriptFile != "" {
			data["scriptFile"] = ep.ScriptFile
			data["scriptUrl"] = fmt.Sprintf("/v1/projects/%s/%s", p.Name, ep.ScriptFile)
		}
		if ep.PromptFile != "" {
			data["promptFile"] = ep.PromptFile
			data["promptUrl"] = fmt.Sprintf("/v1/projects/%s/%s", p.Name, ep.PromptFile)
		}
		if ep.ClipCount > 0 {
			clipUrls := make([]string, 0, len(ep.ClipPaths))
			for _, cp := range ep.ClipPaths {
				clipUrls = append(clipUrls, fmt.Sprintf("/v1/projects/%s/%s", p.Name, cp))
			}
			data["clipUrls"] = clipUrls
		}

		nodes = append(nodes, wfNode{
			ID: id, Type: "media",
			Position: wfPos{X: epStartX + float64(i)*epSpacing, Y: 440},
			Data:     data,
		})
		edges = append(edges, wfEdge{
			ID:     fmt.Sprintf("e-bible-%s", strings.ToLower(ep.Code)),
			Source: bibleID, Target: id,
		})
	}

	// Row 3: Production pipeline (y=600)
	nodes = append(nodes, wfNode{
		ID: "parse", Type: "llm",
		Position: wfPos{X: 100, Y: 600},
		Data:     map[string]any{"label": "解析分镜表", "description": "故事剧本 → 镜别拆解 + 视觉 prompt"},
	})
	// 新增：GPT Image 2 故事板静帧（作为 Seedance i2v 首帧，保证构图和角色一致性）
	nodes = append(nodes, wfNode{
		ID: "storyboard", Type: "tool",
		Position: wfPos{X: 280, Y: 600},
		Data: map[string]any{
			"label":       "故事板静帧",
			"toolName":    "image_generation",
			"description": "GPT Image 2 · 构图理解最强 · 每镜 1 张 720×1280 静帧 → Seedance i2v 首帧锚定",
		},
	})
	nodes = append(nodes, wfNode{
		ID: "seedance", Type: "tool",
		Position: wfPos{X: 460, Y: 600},
		Data:     map[string]any{"label": "Seedance 2.0 生成", "toolName": "video_generation", "description": "故事板静帧做首帧 + 尾帧链式 · 逐镜生成"},
	})
	nodes = append(nodes, wfNode{
		ID: "dub", Type: "tool",
		Position: wfPos{X: 150, Y: 740},
		Data:     map[string]any{"label": "TTS 配音", "toolName": "dubbing", "description": "情感旁白 + 角色对话"},
	})
	nodes = append(nodes, wfNode{
		ID: "bgm", Type: "tool",
		Position: wfPos{X: 400, Y: 740},
		Data:     map[string]any{"label": "BGM 配乐", "toolName": "music_generation", "description": "氛围音乐 · 节拍同步"},
	})
	nodes = append(nodes, wfNode{
		ID: "compose", Type: "tool",
		Position: wfPos{X: 280, Y: 880},
		Data:     map[string]any{"label": "Pro 合成成片", "toolName": "mv_production", "description": "compose_pro · 转场 + 字幕 + 配乐"},
	})
	nodes = append(nodes, wfNode{
		ID: "end", Type: "end",
		Position: wfPos{X: 500, Y: 880},
		Data:     map[string]any{"label": "交付"},
	})

	edges = append(edges, wfEdge{ID: "e-parse-sb", Source: "parse", Target: "storyboard"})
	edges = append(edges, wfEdge{ID: "e-sb-sd", Source: "storyboard", Target: "seedance"})
	edges = append(edges, wfEdge{ID: "e-sd-dub", Source: "seedance", Target: "dub"})
	edges = append(edges, wfEdge{ID: "e-sd-bgm", Source: "seedance", Target: "bgm"})
	edges = append(edges, wfEdge{ID: "e-dub-comp", Source: "dub", Target: "compose"})
	edges = append(edges, wfEdge{ID: "e-bgm-comp", Source: "bgm", Target: "compose"})
	edges = append(edges, wfEdge{ID: "e-comp-end", Source: "compose", Target: "end"})

	if len(p.Episodes) > 0 {
		lastEp := p.Episodes[len(p.Episodes)-1]
		edges = append(edges, wfEdge{
			ID:     "e-ep-parse",
			Source: fmt.Sprintf("ep-%s", strings.ToLower(lastEp.Code)),
			Target: "parse",
		})
	}

	// Character → seedance dashed refs
	for i := range p.Characters {
		if i >= 3 {
			break
		}
		edges = append(edges, wfEdge{
			ID:     fmt.Sprintf("e-ref-c%d", i+1),
			Source: fmt.Sprintf("char-%d", i+1),
			Target: "seedance",
			Style:  map[string]any{"strokeDasharray": "5 5"},
		})
	}

	// Prop nodes
	propsDir := filepath.Join("/app/docs", p.Name, "production", "characters", "props")
	if entries, err := os.ReadDir(propsDir); err == nil {
		propX := 650.0
		propCount := 0
		for _, e := range entries {
			if e.IsDir() || propCount >= 3 {
				continue
			}
			ext := strings.ToLower(filepath.Ext(e.Name()))
			if ext != ".png" && ext != ".jpg" {
				continue
			}
			if !strings.Contains(e.Name(), "sheet") && !strings.Contains(e.Name(), "composite") {
				continue
			}
			propName := strings.TrimSuffix(e.Name(), ext)
			propName = strings.ReplaceAll(propName, "_", " ")

			id := fmt.Sprintf("prop-%d", propCount+1)
			nodes = append(nodes, wfNode{
				ID: id, Type: "media",
				Position: wfPos{X: propX, Y: 600 + float64(propCount)*140},
				Data: map[string]any{
					"label":    propName,
					"category": "prop",
					"imageUrl": fmt.Sprintf("/v1/projects/%s/production/characters/props/%s", p.Name, e.Name()),
				},
			})
			edges = append(edges, wfEdge{
				ID: fmt.Sprintf("e-prop%d-sd", propCount+1), Source: id, Target: "seedance",
				Style: map[string]any{"strokeDasharray": "5 5"},
			})
			propCount++
		}
	}

	return wfDef{Nodes: nodes, Edges: edges}
}
