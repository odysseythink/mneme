package tests

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/ranwei/mneme/pkg/updater"
)

func TestFetchReleaseAssetMatchesPlatform(t *testing.T) {
	expected := []byte("this-is-the-binary")
	sum := sha256.Sum256(expected)
	expectedSHA := hex.EncodeToString(sum[:])

	mux := http.NewServeMux()
	mux.HandleFunc("/repos/owner/mneme/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		assetName := "mneme-" + runtime.GOOS + "-" + runtime.GOARCH
		json.NewEncoder(w).Encode(map[string]any{
			"tag_name": "v0.2.0",
			"assets": []map[string]string{
				{"name": assetName, "browser_download_url": "http://" + r.Host + "/assets/bin"},
				{"name": assetName + ".sha256", "browser_download_url": "http://" + r.Host + "/assets/sum"},
			},
		})
	})
	mux.HandleFunc("/assets/bin", func(w http.ResponseWriter, r *http.Request) {
		w.Write(expected)
	})
	mux.HandleFunc("/assets/sum", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(expectedSHA + "  filename\n"))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	got, version, err := updater.DownloadLatestBinary(srv.URL, "owner/mneme")
	if err != nil {
		t.Fatalf("DownloadLatestBinary: %v", err)
	}
	defer os.Remove(got)
	if version != "v0.2.0" {
		t.Errorf("version = %q, want v0.2.0", version)
	}
	data, _ := os.ReadFile(got)
	if string(data) != string(expected) {
		t.Errorf("downloaded contents differ")
	}
}

func TestDownloadRejectsBadSHA(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/owner/mneme/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		assetName := "mneme-" + runtime.GOOS + "-" + runtime.GOARCH
		json.NewEncoder(w).Encode(map[string]any{
			"tag_name": "v0.2.0",
			"assets": []map[string]string{
				{"name": assetName, "browser_download_url": "http://" + r.Host + "/bad/bin"},
				{"name": assetName + ".sha256", "browser_download_url": "http://" + r.Host + "/bad/sum"},
			},
		})
	})
	mux.HandleFunc("/bad/bin", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("real")) })
	mux.HandleFunc("/bad/sum", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(strings.Repeat("0", 64) + "  bin"))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	_, _, err := updater.DownloadLatestBinary(srv.URL, "owner/mneme")
	if err == nil {
		t.Errorf("expected SHA mismatch error")
	}
}

func TestReplaceBinaryAtomic(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "mneme")
	os.WriteFile(target, []byte("old"), 0755)
	newFile := filepath.Join(dir, "new-bin")
	os.WriteFile(newFile, []byte("new"), 0755)

	if err := updater.ReplaceBinary(newFile, target); err != nil {
		t.Fatalf("ReplaceBinary: %v", err)
	}
	got, _ := os.ReadFile(target)
	if string(got) != "new" {
		t.Errorf("target contents = %q, want %q", string(got), "new")
	}
	info, _ := os.Stat(target)
	if info.Mode()&0111 == 0 {
		t.Errorf("target not executable: %v", info.Mode())
	}
}
