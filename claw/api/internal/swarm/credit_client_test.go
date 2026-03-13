package swarm

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/yinhe/starclaw/internal/node"
)

func makeTestIdentity() *node.Identity {
	pub, priv, _ := ed25519.GenerateKey(nil)
	hash := sha256.Sum256(pub)
	nodeID := "claw:" + hex.EncodeToString(hash[:])[:40]
	return &node.Identity{
		PublicKey:  pub,
		PrivateKey: priv,
		NodeID:     nodeID,
	}
}

func TestHPStatus_Constants(t *testing.T) {
	tests := []struct {
		hp   HPStatus
		want string
	}{
		{HPFull, "full"},
		{HPHealthy, "healthy"},
		{HPLow, "low"},
		{HPCritical, "critical"},
		{HPHibernated, "hibernated"},
		{HPUnknown, "unknown"},
	}
	for _, tt := range tests {
		if string(tt.hp) != tt.want {
			t.Errorf("HPStatus %v: got %q, want %q", tt.hp, string(tt.hp), tt.want)
		}
	}
}

func TestCreditClient_UpdateFromHeartbeat(t *testing.T) {
	id := makeTestIdentity()
	cc := NewCreditClient("http://fake-queen", id)

	if cc.HP() != HPUnknown {
		t.Errorf("initial HP: got %q, want %q", cc.HP(), HPUnknown)
	}

	cb := &CreditBalance{
		Balance:      5000000,
		BalanceStars: 500.0,
		HPStatus:     "healthy",
		TrustLevel:   "basic",
		UpdatedAt:    time.Now(),
	}

	cc.UpdateFromHeartbeat(cb)

	if cc.HP() != HPHealthy {
		t.Errorf("HP after update: got %q, want %q", cc.HP(), HPHealthy)
	}

	cached := cc.CachedBalance()
	if cached == nil {
		t.Fatal("cached balance should not be nil")
	}
	if cached.BalanceStars != 500.0 {
		t.Errorf("cached balance: got %.1f, want 500.0", cached.BalanceStars)
	}
}

func TestCreditClient_HPChangeCallback(t *testing.T) {
	id := makeTestIdentity()
	cc := NewCreditClient("http://fake-queen", id)

	var callbackHP HPStatus
	cc.OnHPChange(func(hp HPStatus) {
		callbackHP = hp
	})

	// First update sets HP from unknown → healthy (no callback for initial)
	cc.UpdateFromHeartbeat(&CreditBalance{HPStatus: "healthy"})

	// Second update: healthy → critical (should trigger callback)
	cc.UpdateFromHeartbeat(&CreditBalance{HPStatus: "critical"})

	if callbackHP != HPCritical {
		t.Errorf("callback HP: got %q, want %q", callbackHP, HPCritical)
	}
}

func TestCreditClient_QueryBalance(t *testing.T) {
	id := makeTestIdentity()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/credits/balance" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		clawID := r.URL.Query().Get("claw_id")
		if clawID != id.NodeID {
			t.Errorf("claw_id: got %q, want %q", clawID, id.NodeID)
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"code": 0,
			"data": map[string]interface{}{
				"claw_id":       clawID,
				"balance":       1000000,
				"balance_stars": 100.0,
				"frozen":        50000,
				"frozen_stars":  5.0,
				"total_in":      2000000,
				"total_out":     1000000,
				"nonce":         3,
				"status":        "active",
				"hp_status":     "healthy",
				"trust_level":   "verified",
			},
		})
	}))
	defer server.Close()

	cc := NewCreditClient(server.URL, id)
	balance, err := cc.QueryBalance()
	if err != nil {
		t.Fatalf("QueryBalance: %v", err)
	}

	if balance.BalanceStars != 100.0 {
		t.Errorf("balance_stars: got %.1f, want 100.0", balance.BalanceStars)
	}
	if balance.Nonce != 3 {
		t.Errorf("nonce: got %d, want 3", balance.Nonce)
	}
	if balance.HPStatus != "healthy" {
		t.Errorf("hp_status: got %q, want %q", balance.HPStatus, "healthy")
	}
	if cc.HP() != HPHealthy {
		t.Errorf("HP after query: got %q, want %q", cc.HP(), HPHealthy)
	}
}

func TestCreditClient_Transfer(t *testing.T) {
	id := makeTestIdentity()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/credits/balance":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"code": 0,
				"data": map[string]interface{}{
					"balance": 1000000, "balance_stars": 100.0, "nonce": 5,
					"hp_status": "healthy", "status": "active", "trust_level": "basic",
				},
			})
		case "/v1/credits/transfer":
			var body map[string]interface{}
			json.NewDecoder(r.Body).Decode(&body)

			// Verify required fields
			if body["from_claw"] != id.NodeID {
				t.Errorf("from_claw mismatch")
			}
			if body["signature"] == nil || body["signature"] == "" {
				t.Error("missing signature")
			}
			if body["public_key"] == nil || body["public_key"] == "" {
				t.Error("missing public_key")
			}

			json.NewEncoder(w).Encode(map[string]interface{}{
				"code": 0,
				"data": map[string]interface{}{
					"txn_id":       "txn-001",
					"from":         id.NodeID,
					"to":           body["to_claw"],
					"amount":       body["amount"],
					"amount_stars": 10.0,
					"new_balance":  900000,
				},
			})
		default:
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	cc := NewCreditClient(server.URL, id)

	result, err := cc.Transfer(TransferRequest{
		ToClaw: "claw:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		Amount: 100000,
		Remark: "test transfer",
	})
	if err != nil {
		t.Fatalf("Transfer: %v", err)
	}
	if result.TxnID != "txn-001" {
		t.Errorf("txn_id: got %q, want %q", result.TxnID, "txn-001")
	}
	if result.NewBalance != 900000 {
		t.Errorf("new_balance: got %d, want 900000", result.NewBalance)
	}
}

func TestCreditClient_TransferValidation(t *testing.T) {
	id := makeTestIdentity()
	cc := NewCreditClient("http://fake", id)

	// Zero amount
	_, err := cc.Transfer(TransferRequest{ToClaw: "claw:bbb", Amount: 0})
	if err == nil {
		t.Error("expected error for zero amount")
	}

	// Negative amount
	_, err = cc.Transfer(TransferRequest{ToClaw: "claw:bbb", Amount: -100})
	if err == nil {
		t.Error("expected error for negative amount")
	}

	// Invalid address
	_, err = cc.Transfer(TransferRequest{ToClaw: "invalid", Amount: 100})
	if err == nil {
		t.Error("expected error for invalid address")
	}

	// Self transfer
	_, err = cc.Transfer(TransferRequest{ToClaw: id.NodeID, Amount: 100})
	if err == nil {
		t.Error("expected error for self transfer")
	}
}

func TestCreditClient_ListTransactions(t *testing.T) {
	id := makeTestIdentity()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/credits/transactions" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		page := r.URL.Query().Get("page")
		if page != "1" {
			t.Errorf("page: got %q, want %q", page, "1")
		}
		txnType := r.URL.Query().Get("type")
		if txnType != "transfer" {
			t.Errorf("type: got %q, want %q", txnType, "transfer")
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"code": 0,
			"data": map[string]interface{}{
				"transactions": []map[string]interface{}{
					{"id": "t1", "type": "transfer", "amount": 50000, "status": "confirmed"},
					{"id": "t2", "type": "transfer", "amount": 30000, "status": "confirmed"},
				},
				"total":     2,
				"page":      1,
				"page_size": 20,
			},
		})
	}))
	defer server.Close()

	cc := NewCreditClient(server.URL, id)
	list, err := cc.ListTransactions(1, 20, "transfer")
	if err != nil {
		t.Fatalf("ListTransactions: %v", err)
	}
	if list.Total != 2 {
		t.Errorf("total: got %d, want 2", list.Total)
	}
	if len(list.Transactions) != 2 {
		t.Errorf("transactions count: got %d, want 2", len(list.Transactions))
	}
}

func TestCreditClient_Stats(t *testing.T) {
	id := makeTestIdentity()
	cc := NewCreditClient("http://fake", id)

	stats := cc.Stats()
	if stats["hp"] != "unknown" {
		t.Errorf("initial hp: got %q, want %q", stats["hp"], "unknown")
	}
	if stats["claw_id"] != id.NodeID {
		t.Errorf("claw_id mismatch")
	}

	// After update
	cc.UpdateFromHeartbeat(&CreditBalance{
		BalanceStars: 500.0,
		HPStatus:     "full",
		TrustLevel:   "verified",
	})

	stats = cc.Stats()
	if stats["balance_stars"] != 500.0 {
		t.Errorf("balance_stars: got %v, want 500.0", stats["balance_stars"])
	}
}

func TestCreditClient_NilQueenURL(t *testing.T) {
	id := makeTestIdentity()
	cc := NewCreditClient("", id)

	_, err := cc.QueryBalance()
	if err == nil {
		t.Error("expected error for empty queen_url")
	}

	_, err = cc.Transfer(TransferRequest{ToClaw: "claw:bbb", Amount: 100})
	if err == nil {
		t.Error("expected error for empty queen_url")
	}

	_, err = cc.ListTransactions(1, 20, "")
	if err == nil {
		t.Error("expected error for empty queen_url")
	}
}
