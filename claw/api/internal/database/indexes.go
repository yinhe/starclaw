package database

import (
	"log"

	"gorm.io/gorm"
)

// EnsureIndexes creates composite indexes that GORM AutoMigrate doesn't handle.
// These target the hottest query patterns identified in the codebase.
func EnsureIndexes(db *gorm.DB) {
	indexes := []struct {
		Table   string
		Name    string
		Columns string
	}{
		// ── Core tables ──
		// Conversations: user list with pinned-first, recent-first
		{"conversations", "idx_conv_user_pinned_updated", "user_id, is_pinned DESC, updated_at DESC"},
		// Messages: conversation timeline (most frequent query)
		{"messages", "idx_msg_conv_created", "conversation_id, created_at"},
		// Agents: user's agent list
		{"agents", "idx_agent_user_public", "user_id, is_public"},

		// ── Observability ──
		// TraceSpans: by trace (reconstruct full trace) and by time range
		{"trace_spans", "idx_span_trace_start", "trace_id, start_time"},
		{"trace_spans", "idx_span_agent_start", "agent_id, start_time DESC"},
		{"trace_spans", "idx_span_user_start", "user_id, start_time DESC"},
		// LogEntries: by level + time (log search)
		{"log_entries", "idx_log_level_time", "level, timestamp DESC"},
		{"log_entries", "idx_log_trace", "trace_id, timestamp"},
		// AlertHistory: recent alerts
		{"alert_histories", "idx_alert_rule_fired", "rule_id, fired_at DESC"},

		// ── Webhook ──
		// EventRules: lookup by event type
		{"event_rules", "idx_rule_event_enabled", "event_type, enabled"},
		// EventLogs: by rule and status (retry queue)
		{"event_logs", "idx_evlog_rule_status", "rule_id, status"},
		{"event_logs", "idx_evlog_status_retry", "status, retry_count"},

		// ── Marketplace ──
		// AgentListings: browse with filters
		{"agent_listings", "idx_listing_status_category", "status, category"},
		{"agent_listings", "idx_listing_trending", "status, downloads DESC"},
		// AgentPurchases: user's purchases
		{"agent_purchases", "idx_purchase_user_listing", "user_id, listing_id"},

		// ── Knowledge ──
		// DocumentChunks: retrieval by knowledge base
		{"document_chunks", "idx_chunk_doc", "document_id"},

		// ── Security ──
		// AuditChainEntries: chain traversal and query
		{"audit_chain_entries", "idx_audit_seq", "sequence"},
		{"audit_chain_entries", "idx_audit_actor_time", "actor, created_at DESC"},
		{"audit_chain_entries", "idx_audit_action_time", "action, created_at DESC"},

		// ── Agent Goals ──
		{"goals", "idx_goal_user_status", "user_id, status"},
		{"goals", "idx_goal_status_deadline", "status, deadline"},
		// GoalSteps: by goal
		{"goal_steps", "idx_step_goal_seq", "goal_id, step_number"},

		// ── Plugins ──
		{"plugin_listings", "idx_plugin_status_category", "status, category"},
		{"plugin_installs", "idx_pinstall_user", "user_id, plugin_id"},

		// ── FineTune ──
		{"lo_ra_adapters", "idx_lora_user_status", "user_id, status"},
		{"training_samples", "idx_sample_adapter", "adapter_id, quality DESC"},
		{"distillation_jobs", "idx_distill_user_status", "user_id, status"},
	}

	for _, idx := range indexes {
		sql := "CREATE INDEX IF NOT EXISTS " + idx.Name + " ON " + idx.Table + " (" + idx.Columns + ")"
		if err := db.Exec(sql).Error; err != nil {
			// Not fatal — table might not exist yet or index syntax differs
			log.Printf("[DB] index %s: %v", idx.Name, err)
		}
	}

	log.Printf("[DB] Ensured %d composite indexes", len(indexes))
}
