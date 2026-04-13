package exchange

import (
	"encoding/json"
	"log"

	"github.com/yinhe/starclaw/internal/model"
	"gorm.io/gorm"
)

func (e *Engine) persistOrder(o *Order) {
	if e.db == nil {
		return
	}
	rec := model.ExchangeOrder{
		OrderID:  o.ID,
		NodeID:   o.NodeID,
		Side:     string(o.Side),
		Type:     string(o.Type),
		Price:    o.Price,
		Quantity: o.Quantity,
		Filled:   o.Filled,
		Status:   string(o.Status),
		FilledAt: o.FilledAt,
	}
	if err := e.db.Save(&rec).Error; err != nil {
		log.Printf("[exchange] persist order %s failed: %v", o.ID, err)
	}
}

func (e *Engine) updateOrderDB(o *Order) {
	if e.db == nil {
		return
	}
	e.db.Model(&model.ExchangeOrder{}).Where("order_id = ?", o.ID).Updates(map[string]interface{}{
		"filled":    o.Filled,
		"status":    string(o.Status),
		"filled_at": o.FilledAt,
	})
}

func (e *Engine) persistTrade(t *Trade) {
	if e.db == nil {
		return
	}
	rec := model.ExchangeTrade{
		TradeID:   t.ID,
		BuyOrder:  t.BuyOrder,
		SellOrder: t.SellOrder,
		BuyerID:   t.BuyerID,
		SellerID:  t.SellerID,
		Price:     t.Price,
		Quantity:  t.Quantity,
		Total:     t.Total,
	}
	if err := e.db.Create(&rec).Error; err != nil {
		log.Printf("[exchange] persist trade %s failed: %v", t.ID, err)
	}
}

func (e *Engine) persistService(s *AgentService) {
	if e.db == nil {
		return
	}
	tags, _ := json.Marshal(s.Tags)
	rec := model.ExchangeService{
		ServiceID:   s.ID,
		AgentID:     s.AgentID,
		NodeID:      s.NodeID,
		Name:        s.Name,
		Description: s.Description,
		Category:    s.Category,
		BasePrice:   s.BasePrice,
		Rating:      s.Rating,
		TotalCalls:  s.TotalCalls,
		Status:      string(s.Status),
		Tags:        string(tags),
	}
	if err := e.db.Save(&rec).Error; err != nil {
		log.Printf("[exchange] persist service %s failed: %v", s.ID, err)
	}
}

func (e *Engine) persistRequest(r *ServiceRequest) {
	if e.db == nil {
		return
	}
	params, _ := json.Marshal(r.Params)
	rec := model.ExchangeRequest{
		RequestID:   r.ID,
		RequesterID: r.RequesterID,
		Title:       r.Title,
		Description: r.Description,
		Category:    r.Category,
		Budget:      r.Budget,
		Params:      string(params),
		Status:      r.Status,
		AssignedTo:  r.AssignedTo,
		CompletedAt: r.CompletedAt,
	}
	if err := e.db.Save(&rec).Error; err != nil {
		log.Printf("[exchange] persist request %s failed: %v", r.ID, err)
	}
}

func (e *Engine) persistBid(b *ServiceBid) {
	if e.db == nil {
		return
	}
	rec := model.ExchangeBid{
		BidID:     b.ID,
		AgentID:   b.AgentID,
		ServiceID: b.ServiceID,
		RequestID: b.RequestID,
		Price:     b.Price,
		ETA:       b.ETA,
		Message:   b.Message,
		Status:    string(b.Status),
	}
	if err := e.db.Save(&rec).Error; err != nil {
		log.Printf("[exchange] persist bid %s failed: %v", b.ID, err)
	}
}

func (e *Engine) persistRating(r *ServiceRating) {
	if e.db == nil {
		return
	}
	rec := model.ExchangeRating{
		RatingID:  r.ID,
		ServiceID: r.ServiceID,
		RequestID: r.RequestID,
		RaterID:   r.RaterID,
		Score:     r.Score,
		Comment:   r.Comment,
	}
	if err := e.db.Create(&rec).Error; err != nil {
		log.Printf("[exchange] persist rating %s failed: %v", r.ID, err)
	}
}

func (e *Engine) loadFromDB() {
	if e.db == nil {
		return
	}

	// Load orders
	var orderRecs []model.ExchangeOrder
	e.db.Where("node_id = ? AND status IN ?", e.nodeID, []string{"open", "partial"}).Find(&orderRecs)
	for _, r := range orderRecs {
		e.orders[r.OrderID] = &Order{
			ID:        r.OrderID,
			NodeID:    r.NodeID,
			Side:      OrderSide(r.Side),
			Type:      OrderType(r.Type),
			Price:     r.Price,
			Quantity:  r.Quantity,
			Filled:    r.Filled,
			Status:    OrderStatus(r.Status),
			CreatedAt: r.CreatedAt,
			FilledAt:  r.FilledAt,
		}
	}

	// Load trades (latest 500)
	var tradeRecs []model.ExchangeTrade
	e.db.Order("created_at desc").Limit(500).Find(&tradeRecs)
	for i := len(tradeRecs) - 1; i >= 0; i-- {
		r := tradeRecs[i]
		e.trades = append(e.trades, Trade{
			ID:        r.TradeID,
			BuyOrder:  r.BuyOrder,
			SellOrder: r.SellOrder,
			BuyerID:   r.BuyerID,
			SellerID:  r.SellerID,
			Price:     r.Price,
			Quantity:  r.Quantity,
			Total:     r.Total,
			Timestamp: r.CreatedAt,
		})
	}
	if len(e.trades) > 0 {
		e.lastPrice = e.trades[len(e.trades)-1].Price
	}

	// Load services
	var svcRecs []model.ExchangeService
	e.db.Where("status = ?", "active").Find(&svcRecs)
	for _, r := range svcRecs {
		var tags []string
		json.Unmarshal([]byte(r.Tags), &tags)
		e.services[r.ServiceID] = &AgentService{
			ID:          r.ServiceID,
			AgentID:     r.AgentID,
			NodeID:      r.NodeID,
			Name:        r.Name,
			Description: r.Description,
			Category:    r.Category,
			BasePrice:   r.BasePrice,
			Rating:      r.Rating,
			TotalCalls:  r.TotalCalls,
			Status:      ServiceStatus(r.Status),
			Tags:        tags,
			CreatedAt:   r.CreatedAt,
		}
	}

	// Load open requests
	var reqRecs []model.ExchangeRequest
	e.db.Where("status IN ?", []string{"open", "assigned"}).Find(&reqRecs)
	for _, r := range reqRecs {
		var params map[string]interface{}
		json.Unmarshal([]byte(r.Params), &params)
		req := &ServiceRequest{
			ID:          r.RequestID,
			RequesterID: r.RequesterID,
			Title:       r.Title,
			Description: r.Description,
			Category:    r.Category,
			Budget:      r.Budget,
			Params:      params,
			Status:      r.Status,
			AssignedTo:  r.AssignedTo,
			Bids:        make([]ServiceBid, 0),
			CreatedAt:   r.CreatedAt,
			CompletedAt: r.CompletedAt,
		}
		// Load bids for this request
		var bidRecs []model.ExchangeBid
		e.db.Where("request_id = ?", r.RequestID).Find(&bidRecs)
		for _, b := range bidRecs {
			req.Bids = append(req.Bids, ServiceBid{
				ID:        b.BidID,
				AgentID:   b.AgentID,
				ServiceID: b.ServiceID,
				RequestID: b.RequestID,
				Price:     b.Price,
				ETA:       b.ETA,
				Message:   b.Message,
				Status:    BidStatus(b.Status),
				CreatedAt: b.CreatedAt,
			})
		}
		e.requests[r.RequestID] = req
	}

	// Recompute stats
	var totalOrders int64
	e.db.Model(&model.ExchangeOrder{}).Count(&totalOrders)
	e.stats.OrdersPlaced = int(totalOrders)
	var filledOrders int64
	e.db.Model(&model.ExchangeOrder{}).Where("status = ?", "filled").Count(&filledOrders)
	e.stats.OrdersFilled = int(filledOrders)
	e.stats.TradesExecuted = len(e.trades)
	e.stats.ServicesListed = len(e.services)
	e.stats.LastPrice = e.lastPrice

	total := len(orderRecs) + len(tradeRecs) + len(svcRecs) + len(reqRecs)
	if total > 0 {
		log.Printf("[exchange] restored from DB: %d orders, %d trades, %d services, %d requests",
			len(orderRecs), len(tradeRecs), len(svcRecs), len(reqRecs))
	}
}

func (e *Engine) SetDB(db *gorm.DB) {
	e.db = db
}
