package envcheck

import (
	"os"
	"path/filepath"
	"runtime"
	"time"
)

type Result struct {
	ChromePath     string
	PackageManager string
	DevServerPort  int
	Framework      string
	DetectedAt     time.Time
}

func Detect(projectRoot string) Result {
	r := Result{DetectedAt: time.Now().UTC()}
	r.ChromePath = findChrome()
	r.PackageManager = findPackageManager(projectRoot)
	r.Framework, r.DevServerPort = findFramework(projectRoot)
	return r
}

func findChrome() string {
	if env := os.Getenv("CHROME_PATH"); env != "" {
		if _, err := os.Stat(env); err == nil {
			return env
		}
	}
	for _, p := range platformChromeCandidates() {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

func platformChromeCandidates() []string {
	switch runtime.GOOS {
	case "darwin":
		return []string{
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			"/Applications/Chromium.app/Contents/MacOS/Chromium",
		}
	case "linux":
		return []string{
			"/usr/bin/google-chrome",
			"/usr/bin/google-chrome-stable",
			"/usr/bin/chromium",
			"/usr/bin/chromium-browser",
		}
	}
	return nil
}

func findPackageManager(root string) string {
	for _, c := range []struct {
		file, mgr string
	}{
		{"pnpm-lock.yaml", "pnpm"},
		{"yarn.lock", "yarn"},
		{"bun.lockb", "bun"},
		{"package-lock.json", "npm"},
	} {
		if _, err := os.Stat(filepath.Join(root, c.file)); err == nil {
			return c.mgr
		}
	}
	return ""
}

func findFramework(root string) (string, int) {
	cfgs := []struct {
		patterns  []string
		framework string
		port      int
	}{
		{[]string{"next.config.js", "next.config.ts", "next.config.mjs"}, "next", 3000},
		{[]string{"vite.config.js", "vite.config.ts"}, "vite", 5173},
		{[]string{"astro.config.mjs", "astro.config.ts"}, "astro", 4321},
		{[]string{"svelte.config.js"}, "sveltekit", 5173},
	}
	for _, c := range cfgs {
		for _, p := range c.patterns {
			if _, err := os.Stat(filepath.Join(root, p)); err == nil {
				return c.framework, c.port
			}
		}
	}
	return "", 0
}
