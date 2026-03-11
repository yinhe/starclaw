package model

import "time"

// NodeBinding links a Queen user to a Claw node and its local user account.
// One Queen user can own multiple Claw nodes.
// One Claw node can only be owned by one Queen user.
type NodeBinding struct {
	ID          string    `json:"id" gorm:"primaryKey;type:varchar(36)"`
	QueenUserID string    `json:"queen_user_id" gorm:"type:varchar(36);index"`
	NodeID      string    `json:"node_id" gorm:"type:varchar(50);uniqueIndex"` // claw:xxxx address
	LocalUserID string    `json:"local_user_id" gorm:"type:varchar(36)"`       // user ID on the Claw node
	NodeName    string    `json:"node_name" gorm:"type:varchar(100)"`          // friendly name
	NodeAddr    string    `json:"node_addr" gorm:"type:varchar(200)"`          // reachable address (IP:port or domain)
	NodeRegion  string    `json:"node_region" gorm:"type:varchar(50)"`         // region hint
	NodeVersion string    `json:"node_version" gorm:"type:varchar(30)"`        // Claw version
	Status      string    `json:"status" gorm:"type:varchar(20);default:active"` // active / inactive / revoked
	LastSeen    time.Time `json:"last_seen"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
