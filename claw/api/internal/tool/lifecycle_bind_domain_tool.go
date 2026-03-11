package tool

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// BindDomainTool manages DNS records for domain binding (MVP: Cloudflare).
type BindDomainTool struct{}

func NewBindDomainTool() *BindDomainTool { return &BindDomainTool{} }

func (t *BindDomainTool) Name() string { return "bind_domain" }

func (t *BindDomainTool) Description() string {
	return "域名绑定管理（MVP: Cloudflare DNS），支持 upsert/status/delete 操作。"
}

func (t *BindDomainTool) Parameters() interface{} {
	return JSONSchema{
		Type: "object",
		Properties: map[string]Property{
			"action": {
				Type:        "string",
				Description: "Action: upsert, status, delete",
				Enum:        []string{"upsert", "status", "delete"},
			},
			"provider": {
				Type:        "string",
				Description: "DNS provider (MVP: cloudflare)",
				Enum:        []string{"cloudflare"},
			},
			"api_token": {
				Type:        "string",
				Description: "Cloudflare API token with DNS edit permission",
			},
			"zone_id": {
				Type:        "string",
				Description: "Cloudflare Zone ID",
			},
			"record_type": {
				Type:        "string",
				Description: "DNS record type: A/CNAME/TXT",
				Enum:        []string{"A", "CNAME", "TXT"},
			},
			"record_name": {
				Type:        "string",
				Description: "DNS record full name, e.g. app.example.com",
			},
			"record_value": {
				Type:        "string",
				Description: "Record value, e.g. cname target or ip",
			},
			"ttl": {
				Type:        "string",
				Description: "TTL seconds, default 120",
			},
			"proxied": {
				Type:        "string",
				Description: "Cloudflare proxy flag, true/false",
			},
		},
		Required: []string{"action", "provider", "api_token", "zone_id", "record_name"},
	}
}

type bindDomainArgs struct {
	Action      string `json:"action"`
	Provider    string `json:"provider"`
	APIToken    string `json:"api_token"`
	ZoneID      string `json:"zone_id"`
	RecordType  string `json:"record_type"`
	RecordName  string `json:"record_name"`
	RecordValue string `json:"record_value"`
	TTL         string `json:"ttl"`
	Proxied     string `json:"proxied"`
}

type cloudflareDNSRecord struct {
	ID      string      `json:"id"`
	Type    string      `json:"type"`
	Name    string      `json:"name"`
	Content string      `json:"content"`
	TTL     int         `json:"ttl"`
	Proxied interface{} `json:"proxied"`
}

type cloudflareResponse struct {
	Success bool                  `json:"success"`
	Result  json.RawMessage       `json:"result"`
	Errors  []map[string]any      `json:"errors"`
	Message string                `json:"message"`
}

func (t *BindDomainTool) Execute(ctx context.Context, argsJSON string) (string, error) {
	args, err := ParseArgs[bindDomainArgs](argsJSON)
	if err != nil {
		return "", err
	}

	if strings.ToLower(strings.TrimSpace(args.Provider)) != "cloudflare" {
		return "", fmt.Errorf("unsupported provider: %s", args.Provider)
	}

	action := strings.ToLower(strings.TrimSpace(args.Action))
	recordType := strings.ToUpper(strings.TrimSpace(defaultStr(args.RecordType, "CNAME")))
	recordName := strings.TrimSpace(args.RecordName)
	if recordName == "" {
		return "", fmt.Errorf("record_name is required")
	}

	switch action {
	case "status":
		return t.statusRecord(ctx, args, recordType, recordName)
	case "upsert":
		return t.upsertRecord(ctx, args, recordType, recordName)
	case "delete":
		return t.deleteRecord(ctx, args, recordType, recordName)
	default:
		return "", fmt.Errorf("unknown action: %s", args.Action)
	}
}

func (t *BindDomainTool) statusRecord(ctx context.Context, args bindDomainArgs, recordType, recordName string) (string, error) {
	records, err := listCloudflareRecords(ctx, args, recordType, recordName)
	if err != nil {
		return "", err
	}
	return toJSON(map[string]any{
		"status":      "success",
		"action":      "bind_domain",
		"phase":       "status",
		"provider":    "cloudflare",
		"record_name": recordName,
		"record_type": recordType,
		"count":       len(records),
		"records":     records,
	}), nil
}

func (t *BindDomainTool) upsertRecord(ctx context.Context, args bindDomainArgs, recordType, recordName string) (string, error) {
	recordValue := strings.TrimSpace(args.RecordValue)
	if recordValue == "" {
		return "", fmt.Errorf("record_value is required for upsert")
	}

	ttl := parseIntWithDefault(args.TTL, 120)
	if ttl < 1 {
		ttl = 120
	}
	proxied := strings.EqualFold(strings.TrimSpace(args.Proxied), "true")

	records, err := listCloudflareRecords(ctx, args, recordType, recordName)
	if err != nil {
		return "", err
	}

	payload := map[string]any{
		"type":    recordType,
		"name":    recordName,
		"content": recordValue,
		"ttl":     ttl,
		"proxied": proxied,
	}

	phase := "create"
	recordID := ""
	if len(records) > 0 {
		phase = "update"
		recordID = records[0].ID
	}

	endpoint := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/dns_records", strings.TrimSpace(args.ZoneID))
	method := http.MethodPost
	if phase == "update" {
		method = http.MethodPut
		endpoint = endpoint + "/" + recordID
	}

	resp, err := doCloudflareRequest(ctx, method, endpoint, strings.TrimSpace(args.APIToken), payload)
	if err != nil {
		return "", err
	}

	var parsed cloudflareResponse
	if err := json.Unmarshal(resp, &parsed); err != nil {
		return "", fmt.Errorf("invalid cloudflare response: %w", err)
	}
	if !parsed.Success {
		return "", fmt.Errorf("cloudflare API failed: %s", string(resp))
	}

	var saved cloudflareDNSRecord
	_ = json.Unmarshal(parsed.Result, &saved)

	return toJSON(map[string]any{
		"status":      "success",
		"action":      "bind_domain",
		"phase":       phase,
		"provider":    "cloudflare",
		"record_id":   saved.ID,
		"record_name": saved.Name,
		"record_type": saved.Type,
		"record_value": saved.Content,
		"ttl":         saved.TTL,
		"proxied":     saved.Proxied,
		"message":     fmt.Sprintf("dns record %s success", phase),
	}), nil
}

func (t *BindDomainTool) deleteRecord(ctx context.Context, args bindDomainArgs, recordType, recordName string) (string, error) {
	records, err := listCloudflareRecords(ctx, args, recordType, recordName)
	if err != nil {
		return "", err
	}
	if len(records) == 0 {
		return toJSON(map[string]any{
			"status":      "success",
			"action":      "bind_domain",
			"phase":       "delete",
			"provider":    "cloudflare",
			"record_name": recordName,
			"record_type": recordType,
			"deleted":     false,
			"message":     "record not found, skip delete",
		}), nil
	}

	endpoint := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/dns_records/%s", strings.TrimSpace(args.ZoneID), records[0].ID)
	resp, err := doCloudflareRequest(ctx, http.MethodDelete, endpoint, strings.TrimSpace(args.APIToken), nil)
	if err != nil {
		return "", err
	}

	var parsed cloudflareResponse
	if err := json.Unmarshal(resp, &parsed); err != nil {
		return "", fmt.Errorf("invalid cloudflare response: %w", err)
	}
	if !parsed.Success {
		return "", fmt.Errorf("cloudflare API failed: %s", string(resp))
	}

	return toJSON(map[string]any{
		"status":      "success",
		"action":      "bind_domain",
		"phase":       "delete",
		"provider":    "cloudflare",
		"record_name": recordName,
		"record_type": recordType,
		"record_id":   records[0].ID,
		"deleted":     true,
		"message":     "dns record deleted",
	}), nil
}

func listCloudflareRecords(ctx context.Context, args bindDomainArgs, recordType, recordName string) ([]cloudflareDNSRecord, error) {
	zoneID := strings.TrimSpace(args.ZoneID)
	apiToken := strings.TrimSpace(args.APIToken)
	if zoneID == "" || apiToken == "" {
		return nil, fmt.Errorf("zone_id and api_token are required")
	}

	endpoint := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/dns_records", zoneID)
	q := url.Values{}
	q.Set("name", recordName)
	if recordType != "" {
		q.Set("type", recordType)
	}
	endpoint = endpoint + "?" + q.Encode()

	resp, err := doCloudflareRequest(ctx, http.MethodGet, endpoint, apiToken, nil)
	if err != nil {
		return nil, err
	}

	var parsed cloudflareResponse
	if err := json.Unmarshal(resp, &parsed); err != nil {
		return nil, fmt.Errorf("invalid cloudflare response: %w", err)
	}
	if !parsed.Success {
		return nil, fmt.Errorf("cloudflare API failed: %s", string(resp))
	}

	var records []cloudflareDNSRecord
	if len(parsed.Result) > 0 {
		if err := json.Unmarshal(parsed.Result, &records); err != nil {
			return nil, fmt.Errorf("failed to parse dns record list: %w", err)
		}
	}
	return records, nil
}

func doCloudflareRequest(ctx context.Context, method, endpoint, apiToken string, payload any) ([]byte, error) {
	var body io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("failed to encode request body: %w", err)
		}
		body = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 25 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cloudflare request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 256*1024))
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("cloudflare HTTP %d: %s", resp.StatusCode, string(respBody))
	}
	return respBody, nil
}
