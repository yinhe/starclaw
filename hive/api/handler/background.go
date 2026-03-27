package handler

import (
	"fmt"
	"log"
	"os/exec"
	"strings"
	"time"

	"starclaw.net/hive/api/model"
)

// BackgroundCleanupLoop runs every interval to destroy expired free-tier instances.
func (h *HiveHandler) BackgroundCleanupLoop(interval time.Duration) {
	log.Printf("[hive] background cleanup started (every %s)", interval)
	time.Sleep(30 * time.Second) // delay startup
	for {
		h.cleanupExpired()
		time.Sleep(interval)
	}
}

func (h *HiveHandler) cleanupExpired() {
	var expired []model.ClawInstance
	h.db.Where("expires_at IS NOT NULL AND expires_at < ? AND status NOT IN ('destroying','destroyed')", time.Now()).Find(&expired)

	if len(expired) == 0 {
		return
	}

	log.Printf("[hive] cleanup: found %d expired instances", len(expired))
	for _, inst := range expired {
		log.Printf("[hive] cleanup: destroying expired instance %s (expired %s)", inst.Slug, inst.ExpiresAt)
		go h.destroyInstanceBackground(&inst)
	}
}

// BackgroundRenewalLoop checks paid instances approaching expiry and auto-renews.
func (h *HiveHandler) BackgroundRenewalLoop(interval time.Duration) {
	log.Printf("[hive] background renewal started (every %s)", interval)
	time.Sleep(60 * time.Second) // delay startup
	for {
		h.checkRenewals()
		time.Sleep(interval)
	}
}

func (h *HiveHandler) checkRenewals() {
	if h.billing == nil {
		return
	}

	// Find paid instances expiring within 24 hours that are still running
	cutoff := time.Now().Add(24 * time.Hour)
	var expiring []model.ClawInstance
	h.db.Where("owner_id != '' AND status = 'running' AND expires_at IS NOT NULL AND expires_at < ?", cutoff).Find(&expiring)

	for _, inst := range expiring {
		// Find the plan
		var lastOrder model.Order
		if err := h.db.Where("instance_id = ? AND status = 'paid'", inst.ID).Order("created_at DESC").First(&lastOrder).Error; err != nil {
			continue // no paid order = free tier, skip
		}

		var plan model.Plan
		if err := h.db.Where("id = ? AND is_active = ?", lastOrder.PlanID, true).First(&plan).Error; err != nil {
			continue
		}

		if plan.PriceMonthly <= 0 {
			continue // free plan, handled by cleanup
		}

		// Try to renew: freeze → settle → extend expiry
		log.Printf("[hive] renewal: attempting to renew %s (plan=%s, cost=%d星能)", inst.Slug, plan.ID, plan.PriceMonthly)

		// Use Consume (direct deduction) for renewals — simpler than freeze+settle
		_, err := h.billing.Consume(inst.OwnerID, plan.PriceMonthly, "hive_renew",
			fmt.Sprintf("Hive %s (%s) 月费续期", inst.Slug, plan.DisplayName))
		if err != nil {
			log.Printf("[hive] renewal: consume failed for %s: %v — will expire", inst.Slug, err)
			NotifyInstanceError(inst.ID, inst.Slug, "renewal_failed: "+err.Error())
			continue
		}

		// Extend expiry by 1 month
		now := time.Now()
		newExpiry := now.AddDate(0, 1, 0)
		h.db.Model(&inst).Updates(map[string]interface{}{
			"expires_at":     newExpiry,
			"last_active_at": now,
		})

		// Create renewal order record
		h.db.Create(&model.Order{
			ID:          fmt.Sprintf("renew-%s-%s", inst.ID[:8], now.Format("0102")),
			InstanceID:  inst.ID,
			ClawID:      inst.OwnerID,
			PlanID:      plan.ID,
			Type:        "renew",
			Amount:      plan.PriceMonthly,
			Status:      "paid",
			PeriodStart: now,
			PeriodEnd:   newExpiry,
			CreatedAt:   now,
		})

		log.Printf("[hive] renewal: %s renewed until %s (%d星能)", inst.Slug, newExpiry.Format("2006-01-02"), plan.PriceMonthly)
	}
}

// BackgroundHealthLoop periodically checks all running containers and auto-restarts crashed ones.
func (h *HiveHandler) BackgroundHealthLoop(interval time.Duration) {
	log.Printf("[hive] background health monitor started (every %s)", interval)
	time.Sleep(90 * time.Second) // delay startup
	for {
		h.healthCheck()
		time.Sleep(interval)
	}
}

func (h *HiveHandler) healthCheck() {
	var instances []model.ClawInstance
	h.db.Where("status IN ('running','error') AND deploy_mode IN ('hive','lite')").Find(&instances)

	for _, inst := range instances {
		if inst.ContainerID == "" {
			continue
		}

		if !containerExists(inst.ContainerID) {
			log.Printf("[hive] health: stale container id for %s (%s) — marking stopped", inst.Slug, inst.ContainerID)
			h.db.Model(&inst).Updates(map[string]interface{}{"status": "stopped", "container_id": ""})
			continue
		}

		running := isContainerRunning(inst.ContainerID)
		now := time.Now()

		// Auto-recover: if a previously error instance is actually running, mark it back to running.
		if running {
			if inst.Status == "error" {
				log.Printf("[hive] health: recovered %s (%s) — status error -> running", inst.Slug, inst.ContainerID)
				h.db.Model(&inst).Updates(map[string]interface{}{"status": "running", "last_active_at": now})
			} else {
				h.db.Model(&inst).Update("last_active_at", now)
			}
			continue
		}

		log.Printf("[hive] health: container %s (%s) is NOT running — restarting", inst.Slug, inst.ContainerID)

		if err := h.docker.StartContainer(inst.ContainerID); err != nil {
			log.Printf("[hive] health: restart failed for %s: %v — marking error", inst.Slug, err)
			h.db.Model(&inst).Update("status", "error")
			NotifyInstanceError(inst.ID, inst.Slug, "container_crashed: restart failed")
			continue
		}

		log.Printf("[hive] health: restarted %s successfully", inst.Slug)
		h.db.Model(&inst).Updates(map[string]interface{}{"status": "running", "last_active_at": now})
	}
}

// isContainerRunning checks if a Docker container is currently running.
func isContainerRunning(containerID string) bool {
	out, err := exec.Command("docker", "inspect", "-f", "{{.State.Running}}", containerID).CombinedOutput()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) == "true"
}

func containerExists(containerID string) bool {
	out, err := exec.Command("docker", "inspect", "-f", "{{.Id}}", containerID).CombinedOutput()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) != ""
}

// destroyInstanceBackground handles async instance destruction (reused by cleanup + API).
func (h *HiveHandler) destroyInstanceBackground(inst *model.ClawInstance) {
	h.db.Model(inst).Update("status", "destroying")

	// Stop and remove container
	if inst.ContainerID != "" {
		h.docker.StopContainer(inst.ContainerID)
		h.docker.RemoveContainer(inst.ContainerID)
	}

	// Remove nginx config
	h.nginx.RemoveConfig(inst.Slug)
	h.nginx.TestConfig()
	h.nginx.Reload()

	// Remove DNS record
	if h.dns != nil {
		h.dns.DeleteRecord(inst.Slug)
	}

	// Drop MySQL database (hive mode)
	if inst.DBName != "" {
		h.mysql.DropDatabase(inst.Slug)
	}

	h.db.Model(inst).Update("status", "destroyed")
	NotifyInstanceDeleted(inst.ID, inst.Slug)
	log.Printf("[hive] instance %s destroyed", inst.Slug)
}
