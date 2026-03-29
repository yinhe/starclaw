package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"starclaw.net/spore/pkg/archive"
	"starclaw.net/spore/pkg/manifest"
)

const version = "0.1.0"

// Sporefile represents the build configuration.
type Sporefile struct {
	Name        string
	Version     string
	Description string
	Binary      string
	BuildCmd    string
	Args        []string
	Ports       []manifest.PortMapping
	HealthURL   string
	MinMemoryMB int
	Platforms   []string // "linux/amd64", "darwin/arm64", etc.
	Include     []string // extra files/dirs to include
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "build":
		cmdBuild()
	case "pack":
		cmdPack()
	case "version":
		fmt.Printf("hatchery v%s\n", version)
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		os.Exit(1)
	}
}

func cmdBuild() {
	outputDir := "dist"
	platforms := []string{fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH)}

	// Parse flags
	for i := 2; i < len(os.Args); i++ {
		switch {
		case os.Args[i] == "--output" && i+1 < len(os.Args):
			outputDir = os.Args[i+1]
			i++
		case os.Args[i] == "--platform" && i+1 < len(os.Args):
			platforms = strings.Split(os.Args[i+1], ",")
			i++
		case os.Args[i] == "--all":
			platforms = []string{
				"linux/amd64", "linux/arm64", "linux/arm",
				"darwin/amd64", "darwin/arm64",
				"windows/amd64",
			}
		}
	}

	os.MkdirAll(outputDir, 0755)

	// Look for Sporefile.yaml or use defaults
	sf := loadSporefileOrDefaults()

	for _, plat := range platforms {
		parts := strings.SplitN(plat, "/", 2)
		if len(parts) != 2 {
			fmt.Fprintf(os.Stderr, "invalid platform: %s\n", plat)
			continue
		}
		targetOS, targetArch := parts[0], parts[1]

		fmt.Printf("🔨 Building %s v%s for %s/%s...\n", sf.Name, sf.Version, targetOS, targetArch)

		// Create a staging directory
		stageDir, err := os.MkdirTemp("", "hatchery-stage-*")
		if err != nil {
			fmt.Fprintf(os.Stderr, "  ❌ create stage dir: %v\n", err)
			continue
		}

		// Run build command (cross-compile)
		binName := sf.Binary
		if targetOS == "windows" && !strings.HasSuffix(binName, ".exe") {
			binName += ".exe"
		}
		binDir := filepath.Join(stageDir, "bin")
		os.MkdirAll(binDir, 0755)

		buildCmd := sf.BuildCmd
		if buildCmd == "" {
			// Default Go build command — always inject version via ldflags
			buildCmd = fmt.Sprintf("go build -ldflags=\"-s -w -X github.com/yinhe/starclaw/internal/molt.Version=%s\" -o %s ./cmd/server",
				sf.Version, filepath.Join(binDir, filepath.Base(binName)))
		} else {
			// Replace variables in the build command
			buildCmd = strings.ReplaceAll(buildCmd, "${TARGET_OS}", targetOS)
			buildCmd = strings.ReplaceAll(buildCmd, "${TARGET_ARCH}", targetArch)
			buildCmd = strings.ReplaceAll(buildCmd, "${VERSION}", sf.Version)
		}

		cmd := exec.Command("sh", "-c", buildCmd)
		if runtime.GOOS == "windows" {
			cmd = exec.Command("cmd", "/c", buildCmd)
		}
		cmd.Env = append(os.Environ(),
			"CGO_ENABLED=0",
			"GOOS="+targetOS,
			"GOARCH="+targetArch,
		)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "  ❌ build failed: %v\n", err)
			os.RemoveAll(stageDir)
			continue
		}

		// Copy include files
		for _, inc := range sf.Include {
			src := inc
			dst := filepath.Join(stageDir, inc)
			os.MkdirAll(filepath.Dir(dst), 0755)
			copyFileOrDir(src, dst)
		}

		// Copy config dir if exists
		if fi, err := os.Stat("config"); err == nil && fi.IsDir() {
			copyFileOrDir("config", filepath.Join(stageDir, "config"))
		}

		// Generate manifest
		mf := manifest.NewDefault(sf.Name, sf.Version, "bin/"+filepath.Base(binName))
		mf.Platform = manifest.Platform{OS: targetOS, Arch: targetArch}
		mf.Description = sf.Description
		mf.Args = sf.Args
		mf.Network.Ports = sf.Ports
		if sf.HealthURL != "" {
			mf.Health.Endpoint = sf.HealthURL
		}
		if sf.MinMemoryMB > 0 {
			mf.Resources.MinMemoryMB = sf.MinMemoryMB
		}
		mf.BuiltAt = time.Now().UTC().Format(time.RFC3339)
		mf.BuiltBy = fmt.Sprintf("hatchery/%s", version)
		mf.Save(filepath.Join(stageDir, "manifest.json"))

		// Pack into .spore archive
		sporeName := mf.PackageName()
		sporePath := filepath.Join(outputDir, sporeName)

		checksum, err := archive.Pack(stageDir, sporePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  ❌ pack failed: %v\n", err)
			os.RemoveAll(stageDir)
			continue
		}

		fi, _ := os.Stat(sporePath)
		sizeMB := float64(fi.Size()) / (1024 * 1024)

		fmt.Printf("  ✅ %s (%.1f MB, %s)\n", sporeName, sizeMB, checksum[:20]+"...")

		os.RemoveAll(stageDir)
	}

	fmt.Println("\n🎉 Build complete!")
}

func cmdPack() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "usage: hatchery pack <directory> [--output <path>]")
		os.Exit(1)
	}
	srcDir := os.Args[2]
	output := ""
	for i := 3; i < len(os.Args); i++ {
		if os.Args[i] == "--output" && i+1 < len(os.Args) {
			output = os.Args[i+1]
			i++
		}
	}

	// Load manifest from directory to determine output name
	mf, err := manifest.Load(filepath.Join(srcDir, "manifest.json"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "load manifest: %v\n", err)
		os.Exit(1)
	}

	if output == "" {
		output = mf.PackageName()
	}

	fmt.Printf("📦 Packing %s → %s...\n", srcDir, output)
	checksum, err := archive.Pack(srcDir, output)
	if err != nil {
		fmt.Fprintf(os.Stderr, "pack: %v\n", err)
		os.Exit(1)
	}

	fi, _ := os.Stat(output)
	fmt.Printf("✅ %s (%.1f MB)\n", output, float64(fi.Size())/(1024*1024))
	fmt.Printf("   Checksum: %s\n", checksum)
}

func loadSporefileOrDefaults() *Sporefile {
	// TODO: parse Sporefile.yaml if present
	// For now return defaults for Claw
	return &Sporefile{
		Name:        "claw",
		Version:     resolveVersion(),
		Description: "StarClaw AI Agent Node",
		Binary:      "claw",
		Args:        []string{"serve"},
		Ports: []manifest.PortMapping{
			{Port: 8080, Protocol: "tcp", Description: "API Server"},
		},
		HealthURL:   "http://localhost:8080/health",
		MinMemoryMB: 256,
		Include:     []string{},
	}
}

// resolveVersion determines the build version from available sources.
// Priority: .version file → git tag → Nydus API → UTC timestamp (never "dev")
func resolveVersion() string {
	// 1. .version file (written by deploy scripts / Makefile)
	if data, err := os.ReadFile("api/.version"); err == nil {
		if v := strings.TrimSpace(string(data)); v != "" && v != "dev" {
			return v
		}
	}
	if data, err := os.ReadFile(".version"); err == nil {
		if v := strings.TrimSpace(string(data)); v != "" && v != "dev" {
			return v
		}
	}

	// 2. git describe
	if out, err := exec.Command("git", "describe", "--tags", "--abbrev=0").Output(); err == nil {
		if v := strings.TrimPrefix(strings.TrimSpace(string(out)), "v"); v != "" {
			return v
		}
	}

	// 3. Nydus API
	client := &http.Client{Timeout: 5 * time.Second}
	if resp, err := client.Get("https://nydus.starclaw.net/releases/latest"); err == nil {
		defer resp.Body.Close()
		var release struct {
			TagName string `json:"tag_name"`
		}
		if json.NewDecoder(resp.Body).Decode(&release) == nil {
			if v := strings.TrimPrefix(release.TagName, "v"); v != "" {
				return v
			}
		}
	}

	// 4. UTC timestamp as last resort
	return time.Now().UTC().Format("2006.0102.1504")
}

func copyFileOrDir(src, dst string) error {
	fi, err := os.Stat(src)
	if err != nil {
		return err
	}
	if fi.IsDir() {
		return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			rel, _ := filepath.Rel(src, path)
			target := filepath.Join(dst, rel)
			if info.IsDir() {
				return os.MkdirAll(target, info.Mode())
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			return os.WriteFile(target, data, info.Mode())
		})
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	os.MkdirAll(filepath.Dir(dst), 0755)
	return os.WriteFile(dst, data, fi.Mode())
}

func printUsage() {
	fmt.Printf(`Hatchery v%s — StarClaw Spore Build Tool

Usage: hatchery <command> [args]

Commands:
  build                    Build .spore packages
    --platform <os/arch>   Target platforms (comma-separated)
    --all                  Build for all supported platforms
    --output <dir>         Output directory (default: dist/)
  
  pack <directory>         Pack a directory into a .spore archive
    --output <path>        Output file path

  version                  Show hatchery version
  help                     Show this help

Examples:
  hatchery build
  hatchery build --platform linux/arm64,darwin/arm64
  hatchery build --all --output release/
  hatchery pack ./my-spore-dir --output my-app.spore

`, version)
}
