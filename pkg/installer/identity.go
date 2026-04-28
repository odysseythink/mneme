package installer

import (
	_ "embed"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/ranwei/mneme/pkg/state"
)

//go:embed templates/identity.md.tmpl
var identityTemplate string

// ProjectMetadata is the data model rendered into identity.md.
type ProjectMetadata struct {
	Name           string
	Language       string
	Framework      string
	PackageManager string
	DevServerURL   string
	ProjectRoot    string
	Intent         string
}

// DetectProjectMetadata inspects projectRoot and returns best-effort facts.
// Never fails — fields default to "unknown" when undetectable.
func DetectProjectMetadata(projectRoot string) ProjectMetadata {
	m := ProjectMetadata{
		Name:           filepath.Base(projectRoot),
		Language:       "unknown",
		Framework:      "unknown",
		PackageManager: "unknown",
		DevServerURL:   "unknown",
		ProjectRoot:    projectRoot,
		Intent:         "(no README description detected)",
	}

	switch {
	case fileExists(projectRoot, "go.mod"):
		m.Language = "Go"
	case fileExists(projectRoot, "package.json"):
		m.Language = "JavaScript/TypeScript"
	case fileExists(projectRoot, "pyproject.toml"), fileExists(projectRoot, "setup.py"), fileExists(projectRoot, "requirements.txt"):
		m.Language = "Python"
	case fileExists(projectRoot, "Cargo.toml"):
		m.Language = "Rust"
	case fileExists(projectRoot, "pom.xml"), fileExists(projectRoot, "build.gradle"):
		m.Language = "Java"
	}

	if m.Language == "JavaScript/TypeScript" {
		switch {
		case fileExists(projectRoot, "pnpm-lock.yaml"):
			m.PackageManager = "pnpm"
		case fileExists(projectRoot, "yarn.lock"):
			m.PackageManager = "yarn"
		case fileExists(projectRoot, "bun.lockb"):
			m.PackageManager = "bun"
		case fileExists(projectRoot, "package-lock.json"):
			m.PackageManager = "npm"
		default:
			m.PackageManager = "npm"
		}
	}

	switch {
	case fileExists(projectRoot, "next.config.js"), fileExists(projectRoot, "next.config.ts"), fileExists(projectRoot, "next.config.mjs"):
		m.Framework = "Next.js"
		m.DevServerURL = "http://localhost:3000"
	case fileExists(projectRoot, "vite.config.ts"), fileExists(projectRoot, "vite.config.js"):
		m.Framework = "Vite"
		m.DevServerURL = "http://localhost:5173"
	case fileExists(projectRoot, "astro.config.mjs"), fileExists(projectRoot, "astro.config.ts"):
		m.Framework = "Astro"
		m.DevServerURL = "http://localhost:4321"
	case fileExists(projectRoot, "svelte.config.js"):
		m.Framework = "SvelteKit"
		m.DevServerURL = "http://localhost:5173"
	}

	if m.Language == "JavaScript/TypeScript" {
		if port := readDevPortFromPackageJSON(filepath.Join(projectRoot, "package.json")); port > 0 {
			m.DevServerURL = "http://localhost:" + strconvItoa(port)
		}
	}

	for _, name := range []string{"README.md", "README"} {
		path := filepath.Join(projectRoot, name)
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		if intent := firstParagraph(string(data)); intent != "" {
			m.Intent = intent
			break
		}
	}

	return m
}

// WriteIdentity renders identity.md into <projectRoot>/.mneme/identity.md.
func WriteIdentity(projectRoot string) error {
	meta := DetectProjectMetadata(projectRoot)
	tmpl, err := template.New("identity").Parse(identityTemplate)
	if err != nil {
		return err
	}
	var sb strings.Builder
	if err := tmpl.Execute(&sb, meta); err != nil {
		return err
	}
	dir := filepath.Join(projectRoot, ".mneme")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return state.AtomicWrite(filepath.Join(dir, "identity.md"), []byte(sb.String()))
}

func fileExists(dir, name string) bool {
	_, err := os.Stat(filepath.Join(dir, name))
	return err == nil
}

func firstParagraph(s string) string {
	lines := strings.Split(s, "\n")
	var buf []string
	for _, ln := range lines {
		trim := strings.TrimSpace(ln)
		if strings.HasPrefix(trim, "#") {
			continue
		}
		if trim == "" {
			if len(buf) > 0 {
				break
			}
			continue
		}
		buf = append(buf, trim)
	}
	out := strings.Join(buf, " ")
	if len(out) > 240 {
		out = out[:240] + "…"
	}
	return out
}

func readDevPortFromPackageJSON(path string) int {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	var pkg struct {
		Scripts map[string]string `json:"scripts"`
	}
	if json.Unmarshal(data, &pkg) != nil {
		return 0
	}
	dev := pkg.Scripts["dev"]
	if dev == "" {
		dev = pkg.Scripts["start"]
	}
	for _, flag := range []string{"--port ", "-p "} {
		if i := strings.Index(dev, flag); i >= 0 {
			rest := dev[i+len(flag):]
			end := strings.IndexAny(rest, " \t")
			if end < 0 {
				end = len(rest)
			}
			n := 0
			for _, c := range rest[:end] {
				if c < '0' || c > '9' {
					break
				}
				n = n*10 + int(c-'0')
			}
			if n > 0 {
				return n
			}
		}
	}
	return 0
}

func strconvItoa(n int) string {
	if n == 0 {
		return "0"
	}
	const digits = "0123456789"
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = digits[n%10]
		n /= 10
	}
	return string(buf[i:])
}
