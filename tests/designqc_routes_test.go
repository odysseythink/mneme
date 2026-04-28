package tests

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ranwei/mneme/pkg/designqc"
)

func TestEnumerateRoutes_FromRouter(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "src")
	if err := os.MkdirAll(src, 0o700); err != nil {
		t.Fatal(err)
	}
	content := `
import { createBrowserRouter } from 'react-router-dom'
const router = createBrowserRouter([
  { path: "/", element: null },
  { path: "/about", element: null },
  { path: "/users/:id", element: null },
])
`
	if err := os.WriteFile(filepath.Join(src, "router.ts"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := designqc.EnumerateRoutes(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("routes: got %d, want 3 (%v)", len(got), got)
	}
	want := []string{"/", "/about", "/users/:id"}
	for i, r := range got {
		if r.Path != want[i] {
			t.Errorf("[%d] path = %q, want %q", i, r.Path, want[i])
		}
	}
}

func TestEnumerateRoutes_Override(t *testing.T) {
	got, err := designqc.EnumerateRoutes(t.TempDir(), []string{"/x", "/y"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Path != "/x" || got[1].Path != "/y" {
		t.Errorf("override: %+v", got)
	}
}

func TestEnumerateRoutes_FallbackRoot(t *testing.T) {
	got, err := designqc.EnumerateRoutes(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Path != "/" || got[0].Slug != "root" {
		t.Errorf("fallback: %+v", got)
	}
}

func TestEnumerateRoutes_SlugDerivation(t *testing.T) {
	cases := map[string]string{
		"/":            "root",
		"/about":       "about",
		"/users/:id":   "users-id",
		"/foo/bar":     "foo-bar",
		"/with spaces": "with-spaces",
	}
	for path, wantSlug := range cases {
		got := designqc.SlugForPath(path)
		if got != wantSlug {
			t.Errorf("slug(%q) = %q, want %q", path, got, wantSlug)
		}
	}
}
