package scanner

import (
	"bufio"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Walk returns project-relative file paths using git ls-files.
// Filters out files larger than 1MB and binary files (null byte in first 512 bytes).
func Walk(projectRoot string) ([]string, error) {
	cmd := exec.Command("git", "ls-files", "--cached", "--others", "--exclude-standard")
	cmd.Dir = projectRoot
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var paths []string
	sc := bufio.NewScanner(strings.NewReader(string(out)))
	for sc.Scan() {
		rel := sc.Text()
		if rel == "" {
			continue
		}
		abs := filepath.Join(projectRoot, rel)
		info, err := os.Stat(abs)
		if err != nil {
			continue
		}
		if info.Size() > 1<<20 {
			continue
		}
		f, err := os.Open(abs)
		if err != nil {
			continue
		}
		head := make([]byte, 512)
		n, _ := f.Read(head)
		f.Close()
		binary := false
		for _, b := range head[:n] {
			if b == 0 {
				binary = true
				break
			}
		}
		if binary {
			continue
		}
		paths = append(paths, rel)
	}
	return paths, nil
}
