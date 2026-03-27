package main

import (
	"fmt"
	"io/fs"
	"log"
	"os/exec"
	"runtime"

	"starclaw.net/spore/desktop"
	"starclaw.net/spore/desktop/api"
	"starclaw.net/spore/pkg/platform"
	sporeRuntime "starclaw.net/spore/pkg/runtime"
)

const version = "2026.0327.0134"
const defaultAddr = "127.0.0.1:7890"

func main() {
	info := platform.Detect()
	mgr := sporeRuntime.NewManager(info.SporeHome)

	// Get embedded web assets
	var webFS fs.FS
	if desktop.HasAssets() {
		subFS, err := fs.Sub(desktop.Assets, "web/dist")
		if err == nil {
			webFS = subFS
		}
	}

	srv := api.NewServer(mgr, webFS, version)

	// Auto-open browser
	go func() {
		url := fmt.Sprintf("http://%s", defaultAddr)
		log.Printf("[spore-desktop] Opening %s in browser...", url)
		openBrowser(url)
	}()

	fmt.Println("╔══════════════════════════════════════════╗")
	fmt.Printf("║   Spore Desktop v%-23s ║\n", version)
	fmt.Println("║   http://127.0.0.1:7890                  ║")
	fmt.Println("║   Press Ctrl+C to quit                   ║")
	fmt.Println("╚══════════════════════════════════════════╝")

	if err := srv.ListenAndServe(defaultAddr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
}
