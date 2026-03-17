package desktop

import "embed"

//go:embed web/dist/*
var Assets embed.FS

// HasAssets returns true if the embedded web assets contain files.
func HasAssets() bool {
	entries, err := Assets.ReadDir("web/dist")
	if err != nil {
		return false
	}
	return len(entries) > 0
}
