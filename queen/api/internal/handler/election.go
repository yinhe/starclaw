package handler

import (
	"fmt"
	"log"
	"net/http"
	"sort"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"starclaw.net/queen/api/internal/database"
	"starclaw.net/queen/api/internal/middleware"
	"starclaw.net/queen/api/internal/model"
	"gorm.io/gorm"
)

type ElectionHandler struct{}

// ════════════════════════════════════════════════════════════
// Admin: Create / Close / List elections (Queen 运营平台)
// ════════════════════════════════════════════════════════════

// POST /admin/election — Create a new Cerebrate election
func (h *ElectionHandler) AdminCreateElection(c *gin.Context) {
	var req struct {
		Title        string `json:"title" binding:"required"`
		Description  string `json:"description"`
		Seats        int    `json:"seats"`
		DurationDays int    `json:"duration_days"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数不完整"})
		return
	}

	if req.Seats <= 0 {
		req.Seats = model.MaxCerebrates
	}
	if req.DurationDays <= 0 {
		req.DurationDays = 7
	}

	// Only one open election at a time
	var openCount int64
	database.DB.Model(&model.TeamElection{}).Where("status = ?", "open").Count(&openCount)
	if openCount > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "已有进行中的选举，请先关闭"})
		return
	}

	now := time.Now()
	election := model.TeamElection{
		ID:          uuid.New().String(),
		Title:       req.Title,
		Description: req.Description,
		Seats:       req.Seats,
		Status:      "open",
		StartAt:     now,
		EndAt:       now.AddDate(0, 0, req.DurationDays),
	}
	database.DB.Create(&election)

	log.Printf("[election] Created: %s seats=%d ends=%s", election.Title, election.Seats, election.EndAt.Format("2006-01-02"))
	c.JSON(http.StatusCreated, gin.H{"election": election})
}

// POST /admin/election/:id/close — Close an election and apply results
func (h *ElectionHandler) AdminCloseElection(c *gin.Context) {
	electionID := c.Param("id")

	var election model.TeamElection
	if err := database.DB.Where("id = ?", electionID).First(&election).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "选举不存在"})
		return
	}
	if election.Status != "open" {
		c.JSON(http.StatusConflict, gin.H{"error": fmt.Sprintf("选举状态为 %s，无法关闭", election.Status)})
		return
	}

	db := database.DB

	// Tally votes
	type voteCount struct {
		CandidateID string
		Votes       int64
	}
	var results []voteCount
	db.Model(&model.TeamVote{}).
		Select("candidate_id, COUNT(*) as votes").
		Where("election_id = ?", electionID).
		Group("candidate_id").
		Order("votes DESC").
		Find(&results)

	// Sort by votes descending (already ordered by SQL, but ensure)
	sort.Slice(results, func(i, j int) bool { return results[i].Votes > results[j].Votes })

	// Determine winners (top N by seats)
	winnerIDs := make([]string, 0, election.Seats)
	for i, r := range results {
		if i >= election.Seats {
			break
		}
		winnerIDs = append(winnerIDs, r.CandidateID)
	}

	// Apply results in transaction
	err := db.Transaction(func(tx *gorm.DB) error {
		// Demote all current Cerebrates to Overlord
		tx.Model(&model.TeamPartner{}).
			Where("level = ? AND status = ?", "cerebrate", "active").
			Update("level", "overlord")

		// Promote winners to Cerebrate
		if len(winnerIDs) > 0 {
			tx.Model(&model.TeamPartner{}).
				Where("id IN ? AND status = ?", winnerIDs, "active").
				Update("level", "cerebrate")
		}

		// Close election
		now := time.Now()
		tx.Model(&election).Updates(map[string]interface{}{
			"status":    "closed",
			"closed_at": &now,
		})

		return nil
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Build response with winner names
	type winner struct {
		ID    string `json:"id"`
		Name  string `json:"name"`
		Votes int64  `json:"votes"`
	}
	var winners []winner
	for _, r := range results {
		var p model.TeamPartner
		if err := db.Where("id = ?", r.CandidateID).First(&p).Error; err == nil {
			w := winner{ID: p.ID, Name: p.Name, Votes: r.Votes}
			winners = append(winners, w)
		}
	}

	log.Printf("[election] Closed: %s, %d winners elected", election.Title, len(winnerIDs))
	c.JSON(http.StatusOK, gin.H{
		"message":  fmt.Sprintf("选举已关闭，%d 位脑虫当选", len(winnerIDs)),
		"winners":  winners,
		"election": election,
	})
}

// GET /admin/election — List all elections
func (h *ElectionHandler) AdminListElections(c *gin.Context) {
	var elections []model.TeamElection
	database.DB.Order("created_at DESC").Find(&elections)
	c.JSON(http.StatusOK, gin.H{"elections": elections})
}

// DELETE /admin/election/:id — Cancel an open election
func (h *ElectionHandler) AdminCancelElection(c *gin.Context) {
	electionID := c.Param("id")
	var election model.TeamElection
	if err := database.DB.Where("id = ? AND status = ?", electionID, "open").First(&election).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "未找到进行中的选举"})
		return
	}
	database.DB.Model(&election).Update("status", "cancelled")
	c.JSON(http.StatusOK, gin.H{"message": "选举已取消"})
}

// ════════════════════════════════════════════════════════════
// Team Partner: Vote + View (团队合伙人)
// ════════════════════════════════════════════════════════════

// POST /partner/election/vote — Cast a vote (team partners only)
func (h *ElectionHandler) Vote(c *gin.Context) {
	partnerID := c.GetString("partner_id")

	var req struct {
		CandidateID string `json:"candidate_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请指定候选人"})
		return
	}

	db := database.DB

	// Find open election
	var election model.TeamElection
	if err := db.Where("status = ?", "open").First(&election).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "当前没有进行中的选举"})
		return
	}

	// Check election not expired
	if time.Now().After(election.EndAt) {
		c.JSON(http.StatusConflict, gin.H{"error": "选举已过投票截止时间"})
		return
	}

	// Validate candidate is an active team partner
	var candidate model.TeamPartner
	if err := db.Where("id = ? AND status = ?", req.CandidateID, "active").First(&candidate).Error; err != nil {
		middleware.Fail(c, http.StatusNotFound, middleware.CodeNotFound, "候选人不存在或已停用")
		return
	}

	// Cannot vote for self
	if req.CandidateID == partnerID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "不能投票给自己"})
		return
	}

	// Check if already voted in this election
	var existingVote model.TeamVote
	if err := db.Where("election_id = ? AND voter_id = ?", election.ID, partnerID).First(&existingVote).Error; err == nil {
		// Already voted — update vote
		db.Model(&existingVote).Update("candidate_id", req.CandidateID)
		c.JSON(http.StatusOK, gin.H{"message": "投票已更新", "candidate": candidate.Name})
		return
	}

	// Cast new vote
	vote := model.TeamVote{
		ID:          uuid.New().String(),
		ElectionID:  election.ID,
		VoterID:     partnerID,
		CandidateID: req.CandidateID,
	}
	db.Create(&vote)

	log.Printf("[election] Vote: voter=%s candidate=%s (%s)", partnerID, req.CandidateID, candidate.Name)
	c.JSON(http.StatusCreated, gin.H{"message": "投票成功", "candidate": candidate.Name})
}

// GET /partner/election — View current election + results
func (h *ElectionHandler) GetCurrentElection(c *gin.Context) {
	partnerID := c.GetString("partner_id")
	db := database.DB

	// Find latest election (open or most recently closed)
	var election model.TeamElection
	if err := db.Order("created_at DESC").First(&election).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "暂无选举"})
		return
	}

	// Get vote tally
	type candidate struct {
		ID    string `json:"id"`
		Name  string `json:"name"`
		Votes int64  `json:"votes"`
		Level string `json:"level"`
	}
	type voteRow struct {
		CandidateID string
		Votes       int64
	}
	var rows []voteRow
	db.Model(&model.TeamVote{}).
		Select("candidate_id, COUNT(*) as votes").
		Where("election_id = ?", election.ID).
		Group("candidate_id").
		Order("votes DESC").
		Find(&rows)

	var candidates []candidate
	for _, r := range rows {
		var p model.TeamPartner
		if err := db.Where("id = ?", r.CandidateID).First(&p).Error; err == nil {
			candidates = append(candidates, candidate{
				ID: p.ID, Name: p.Name, Votes: r.Votes, Level: p.Level,
			})
		}
	}

	// Total voters
	var totalVoters int64
	db.Model(&model.TeamVote{}).Where("election_id = ?", election.ID).Count(&totalVoters)

	// Total eligible voters
	var totalEligible int64
	db.Model(&model.TeamPartner{}).Where("status = ?", "active").Count(&totalEligible)

	// My vote
	var myVote model.TeamVote
	myVotedFor := ""
	if err := db.Where("election_id = ? AND voter_id = ?", election.ID, partnerID).First(&myVote).Error; err == nil {
		myVotedFor = myVote.CandidateID
	}

	c.JSON(http.StatusOK, gin.H{
		"election":       election,
		"candidates":     candidates,
		"total_voters":   totalVoters,
		"total_eligible": totalEligible,
		"my_vote":        myVotedFor,
	})
}

// GET /partner/election/candidates — List all eligible candidates (active team partners)
func (h *ElectionHandler) ListCandidates(c *gin.Context) {
	partnerID := c.GetString("partner_id")

	var partners []model.TeamPartner
	database.DB.Where("status = ? AND id != ?", "active", partnerID).
		Select("id, name, region, level").
		Order("name ASC").
		Find(&partners)

	c.JSON(http.StatusOK, gin.H{"candidates": partners})
}
