package designqc

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
)

type Framework struct {
	Name        string
	DefaultPort int
}

func (f *Framework) BaseURL() string {
	return fmt.Sprintf("http://localhost:%d", f.DefaultPort)
}

var viteConfigNames = []string{"vite.config.ts", "vite.config.js", "vite.config.mjs"}
var portRegex = regexp.MustCompile(`port:\s*(\d+)`)

func DetectFramework(projectRoot string) (*Framework, error) {
	for _, name := range viteConfigNames {
		path := filepath.Join(projectRoot, name)
		if data, err := os.ReadFile(path); err == nil {
			port := 5173
			if m := portRegex.FindSubmatch(data); m != nil {
				if n, err := strconv.Atoi(string(m[1])); err == nil && n > 0 {
					port = n
				}
			}
			return &Framework{Name: "vite", DefaultPort: port}, nil
		}
	}
	return nil, errors.New("no Vite config found; pass --route /path to skip auto-detection")
}
