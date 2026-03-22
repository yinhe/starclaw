package service

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

// DNSService manages Aliyun DNS records for Hive subdomains.
type DNSService struct {
	accessKeyID     string
	accessKeySecret string
	domain          string // e.g. starclaw.me
	client          *http.Client
}

type DNSConfig struct {
	AccessKeyID     string
	AccessKeySecret string
	Domain          string // e.g. starclaw.me
}

func NewDNSService(cfg DNSConfig) *DNSService {
	return &DNSService{
		accessKeyID:     cfg.AccessKeyID,
		accessKeySecret: cfg.AccessKeySecret,
		domain:          cfg.Domain,
		client:          &http.Client{Timeout: 15 * time.Second},
	}
}

// AddRecord creates an A record for a subdomain pointing to the given IP.
// For hive mode: slug → shared hive server IP.
// For ECS mode: slug → dedicated ECS public IP.
func (s *DNSService) AddRecord(subdomain, ip string) (string, error) {
	params := map[string]string{
		"Action":     "AddDomainRecord",
		"DomainName": s.domain,
		"RR":         subdomain,
		"Type":       "A",
		"Value":      ip,
		"TTL":        "600",
	}

	body, err := s.callAPI(params)
	if err != nil {
		return "", fmt.Errorf("add DNS record: %w", err)
	}

	var resp struct {
		RecordId  string `json:"RecordId"`
		RequestId string `json:"RequestId"`
	}
	json.Unmarshal(body, &resp)
	return resp.RecordId, nil
}

// UpdateRecord updates an existing DNS record's value (e.g. IP change).
func (s *DNSService) UpdateRecord(recordID, subdomain, ip string) error {
	params := map[string]string{
		"Action":   "UpdateDomainRecord",
		"RecordId": recordID,
		"RR":       subdomain,
		"Type":     "A",
		"Value":    ip,
		"TTL":      "600",
	}
	_, err := s.callAPI(params)
	return err
}

// DeleteRecord removes a DNS record.
func (s *DNSService) DeleteRecord(recordID string) error {
	params := map[string]string{
		"Action":   "DeleteDomainRecord",
		"RecordId": recordID,
	}
	_, err := s.callAPI(params)
	return err
}

// GetRecord looks up the current DNS record for a subdomain.
func (s *DNSService) GetRecord(subdomain string) (recordID, ip string, err error) {
	params := map[string]string{
		"Action":      "DescribeDomainRecords",
		"DomainName":  s.domain,
		"RRKeyWord":   subdomain,
		"TypeKeyWord": "A",
		"PageSize":    "1",
	}

	body, err := s.callAPI(params)
	if err != nil {
		return "", "", err
	}

	var resp struct {
		DomainRecords struct {
			Record []struct {
				RecordId string `json:"RecordId"`
				RR       string `json:"RR"`
				Value    string `json:"Value"`
			} `json:"Record"`
		} `json:"DomainRecords"`
	}
	json.Unmarshal(body, &resp)

	for _, r := range resp.DomainRecords.Record {
		if r.RR == subdomain {
			return r.RecordId, r.Value, nil
		}
	}
	return "", "", fmt.Errorf("record not found: %s.%s", subdomain, s.domain)
}

// ──── Aliyun DNS API v1 Signature ────

func (s *DNSService) callAPI(params map[string]string) ([]byte, error) {
	params["Format"] = "JSON"
	params["Version"] = "2015-01-09"
	params["AccessKeyId"] = s.accessKeyID
	params["SignatureMethod"] = "HMAC-SHA256"
	params["SignatureVersion"] = "1.0"
	params["SignatureNonce"] = uuid.New().String()
	params["Timestamp"] = time.Now().UTC().Format("2006-01-02T15:04:05Z")

	params["Signature"] = s.sign(params)

	values := url.Values{}
	for k, v := range params {
		values.Set(k, v)
	}

	resp, err := s.client.Get("https://alidns.aliyuncs.com/?" + values.Encode())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		var errResp struct {
			Code    string `json:"Code"`
			Message string `json:"Message"`
		}
		json.Unmarshal(body, &errResp)
		return nil, fmt.Errorf("aliyun DNS %s: %s", errResp.Code, errResp.Message)
	}

	return body, nil
}

func (s *DNSService) sign(params map[string]string) string {
	keys := make([]string, 0, len(params))
	for k := range params {
		if k == "Signature" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var pairs []string
	for _, k := range keys {
		pairs = append(pairs, url.QueryEscape(k)+"="+url.QueryEscape(params[k]))
	}
	canonicalized := strings.Join(pairs, "&")

	stringToSign := "GET&" + url.QueryEscape("/") + "&" + url.QueryEscape(canonicalized)

	mac := hmac.New(sha256.New, []byte(s.accessKeySecret+"&"))
	mac.Write([]byte(stringToSign))
	return hex.EncodeToString(mac.Sum(nil))
}
