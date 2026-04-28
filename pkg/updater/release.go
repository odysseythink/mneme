package updater

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const DefaultGitHubAPIBase = "https://api.github.com"

type ghAsset struct {
	Name        string `json:"name"`
	DownloadURL string `json:"browser_download_url"`
}

type ghRelease struct {
	TagName string    `json:"tag_name"`
	Assets  []ghAsset `json:"assets"`
}

func DownloadLatestBinary(apiBase, repo string) (string, string, error) {
	url := strings.TrimRight(apiBase, "/") + "/repos/" + repo + "/releases/latest"
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return "", "", fmt.Errorf("fetch release: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", "", fmt.Errorf("release API returned %s", resp.Status)
	}
	var rel ghRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return "", "", fmt.Errorf("parse release: %w", err)
	}

	wantName := fmt.Sprintf("mneme-%s-%s", runtime.GOOS, runtime.GOARCH)
	var binAsset, sumAsset *ghAsset
	for i := range rel.Assets {
		switch rel.Assets[i].Name {
		case wantName:
			binAsset = &rel.Assets[i]
		case wantName + ".sha256":
			sumAsset = &rel.Assets[i]
		}
	}
	if binAsset == nil {
		return "", "", fmt.Errorf("no asset named %s in release %s", wantName, rel.TagName)
	}
	if sumAsset == nil {
		return "", "", fmt.Errorf("no checksum asset %s.sha256 in release %s", wantName, rel.TagName)
	}

	sumBytes, err := fetchBytes(client, sumAsset.DownloadURL)
	if err != nil {
		return "", "", fmt.Errorf("fetch checksum: %w", err)
	}
	expectedSHA := strings.Fields(string(sumBytes))
	if len(expectedSHA) == 0 {
		return "", "", errors.New("empty checksum file")
	}

	tmp, err := os.CreateTemp("", "mneme-update-*")
	if err != nil {
		return "", "", err
	}
	tmpPath := tmp.Name()
	defer tmp.Close()

	binResp, err := client.Get(binAsset.DownloadURL)
	if err != nil {
		os.Remove(tmpPath)
		return "", "", fmt.Errorf("fetch binary: %w", err)
	}
	defer binResp.Body.Close()
	if binResp.StatusCode != 200 {
		os.Remove(tmpPath)
		return "", "", fmt.Errorf("binary download returned %s", binResp.Status)
	}

	hasher := sha256.New()
	if _, err := io.Copy(io.MultiWriter(tmp, hasher), binResp.Body); err != nil {
		os.Remove(tmpPath)
		return "", "", fmt.Errorf("write binary: %w", err)
	}
	gotSHA := hex.EncodeToString(hasher.Sum(nil))
	if gotSHA != expectedSHA[0] {
		os.Remove(tmpPath)
		return "", "", fmt.Errorf("SHA256 mismatch: got %s, want %s", gotSHA, expectedSHA[0])
	}
	return tmpPath, rel.TagName, nil
}

func fetchBytes(client *http.Client, url string) ([]byte, error) {
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("status %s", resp.Status)
	}
	return io.ReadAll(resp.Body)
}

func ReplaceBinary(newPath, target string) error {
	if err := os.Chmod(newPath, 0755); err != nil {
		return err
	}
	if err := os.Rename(newPath, target); err != nil {
		in, ferr := os.Open(newPath)
		if ferr != nil {
			return err
		}
		defer in.Close()
		out, ferr := os.OpenFile(target, os.O_TRUNC|os.O_WRONLY, 0755)
		if ferr != nil {
			out, ferr = os.Create(target)
			if ferr != nil {
				return err
			}
		}
		defer out.Close()
		if _, ferr := io.Copy(out, in); ferr != nil {
			return ferr
		}
		os.Chmod(target, 0755)
		os.Remove(newPath)
		return nil
	}
	return os.Chmod(target, 0755)
}

func TempDirCleanup() {
	tmp := os.TempDir()
	entries, _ := os.ReadDir(tmp)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "mneme-update-") {
			os.Remove(filepath.Join(tmp, e.Name()))
		}
	}
}
