package tests

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestScanCheckCleanProject(t *testing.T) {
	bin := buildMnemeBinary(t)
	home := t.TempDir()
	proj := makeGitRepo(t)

	// Create .gitignore to exclude .mneme
	if err := os.WriteFile(filepath.Join(proj, ".gitignore"), []byte(".mneme/\n"), 0644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "add", ".gitignore")
	cmd.Dir = proj
	cmd.CombinedOutput()

	if err := os.WriteFile(filepath.Join(proj, "main.go"), []byte("package main\n"), 0644); err != nil {
		t.Fatal(err)
	}
	// Stage the file so git ls-files will see it
	cmd = exec.Command("git", "add", "main.go")
	cmd.Dir = proj
	cmd.CombinedOutput()

	mneme := filepath.Join(proj, ".mneme")
	os.MkdirAll(mneme, 0755)
	os.WriteFile(filepath.Join(mneme, ".local-id"), []byte("id\n"), 0644)

	scan := exec.Command(bin, "scan")
	scan.Dir = proj
	scan.Env = append(os.Environ(), "HOME="+home)
	if out, err := scan.CombinedOutput(); err != nil {
		t.Fatalf("initial scan: %v\n%s", err, out)
	}

	check := exec.Command(bin, "scan", "--check")
	check.Dir = proj
	check.Env = append(os.Environ(), "HOME="+home)
	out, err := check.CombinedOutput()
	if err != nil {
		t.Fatalf("scan --check on clean project: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "no drift") && !strings.Contains(string(out), "in sync") {
		t.Errorf("expected drift-free message, got: %s", out)
	}
}

func TestScanCheckDriftExits1(t *testing.T) {
	bin := buildMnemeBinary(t)
	home := t.TempDir()
	proj := makeGitRepo(t)

	// Create .gitignore to exclude .mneme
	if err := os.WriteFile(filepath.Join(proj, ".gitignore"), []byte(".mneme/\n"), 0644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "add", ".gitignore")
	cmd.Dir = proj
	cmd.CombinedOutput()

	if err := os.WriteFile(filepath.Join(proj, "main.go"), []byte("package main\n"), 0644); err != nil {
		t.Fatal(err)
	}
	// Stage the file so git ls-files will see it
	cmd = exec.Command("git", "add", "main.go")
	cmd.Dir = proj
	cmd.CombinedOutput()

	mneme := filepath.Join(proj, ".mneme")
	os.MkdirAll(mneme, 0755)
	os.WriteFile(filepath.Join(mneme, ".local-id"), []byte("id\n"), 0644)

	scan := exec.Command(bin, "scan")
	scan.Dir = proj
	scan.Env = append(os.Environ(), "HOME="+home)
	if out, err := scan.CombinedOutput(); err != nil {
		t.Fatalf("initial scan: %v\n%s", err, out)
	}

	if err := os.WriteFile(filepath.Join(proj, "added.go"), []byte("package main\n"), 0644); err != nil {
		t.Fatal(err)
	}
	// Stage the new file so git ls-files will see it
	cmd = exec.Command("git", "add", "added.go")
	cmd.Dir = proj
	cmd.CombinedOutput()

	check := exec.Command(bin, "scan", "--check")
	check.Dir = proj
	check.Env = append(os.Environ(), "HOME="+home)
	out, err := check.CombinedOutput()
	if err == nil {
		t.Errorf("scan --check with drift should exit nonzero; output=%s", out)
	}
	if !strings.Contains(string(out), "drift") {
		t.Errorf("expected 'drift' in output, got: %s", out)
	}
}
