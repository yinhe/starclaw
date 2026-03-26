package handler

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"

	pheromone "starclaw.net/pheromone/sdk"
	"starclaw.net/queen/api/internal/database"
	"starclaw.net/queen/api/internal/model"
)

// ph holds the Pheromone client singleton for use by handlers.
var (
	ph   *pheromone.Client
	phMu sync.RWMutex
)

// SetPheromone stores the Pheromone client for handler use.
func SetPheromone(client *pheromone.Client) {
	phMu.Lock()
	ph = client
	phMu.Unlock()
}

// RegisterPheromoneRPC registers Queen's RPC handlers on the Pheromone ESB.
// This exposes credit-check and user-lookup as NATS request/reply endpoints
// so other services (Synapse, Hive) can call them without HTTP.
func RegisterPheromoneRPC(ph *pheromone.Client) {
	// RPC: check-credit — returns user balance and quota status
	if err := ph.HandleRPC(pheromone.RPCCheckCredit, handleCheckCredit); err != nil {
		log.Printf("[queen] pheromone RPC register %s failed: %v", pheromone.RPCCheckCredit, err)
	}

	// RPC: deduct-credit — deducts credits for resource usage
	if err := ph.HandleRPC(pheromone.RPCDeductCredit, handleDeductCredit); err != nil {
		log.Printf("[queen] pheromone RPC register %s failed: %v", pheromone.RPCDeductCredit, err)
	}

	// RPC: get-user — returns basic user info
	if err := ph.HandleRPC(pheromone.RPCGetUser, handleGetUser); err != nil {
		log.Printf("[queen] pheromone RPC register %s failed: %v", pheromone.RPCGetUser, err)
	}

	log.Printf("[queen] pheromone RPC handlers registered: %s, %s, %s",
		pheromone.RPCCheckCredit, pheromone.RPCDeductCredit, pheromone.RPCGetUser)
}

// PublishUserEvent publishes a user event to the Pheromone ESB.
func PublishUserEvent(subject string, evt pheromone.UserEvent) {
	phMu.RLock()
	c := ph
	phMu.RUnlock()
	if c == nil {
		return
	}
	if err := c.Publish(subject, evt); err != nil {
		log.Printf("[queen] pheromone publish %s failed: %v", subject, err)
	}
}

// PublishPaymentEvent publishes a payment event to the Pheromone ESB.
func PublishPaymentEvent(userID string, amount int64, orderNo string) {
	phMu.RLock()
	c := ph
	phMu.RUnlock()
	if c == nil {
		return
	}
	if err := c.Publish(pheromone.SubjectPayment, pheromone.PaymentEvent{
		UserID:  userID,
		Amount:  amount,
		OrderNo: orderNo,
	}); err != nil {
		log.Printf("[queen] pheromone publish payment failed: %v", err)
	}
}

func handleCheckCredit(data []byte) (interface{}, error) {
	var req pheromone.CreditRequest
	if err := json.Unmarshal(data, &req); err != nil {
		return nil, fmt.Errorf("invalid request: %w", err)
	}
	if req.UserID == "" {
		return nil, fmt.Errorf("user_id is required")
	}

	// Look up UserBalance (billing system)
	var bal model.UserBalance
	if err := database.DB.Where("user_id = ?", req.UserID).First(&bal).Error; err != nil {
		// No balance record — return zero
		return pheromone.CreditResponse{
			UserID:  req.UserID,
			Balance: 0,
			OK:      false,
		}, nil
	}

	return pheromone.CreditResponse{
		UserID:  req.UserID,
		Balance: float64(bal.Balance),
		OK:      bal.Balance > 0,
	}, nil
}

func handleDeductCredit(data []byte) (interface{}, error) {
	var req pheromone.CreditRequest
	if err := json.Unmarshal(data, &req); err != nil {
		return nil, fmt.Errorf("invalid request: %w", err)
	}
	if req.UserID == "" || req.Amount <= 0 {
		return nil, fmt.Errorf("user_id and positive amount required")
	}

	amount := int64(req.Amount)

	var bal model.UserBalance
	if err := database.DB.Where("user_id = ?", req.UserID).First(&bal).Error; err != nil {
		return nil, fmt.Errorf("user not found")
	}
	if bal.Balance < amount {
		return pheromone.CreditResponse{
			UserID:  req.UserID,
			Balance: float64(bal.Balance),
			OK:      false,
		}, nil
	}

	// Deduct
	result := database.DB.Model(&bal).Update("balance", bal.Balance-amount)
	if result.Error != nil {
		return nil, fmt.Errorf("deduct failed: %w", result.Error)
	}

	return pheromone.CreditResponse{
		UserID:  req.UserID,
		Balance: float64(bal.Balance - amount),
		OK:      true,
	}, nil
}

type getUserRequest struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
}

type getUserResponse struct {
	ID       string `json:"id"`
	Email    string `json:"email"`
	Nickname string `json:"nickname"`
	Role     string `json:"role"`
	Status   string `json:"status"`
	Avatar   string `json:"avatar"`
}

func handleGetUser(data []byte) (interface{}, error) {
	var req getUserRequest
	if err := json.Unmarshal(data, &req); err != nil {
		return nil, fmt.Errorf("invalid request: %w", err)
	}

	var user model.User
	if req.UserID != "" {
		if err := database.DB.Where("id = ?", req.UserID).First(&user).Error; err != nil {
			return nil, fmt.Errorf("user not found")
		}
	} else if req.Email != "" {
		if err := database.DB.Where("email = ?", req.Email).First(&user).Error; err != nil {
			return nil, fmt.Errorf("user not found")
		}
	} else {
		return nil, fmt.Errorf("user_id or email required")
	}

	return getUserResponse{
		ID:       user.ID,
		Email:    user.Email,
		Nickname: user.Nickname,
		Role:     user.Role,
		Status:   user.Status,
		Avatar:   user.Avatar,
	}, nil
}
