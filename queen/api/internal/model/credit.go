package model

import "time"

// CreditAccount tracks a claw node's star credit balance (星力账户)
type CreditAccount struct {
	ID          string    `json:"id" gorm:"primaryKey;type:varchar(36)"`
	ClawID      string    `json:"claw_id" gorm:"type:varchar(60);uniqueIndex"` // claw:xxxx address
	PublicKey   string    `json:"public_key" gorm:"type:varchar(128)"`         // hex-encoded Ed25519 public key
	Balance     int64     `json:"balance" gorm:"default:0"`                    // 可用余额，单位：最小星力（1 Star = 10000 units）
	Frozen      int64     `json:"frozen" gorm:"default:0"`                     // 冻结金额（赏金/押金）
	TotalIn     int64     `json:"total_in" gorm:"default:0"`                   // 累计收入
	TotalOut    int64     `json:"total_out" gorm:"default:0"`                  // 累计支出
	Nonce       int64     `json:"nonce" gorm:"default:0"`                      // 最新 nonce（防重放）
	Status      string    `json:"status" gorm:"type:varchar(20);default:active"` // active / hibernated / banned
	TrustLevel  string    `json:"trust_level" gorm:"type:varchar(20);default:basic"` // basic / verified / certified
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// CreditTransaction records every star credit change (星力交易流水)
type CreditTransaction struct {
	ID        string    `json:"id" gorm:"primaryKey;type:varchar(36)"`
	FromClaw  string    `json:"from_claw" gorm:"type:varchar(60);index"` // 发送方 claw 地址（grant 时为 "system"）
	ToClaw    string    `json:"to_claw" gorm:"type:varchar(60);index"`   // 接收方 claw 地址
	Amount    int64     `json:"amount"`                                  // 交易金额（正数）
	Fee       int64     `json:"fee" gorm:"default:0"`                    // 手续费
	Type      string    `json:"type" gorm:"type:varchar(30);index"`      // transfer / grant / consume / mining_reward / bounty / freeze / unfreeze / settle
	Nonce     int64     `json:"nonce"`                                   // 发送方 nonce
	Signature string    `json:"signature,omitempty" gorm:"type:text"`    // Ed25519 签名（hex）
	Remark    string    `json:"remark" gorm:"type:varchar(500)"`
	Status    string    `json:"status" gorm:"type:varchar(20);default:confirmed"` // confirmed / failed
	CreatedAt time.Time `json:"created_at"`
}

// CreditFreeze records frozen credits (冻结记录)
type CreditFreeze struct {
	ID        string     `json:"id" gorm:"primaryKey;type:varchar(36)"`
	ClawID    string     `json:"claw_id" gorm:"type:varchar(60);index"`
	Amount    int64      `json:"amount"`                                    // 冻结金额
	Reason    string     `json:"reason" gorm:"type:varchar(30)"`            // bounty / deposit / multisig
	RefID     string     `json:"ref_id" gorm:"type:varchar(36);index"`      // 关联 ID（赏金 ID / 押金 ID）
	Status    string     `json:"status" gorm:"type:varchar(20);default:frozen"` // frozen / released / settled
	ReleasedAt *time.Time `json:"released_at"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}
