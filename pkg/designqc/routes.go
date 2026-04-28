package designqc

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type Route struct {
	Path string
	Slug string
}

var (
	routerFileRegex = regexp.MustCompile(`(?i)router.*\.(ts|tsx|js|jsx)$`)
	pathLiteral     = regexp.MustCompile(`path:\s*['"]([^'"]+)['"]`)
	nonAlnum        = regexp.MustCompile(`[^a-z0-9-]+`)
)

func EnumerateRoutes(projectRoot string, override []string) ([]Route, error) {
	if len(override) > 0 {
		out := make([]Route, 0, len(override))
		for _, p := range override {
			out = append(out, Route{Path: p, Slug: SlugForPath(p)})
		}
		return out, nil
	}

	srcDir := filepath.Join(projectRoot, "src")
	seen := map[string]struct{}{}
	_ = filepath.WalkDir(srcDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if !routerFileRegex.MatchString(d.Name()) {
			return nil
		}
		data, rerr := os.ReadFile(path)
		if rerr != nil {
			return nil
		}
		for _, m := range pathLiteral.FindAllSubmatch(data, -1) {
			seen[string(m[1])] = struct{}{}
		}
		return nil
	})

	if len(seen) == 0 {
		return []Route{{Path: "/", Slug: "root"}}, nil
	}
	paths := make([]string, 0, len(seen))
	for p := range seen {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	out := make([]Route, 0, len(paths))
	for _, p := range paths {
		out = append(out, Route{Path: p, Slug: SlugForPath(p)})
	}
	return out, nil
}

func SlugForPath(p string) string {
	if p == "" || p == "/" {
		return "root"
	}
	s := strings.TrimPrefix(p, "/")
	s = strings.ReplaceAll(s, "/", "-")
	s = strings.ToLower(s)
	s = nonAlnum.ReplaceAllString(s, "-")
	// collapse consecutive dashes to single dash
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	s = strings.Trim(s, "-")
	if s == "" {
		return "root"
	}
	return s
}
