package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

// WebSearchTool performs web searches using a search API
type WebSearchTool struct {
	apiKey string
	engine string // "google" or "bing"
	client *http.Client
}

type WebSearchConfig struct {
	APIKey string
	Engine string // "google" or "bing", defaults to "google"
}

func NewWebSearchTool(cfg WebSearchConfig) *WebSearchTool {
	engine := cfg.Engine
	if engine == "" {
		engine = "google"
	}
	return &WebSearchTool{
		apiKey: cfg.APIKey,
		engine: engine,
		client: &http.Client{},
	}
}

func (t *WebSearchTool) Name() string {
	return "web_search"
}

func (t *WebSearchTool) Description() string {
	return "搜索互联网信息。当需要查找最新资讯、新闻、事实数据时使用此技能。"
}

func (t *WebSearchTool) Parameters() interface{} {
	return JSONSchema{
		Type: "object",
		Properties: map[string]Property{
			"query": {
				Type:        "string",
				Description: "The search query to look up",
			},
			"num_results": {
				Type:        "integer",
				Description: "Number of results to return (default: 5, max: 10)",
			},
		},
		Required: []string{"query"},
	}
}

type webSearchArgs struct {
	Query      string `json:"query"`
	NumResults int    `json:"num_results"`
}

type searchResult struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet"`
}

func (t *WebSearchTool) Execute(ctx context.Context, args string) (string, error) {
	parsed, err := ParseArgs[webSearchArgs](args)
	if err != nil {
		return "", err
	}

	if parsed.Query == "" {
		return "", fmt.Errorf("query is required")
	}

	numResults := parsed.NumResults
	if numResults <= 0 || numResults > 10 {
		numResults = 5
	}

	// Try DuckDuckGo HTML search first (real search results)
	results, err := t.searchDuckDuckGoHTML(ctx, parsed.Query, numResults)
	if err != nil || len(results) == 0 {
		// Fallback: try DuckDuckGo Instant Answer API
		results, _ = t.searchDuckDuckGoAPI(ctx, parsed.Query, numResults)
	}

	if len(results) == 0 {
		return "No search results found for the given query. Try using different keywords or a simpler query.", nil
	}

	output, _ := json.MarshalIndent(results, "", "  ")
	return string(output), nil
}

// searchDuckDuckGoHTML scrapes DuckDuckGo lite HTML search for real results
func (t *WebSearchTool) searchDuckDuckGoHTML(ctx context.Context, query string, maxResults int) ([]searchResult, error) {
	u := "https://lite.duckduckgo.com/lite/"

	form := url.Values{}
	form.Set("q", query)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	html := string(body)

	var results []searchResult

	// Parse DuckDuckGo lite results: links are in <a class="result-link" href="...">title</a>
	// and snippets are in <td class="result-snippet">...</td>

	// Extract result links
	linkRe := regexp.MustCompile(`<a[^>]+rel="nofollow"[^>]+href="([^"]+)"[^>]*>([^<]+)</a>`)
	snippetRe := regexp.MustCompile(`<td\s+class="result-snippet"[^>]*>([\s\S]*?)</td>`)

	links := linkRe.FindAllStringSubmatch(html, -1)
	snippets := snippetRe.FindAllStringSubmatch(html, -1)

	for i, link := range links {
		if len(results) >= maxResults {
			break
		}
		if len(link) < 3 {
			continue
		}
		href := strings.TrimSpace(link[1])
		title := strings.TrimSpace(link[2])

		// Skip DuckDuckGo internal links
		if strings.Contains(href, "duckduckgo.com") {
			continue
		}

		snippet := ""
		if i < len(snippets) && len(snippets[i]) >= 2 {
			snippet = strings.TrimSpace(snippets[i][1])
			// Strip HTML tags from snippet
			snippet = regexp.MustCompile(`<[^>]*>`).ReplaceAllString(snippet, "")
			snippet = strings.TrimSpace(snippet)
		}

		results = append(results, searchResult{
			Title:   title,
			URL:     href,
			Snippet: snippet,
		})
	}

	return results, nil
}

// searchDuckDuckGoAPI uses DuckDuckGo Instant Answer API (fallback, limited)
func (t *WebSearchTool) searchDuckDuckGoAPI(ctx context.Context, query string, maxResults int) ([]searchResult, error) {
	u := fmt.Sprintf("https://api.duckduckgo.com/?q=%s&format=json&no_html=1&no_redirect=1",
		url.QueryEscape(query))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var ddg struct {
		Abstract       string `json:"Abstract"`
		AbstractSource string `json:"AbstractSource"`
		AbstractURL    string `json:"AbstractURL"`
		RelatedTopics  []struct {
			Text     string `json:"Text"`
			FirstURL string `json:"FirstURL"`
		} `json:"RelatedTopics"`
	}

	if err := json.Unmarshal(body, &ddg); err != nil {
		return nil, err
	}

	var results []searchResult

	if ddg.Abstract != "" {
		results = append(results, searchResult{
			Title:   ddg.AbstractSource,
			URL:     ddg.AbstractURL,
			Snippet: ddg.Abstract,
		})
	}

	for _, topic := range ddg.RelatedTopics {
		if len(results) >= maxResults {
			break
		}
		if topic.Text != "" && topic.FirstURL != "" {
			results = append(results, searchResult{
				Title:   "",
				URL:     topic.FirstURL,
				Snippet: topic.Text,
			})
		}
	}

	return results, nil
}
