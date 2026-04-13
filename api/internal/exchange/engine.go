package exchange

import (
	"fmt"
	"log"
	"sort"
	"sync"
	"time"

	"github.com/yinhe/starclaw/internal/nerve"
	"gorm.io/gorm"
)

// ════════════════════════════════════════════════════════════
// StarEnergy Exchange — 星能交易所 (Phase 5B)
//
// 节点间星能P2P交易引擎:
//   - 挂单: 买单/卖单 (限价/市价)
//   - 撮合: 价格优先 · 时间优先
//   - 结算: 即时转账 · 交易审计
//   - 价格发现: 最新成交 · 买一卖一 · K线
//
// Agent Marketplace:
//   - 服务发布: Agent 注册可提供的服务
//   - 竞价投标: 需求方发布任务，Agent 竞价
//   - 服务评价: 完成后评分
//   - 动态定价: 基于供需 + 信誉
// ════════════════════════════════════════════════════════════

// ── Types ──

type OrderSide string
type OrderType string
type OrderStatus string
type ServiceStatus string
type BidStatus string

const (
	SideBuy  OrderSide = "buy"
	SideSell OrderSide = "sell"

	TypeLimit  OrderType = "limit"
	TypeMarket OrderType = "market"

	OrderOpen      OrderStatus = "open"
	OrderFilled    OrderStatus = "filled"
	OrderPartial   OrderStatus = "partial"
	OrderCancelled OrderStatus = "cancelled"

	SvcActive  ServiceStatus = "active"
	SvcPaused  ServiceStatus = "paused"
	SvcRetired ServiceStatus = "retired"

	BidPending  BidStatus = "pending"
	BidAccepted BidStatus = "accepted"
	BidRejected BidStatus = "rejected"
	BidComplete BidStatus = "complete"
)

// ── Exchange Data ──

type Order struct {
	ID        string      `json:"id"`
	NodeID    string      `json:"node_id"`
	Side      OrderSide   `json:"side"`
	Type      OrderType   `json:"type"`
	Price     float64     `json:"price"` // per unit, 0 for market orders
	Quantity  float64     `json:"quantity"`
	Filled    float64     `json:"filled"`
	Status    OrderStatus `json:"status"`
	CreatedAt time.Time   `json:"created_at"`
	FilledAt  *time.Time  `json:"filled_at,omitempty"`
}

type Trade struct {
	ID        string    `json:"id"`
	BuyOrder  string    `json:"buy_order_id"`
	SellOrder string    `json:"sell_order_id"`
	BuyerID   string    `json:"buyer_id"`
	SellerID  string    `json:"seller_id"`
	Price     float64   `json:"price"`
	Quantity  float64   `json:"quantity"`
	Total     float64   `json:"total"` // price * quantity
	Timestamp time.Time `json:"timestamp"`
}

type OrderBook struct {
	Bids []OrderBookEntry `json:"bids"` // buy orders, highest first
	Asks []OrderBookEntry `json:"asks"` // sell orders, lowest first
}

type OrderBookEntry struct {
	Price    float64 `json:"price"`
	Quantity float64 `json:"quantity"`
	Count    int     `json:"count"`
}

// ── Marketplace Data ──

type AgentService struct {
	ID          string        `json:"id"`
	AgentID     string        `json:"agent_id"`
	NodeID      string        `json:"node_id"`
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Category    string        `json:"category"`   // compute, inference, data, creative, analysis
	BasePrice   float64       `json:"base_price"` // StarEnergy per call
	Rating      float64       `json:"rating"`     // 0-5
	TotalCalls  int           `json:"total_calls"`
	Status      ServiceStatus `json:"status"`
	Tags        []string      `json:"tags,omitempty"`
	CreatedAt   time.Time     `json:"created_at"`
}

type ServiceRequest struct {
	ID          string                 `json:"id"`
	RequesterID string                 `json:"requester_id"`
	Title       string                 `json:"title"`
	Description string                 `json:"description"`
	Category    string                 `json:"category"`
	Budget      float64                `json:"budget"` // max StarEnergy willing to pay
	Params      map[string]interface{} `json:"params,omitempty"`
	Status      string                 `json:"status"` // open, assigned, completed, cancelled
	AssignedTo  string                 `json:"assigned_to,omitempty"`
	Bids        []ServiceBid           `json:"bids"`
	CreatedAt   time.Time              `json:"created_at"`
	CompletedAt *time.Time             `json:"completed_at,omitempty"`
}

type ServiceBid struct {
	ID        string    `json:"id"`
	AgentID   string    `json:"agent_id"`
	ServiceID string    `json:"service_id"`
	RequestID string    `json:"request_id"`
	Price     float64   `json:"price"`
	ETA       string    `json:"eta"` // estimated time
	Message   string    `json:"message,omitempty"`
	Status    BidStatus `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

type ServiceRating struct {
	ID        string    `json:"id"`
	ServiceID string    `json:"service_id"`
	RequestID string    `json:"request_id"`
	RaterID   string    `json:"rater_id"`
	Score     float64   `json:"score"` // 1-5
	Comment   string    `json:"comment,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// ── Engine ──

type EngineConfig struct {
	TradingFeeRate float64 `json:"trading_fee_rate"` // e.g. 0.001 = 0.1%
	MinOrderSize   float64 `json:"min_order_size"`
	MaxOrderSize   float64 `json:"max_order_size"`
	EnableTrading  bool    `json:"enable_trading"`
}

func DefaultConfig() *EngineConfig {
	return &EngineConfig{
		TradingFeeRate: 0.001,
		MinOrderSize:   0.1,
		MaxOrderSize:   100000,
		EnableTrading:  true,
	}
}

type Engine struct {
	mu        sync.RWMutex
	db        *gorm.DB
	nodeID    string
	config    *EngineConfig
	orders    map[string]*Order
	trades    []Trade
	services  map[string]*AgentService
	requests  map[string]*ServiceRequest
	ratings   []ServiceRating
	lastPrice float64
	stats     EngineStats
	startAt   time.Time
	nextID    int
}

type EngineStats struct {
	// Exchange
	OrdersPlaced    int     `json:"orders_placed"`
	OrdersFilled    int     `json:"orders_filled"`
	OrdersCancelled int     `json:"orders_cancelled"`
	TradesExecuted  int     `json:"trades_executed"`
	TotalVolume     float64 `json:"total_volume"`
	LastPrice       float64 `json:"last_price"`

	// Marketplace
	ServicesListed     int     `json:"services_listed"`
	RequestsCreated    int     `json:"requests_created"`
	RequestsCompleted  int     `json:"requests_completed"`
	BidsPlaced         int     `json:"bids_placed"`
	TotalServiceVolume float64 `json:"total_service_volume"`

	Uptime string `json:"uptime"`
}

var (
	globalEngine *Engine
	engineOnce   sync.Once
)

func InitEngine(nodeID string, cfg *EngineConfig, db *gorm.DB) *Engine {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	engineOnce.Do(func() {
		globalEngine = &Engine{
			db:       db,
			nodeID:   nodeID,
			config:   cfg,
			orders:   make(map[string]*Order),
			trades:   make([]Trade, 0),
			services: make(map[string]*AgentService),
			requests: make(map[string]*ServiceRequest),
			ratings:  make([]ServiceRating, 0),
			startAt:  time.Now(),
		}
		globalEngine.loadFromDB()
		log.Printf("[exchange] StarEnergy exchange + marketplace initialized (fee=%.2f%%, db=%v)", cfg.TradingFeeRate*100, db != nil)
	})
	return globalEngine
}

func GetEngine() *Engine {
	return globalEngine
}

func (e *Engine) genID(prefix string) string {
	e.nextID++
	return fmt.Sprintf("%s-%d-%d", prefix, time.Now().UnixMilli(), e.nextID)
}

// ══════════════ Exchange ══════════════

func (e *Engine) PlaceOrder(nodeID string, side OrderSide, orderType OrderType, price, quantity float64) (*Order, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.config.EnableTrading {
		return nil, fmt.Errorf("trading is disabled")
	}
	if quantity < e.config.MinOrderSize || quantity > e.config.MaxOrderSize {
		return nil, fmt.Errorf("quantity %.2f outside allowed range [%.2f, %.2f]", quantity, e.config.MinOrderSize, e.config.MaxOrderSize)
	}
	if orderType == TypeLimit && price <= 0 {
		return nil, fmt.Errorf("limit order requires positive price")
	}

	order := &Order{
		ID:        e.genID("ord"),
		NodeID:    nodeID,
		Side:      side,
		Type:      orderType,
		Price:     price,
		Quantity:  quantity,
		Filled:    0,
		Status:    OrderOpen,
		CreatedAt: time.Now(),
	}
	e.orders[order.ID] = order
	e.stats.OrdersPlaced++
	go e.persistOrder(order)

	// Try to match
	e.matchOrder(order)

	if bus := nerve.GetBus(); bus != nil {
		bus.Publish("exchange.order.placed", "exchange", map[string]interface{}{
			"order_id": order.ID,
			"side":     string(side),
			"price":    price,
			"quantity": quantity,
			"status":   string(order.Status),
		})
	}

	return order, nil
}

func (e *Engine) CancelOrder(orderID string) error {
	order, ok := e.orders[orderID]
	if !ok {
		return fmt.Errorf("order %s not found", orderID)
	}
	if order.Status != OrderOpen && order.Status != OrderPartial {
		return fmt.Errorf("order %s cannot be cancelled (status=%s)", orderID, order.Status)
	}
	order.Status = OrderCancelled
	e.stats.OrdersCancelled++
	go e.updateOrderDB(order)
	return nil
}

func (e *Engine) matchOrder(incoming *Order) {
	// Collect opposing orders
	var opposing []*Order
	for _, o := range e.orders {
		if o.ID == incoming.ID {
			continue
		}
		if o.Status != OrderOpen && o.Status != OrderPartial {
			continue
		}
		if incoming.Side == SideBuy && o.Side == SideSell {
			if incoming.Type == TypeMarket || o.Type == TypeMarket || incoming.Price >= o.Price {
				opposing = append(opposing, o)
			}
		} else if incoming.Side == SideSell && o.Side == SideBuy {
			if incoming.Type == TypeMarket || o.Type == TypeMarket || incoming.Price <= o.Price {
				opposing = append(opposing, o)
			}
		}
	}

	// Sort: best price first, then time
	if incoming.Side == SideBuy {
		sort.Slice(opposing, func(i, j int) bool {
			if opposing[i].Price != opposing[j].Price {
				return opposing[i].Price < opposing[j].Price // lowest sell first
			}
			return opposing[i].CreatedAt.Before(opposing[j].CreatedAt)
		})
	} else {
		sort.Slice(opposing, func(i, j int) bool {
			if opposing[i].Price != opposing[j].Price {
				return opposing[i].Price > opposing[j].Price // highest buy first
			}
			return opposing[i].CreatedAt.Before(opposing[j].CreatedAt)
		})
	}

	remaining := incoming.Quantity - incoming.Filled
	for _, match := range opposing {
		if remaining <= 0 {
			break
		}
		available := match.Quantity - match.Filled
		fillQty := remaining
		if available < fillQty {
			fillQty = available
		}

		tradePrice := match.Price
		if match.Type == TypeMarket {
			tradePrice = incoming.Price
		}
		if tradePrice <= 0 {
			tradePrice = e.lastPrice
		}

		// Execute trade
		trade := Trade{
			ID:        e.genID("trd"),
			Price:     tradePrice,
			Quantity:  fillQty,
			Total:     tradePrice * fillQty,
			Timestamp: time.Now(),
		}
		if incoming.Side == SideBuy {
			trade.BuyOrder = incoming.ID
			trade.SellOrder = match.ID
			trade.BuyerID = incoming.NodeID
			trade.SellerID = match.NodeID
		} else {
			trade.SellOrder = incoming.ID
			trade.BuyOrder = match.ID
			trade.SellerID = incoming.NodeID
			trade.BuyerID = match.NodeID
		}
		e.trades = append(e.trades, trade)
		e.stats.TradesExecuted++
		e.stats.TotalVolume += trade.Total
		e.lastPrice = tradePrice
		e.stats.LastPrice = tradePrice
		go e.persistTrade(&trade)

		// Update fills
		incoming.Filled += fillQty
		match.Filled += fillQty
		remaining -= fillQty

		now := time.Now()
		if match.Filled >= match.Quantity {
			match.Status = OrderFilled
			match.FilledAt = &now
			e.stats.OrdersFilled++
		} else {
			match.Status = OrderPartial
		}
		go e.updateOrderDB(match)
	}

	now := time.Now()
	if incoming.Filled >= incoming.Quantity {
		incoming.Status = OrderFilled
		incoming.FilledAt = &now
		e.stats.OrdersFilled++
	} else if incoming.Filled > 0 {
		incoming.Status = OrderPartial
	}
}

func (e *Engine) GetOrderBook(depth int) *OrderBook {
	e.mu.RLock()
	defer e.mu.RUnlock()

	bidMap := make(map[float64]float64)
	askMap := make(map[float64]float64)
	bidCount := make(map[float64]int)
	askCount := make(map[float64]int)

	for _, o := range e.orders {
		if o.Status != OrderOpen && o.Status != OrderPartial {
			continue
		}
		rem := o.Quantity - o.Filled
		if o.Side == SideBuy && o.Price > 0 {
			bidMap[o.Price] += rem
			bidCount[o.Price]++
		} else if o.Side == SideSell && o.Price > 0 {
			askMap[o.Price] += rem
			askCount[o.Price]++
		}
	}

	book := &OrderBook{}
	for p, q := range bidMap {
		book.Bids = append(book.Bids, OrderBookEntry{Price: p, Quantity: q, Count: bidCount[p]})
	}
	for p, q := range askMap {
		book.Asks = append(book.Asks, OrderBookEntry{Price: p, Quantity: q, Count: askCount[p]})
	}

	sort.Slice(book.Bids, func(i, j int) bool { return book.Bids[i].Price > book.Bids[j].Price })
	sort.Slice(book.Asks, func(i, j int) bool { return book.Asks[i].Price < book.Asks[j].Price })

	if depth > 0 && len(book.Bids) > depth {
		book.Bids = book.Bids[:depth]
	}
	if depth > 0 && len(book.Asks) > depth {
		book.Asks = book.Asks[:depth]
	}

	return book
}

func (e *Engine) RecentTrades(limit int) []Trade {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if limit <= 0 || limit > len(e.trades) {
		limit = len(e.trades)
	}
	if limit == 0 {
		return nil
	}
	start := len(e.trades) - limit
	result := make([]Trade, limit)
	copy(result, e.trades[start:])
	return result
}

// ══════════════ Marketplace ══════════════

func (e *Engine) ListService(agentID, nodeID, name, description, category string, basePrice float64, tags []string) *AgentService {
	e.mu.Lock()
	defer e.mu.Unlock()

	svc := &AgentService{
		ID:          e.genID("svc"),
		AgentID:     agentID,
		NodeID:      nodeID,
		Name:        name,
		Description: description,
		Category:    category,
		BasePrice:   basePrice,
		Status:      SvcActive,
		Tags:        tags,
		CreatedAt:   time.Now(),
	}
	e.services[svc.ID] = svc
	e.stats.ServicesListed++
	go e.persistService(svc)

	if bus := nerve.GetBus(); bus != nil {
		bus.Publish("exchange.service.listed", "exchange", map[string]interface{}{
			"service_id": svc.ID,
			"name":       name,
			"category":   category,
			"price":      basePrice,
		})
	}

	return svc
}

func (e *Engine) GetServices(category string, limit int) []*AgentService {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var result []*AgentService
	for _, s := range e.services {
		if s.Status != SvcActive {
			continue
		}
		if category != "" && s.Category != category {
			continue
		}
		result = append(result, s)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Rating > result[j].Rating })
	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}
	return result
}

func (e *Engine) CreateRequest(requesterID, title, description, category string, budget float64, params map[string]interface{}) *ServiceRequest {
	e.mu.Lock()
	defer e.mu.Unlock()

	req := &ServiceRequest{
		ID:          e.genID("req"),
		RequesterID: requesterID,
		Title:       title,
		Description: description,
		Category:    category,
		Budget:      budget,
		Params:      params,
		Status:      "open",
		Bids:        make([]ServiceBid, 0),
		CreatedAt:   time.Now(),
	}
	e.requests[req.ID] = req
	e.stats.RequestsCreated++
	go e.persistRequest(req)

	if bus := nerve.GetBus(); bus != nil {
		bus.Publish("exchange.request.created", "exchange", map[string]interface{}{
			"request_id": req.ID,
			"title":      title,
			"budget":     budget,
			"category":   category,
		})
	}

	return req
}

func (e *Engine) PlaceBid(requestID, agentID, serviceID string, price float64, eta, message string) (*ServiceBid, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	req, ok := e.requests[requestID]
	if !ok {
		return nil, fmt.Errorf("request %s not found", requestID)
	}
	if req.Status != "open" {
		return nil, fmt.Errorf("request %s is not open (status=%s)", requestID, req.Status)
	}
	if price > req.Budget {
		return nil, fmt.Errorf("bid price %.2f exceeds budget %.2f", price, req.Budget)
	}

	bid := ServiceBid{
		ID:        e.genID("bid"),
		AgentID:   agentID,
		ServiceID: serviceID,
		RequestID: requestID,
		Price:     price,
		ETA:       eta,
		Message:   message,
		Status:    BidPending,
		CreatedAt: time.Now(),
	}
	req.Bids = append(req.Bids, bid)
	e.stats.BidsPlaced++
	go e.persistBid(&bid)
	return &bid, nil
}

func (e *Engine) AcceptBid(requestID, bidID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	req, ok := e.requests[requestID]
	if !ok {
		return fmt.Errorf("request %s not found", requestID)
	}
	for i := range req.Bids {
		if req.Bids[i].ID == bidID {
			req.Bids[i].Status = BidAccepted
			req.Status = "assigned"
			req.AssignedTo = req.Bids[i].AgentID

			// Reject other bids
			for j := range req.Bids {
				if j != i && req.Bids[j].Status == BidPending {
					req.Bids[j].Status = BidRejected
					bid := req.Bids[j]
					go e.persistBid(&bid)
				}
			}
			accepted := req.Bids[i]
			go e.persistBid(&accepted)
			go e.persistRequest(req)
			return nil
		}
	}
	return fmt.Errorf("bid %s not found in request %s", bidID, requestID)
}

func (e *Engine) CompleteRequest(requestID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	req, ok := e.requests[requestID]
	if !ok {
		return fmt.Errorf("request %s not found", requestID)
	}
	now := time.Now()
	req.Status = "completed"
	req.CompletedAt = &now

	// Mark accepted bid as complete
	for i := range req.Bids {
		if req.Bids[i].Status == BidAccepted {
			req.Bids[i].Status = BidComplete
			e.stats.TotalServiceVolume += req.Bids[i].Price
			break
		}
	}
	e.stats.RequestsCompleted++
	go e.persistRequest(req)
	return nil
}

func (e *Engine) RateService(serviceID, requestID, raterID string, score float64, comment string) (*ServiceRating, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	svc, ok := e.services[serviceID]
	if !ok {
		return nil, fmt.Errorf("service %s not found", serviceID)
	}
	if score < 1 || score > 5 {
		return nil, fmt.Errorf("score must be 1-5")
	}

	rating := ServiceRating{
		ID:        e.genID("rat"),
		ServiceID: serviceID,
		RequestID: requestID,
		RaterID:   raterID,
		Score:     score,
		Comment:   comment,
		CreatedAt: time.Now(),
	}
	e.ratings = append(e.ratings, rating)
	go e.persistRating(&rating)

	// Update service rating (running average)
	svc.TotalCalls++
	svc.Rating = (svc.Rating*float64(svc.TotalCalls-1) + score) / float64(svc.TotalCalls)
	go e.persistService(svc)

	return &rating, nil
}

func (e *Engine) ListRequests(status string, limit int) []*ServiceRequest {
	e.mu.RLock()
	defer e.mu.RUnlock()
	var result []*ServiceRequest
	for _, r := range e.requests {
		if status != "" && r.Status != status {
			continue
		}
		result = append(result, r)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].CreatedAt.After(result[j].CreatedAt) })
	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}
	return result
}

// ── Stats ──

func (e *Engine) Stats() *EngineStats {
	e.mu.RLock()
	defer e.mu.RUnlock()
	s := e.stats
	s.Uptime = time.Since(e.startAt).Round(time.Second).String()
	s.LastPrice = e.lastPrice
	return &s
}
