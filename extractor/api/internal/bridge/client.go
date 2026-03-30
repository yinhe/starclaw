package bridge

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client communicates with the Python miniQMT bridge via HTTP REST.
type Client struct {
	BaseURL    string
	HTTPClient *http.Client
}

func NewClient(baseURL string) *Client {
	return &Client{
		BaseURL: baseURL,
		HTTPClient: &http.Client{
			Timeout: 300 * time.Second,
		},
	}
}

// --- Order commands ---

type SubmitOrderReq struct {
	Account   string  `json:"account"`
	Code      string  `json:"code"`
	Direction string  `json:"direction"` // buy, sell
	Price     float64 `json:"price"`
	Volume    int     `json:"volume"`
	OrderType string  `json:"order_type"` // limit, market
}

type SubmitOrderResp struct {
	QMTOrderID string `json:"qmt_order_id"`
	Status     string `json:"status"`
}

func (c *Client) SubmitOrder(req SubmitOrderReq) (*SubmitOrderResp, error) {
	var resp SubmitOrderResp
	if err := c.post("/order/submit", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) CancelOrder(account, qmtOrderID string) error {
	body := map[string]string{"account": account, "order_id": qmtOrderID}
	return c.post("/order/cancel", body, nil)
}

// --- Account queries ---

type AccountInfo struct {
	Account     string  `json:"account"`
	TotalAssets float64 `json:"total_assets"`
	Available   float64 `json:"available"`
	Frozen      float64 `json:"frozen"`
	MarketValue float64 `json:"market_value"`
}

func (c *Client) GetAccountInfo(account string) (*AccountInfo, error) {
	var resp AccountInfo
	if err := c.get(fmt.Sprintf("/account/info?account=%s", account), &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

type PositionItem struct {
	Code        string  `json:"code"`
	Name        string  `json:"name"`
	Volume      int     `json:"volume"`
	AvailVolume int     `json:"avail_volume"`
	CostPrice   float64 `json:"cost_price"`
	MarketPrice float64 `json:"market_price"`
	PnLFloat    float64 `json:"pnl_float"`
}

func (c *Client) GetPositions(account string) ([]PositionItem, error) {
	var resp []PositionItem
	if err := c.get(fmt.Sprintf("/account/positions?account=%s", account), &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// --- Market data ---

type QuoteItem struct {
	Code      string  `json:"code"`
	Name      string  `json:"name"`
	Price     float64 `json:"price"`
	Open      float64 `json:"open"`
	High      float64 `json:"high"`
	Low       float64 `json:"low"`
	PreClose  float64 `json:"pre_close"`
	Volume    int64   `json:"volume"`
	Amount    float64 `json:"amount"`
	Timestamp string  `json:"timestamp"`
}

func (c *Client) GetQuote(codes string) ([]QuoteItem, error) {
	var resp []QuoteItem
	if err := c.get(fmt.Sprintf("/market/quote?codes=%s", codes), &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

type KlineItem struct {
	Date   string  `json:"date"`
	Open   float64 `json:"open"`
	High   float64 `json:"high"`
	Low    float64 `json:"low"`
	Close  float64 `json:"close"`
	Volume int64   `json:"volume"`
	Amount float64 `json:"amount"`
}

func (c *Client) GetKline(code, period string, count int) ([]KlineItem, error) {
	var resp []KlineItem
	url := fmt.Sprintf("/market/kline?code=%s&period=%s&count=%d", code, period, count)
	if err := c.get(url, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// --- Strategy commands ---

func (c *Client) StartStrategy(strategyID, account string, params map[string]interface{}) error {
	body := map[string]interface{}{
		"strategy_id": strategyID,
		"account":     account,
		"params":      params,
	}
	return c.post("/strategy/start", body, nil)
}

func (c *Client) StopStrategy(strategyID string) error {
	return c.post("/strategy/stop", map[string]string{"strategy_id": strategyID}, nil)
}

// --- Scan (trigger Python strategy executor) ---

func (c *Client) TriggerScan(minScore float64, topN int) (map[string]interface{}, error) {
	body := map[string]interface{}{}
	if minScore > 0 {
		body["min_score"] = minScore
	}
	if topN > 0 {
		body["top_n"] = topN
	}
	var resp map[string]interface{}
	if err := c.post("/scan", body, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *Client) GetScanStatus() (map[string]interface{}, error) {
	var resp map[string]interface{}
	if err := c.get("/scan/status", &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// --- Health ---

func (c *Client) Ping() error {
	return c.get("/health", nil)
}

// --- HTTP helpers ---

func (c *Client) get(path string, result interface{}) error {
	resp, err := c.HTTPClient.Get(c.BaseURL + path)
	if err != nil {
		return fmt.Errorf("bridge GET %s: %w", path, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("bridge GET %s: status %d: %s", path, resp.StatusCode, string(body))
	}
	if result != nil {
		return json.NewDecoder(resp.Body).Decode(result)
	}
	return nil
}

func (c *Client) post(path string, body interface{}, result interface{}) error {
	data, err := json.Marshal(body)
	if err != nil {
		return err
	}
	resp, err := c.HTTPClient.Post(c.BaseURL+path, "application/json", bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("bridge POST %s: %w", path, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("bridge POST %s: status %d: %s", path, resp.StatusCode, string(respBody))
	}
	if result != nil {
		return json.NewDecoder(resp.Body).Decode(result)
	}
	return nil
}
