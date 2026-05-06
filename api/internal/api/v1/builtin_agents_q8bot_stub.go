//go:build !private

package v1

// OSS-safe stub for seedQ8botMarketplaceTemplate.
//
// Real implementation lives in builtin_agents_q8bot.go (//go:build private),
// which is private (proprietary trading strategy + full system prompt).
// Default `go build` (no tags) compiles this no-op — keeps OSS CI green
// without leaking the real template. Internal builds opt-in via `-tags private`.
//
// The Nydus polyrepo sync hook also strips the private file during claw.git
// overlay (defense in depth — see nydus/hooks/post-receive-starclaw).

import "gorm.io/gorm"

func seedQ8botMarketplaceTemplate(_ *gorm.DB, _ string) {
	// no-op in OSS build
}
