package tests

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ranwei/mneme/pkg/designqc"
)

const fixtureHTML = `<!doctype html><html><head><title>fix</title></head>
<body style="margin:0">
<div style="width:1440px;height:600px;background:#222;color:#fff;font:48px sans-serif;display:flex;align-items:center;justify-content:center">
hello mneme
</div>
</body></html>`

func TestCapture_HappyPath(t *testing.T) {
	if _, err := designqc.DetectChrome(); err != nil {
		t.Skipf("Chrome not available: %v", err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(fixtureHTML))
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	br := designqc.NewBrowser(ctx)
	defer br.Close()

	jpeg, w, h, err := br.Capture(designqc.CaptureOptions{
		URL: srv.URL, Quality: 80, MaxWidth: 1440, Timeout: 25 * time.Second,
	})
	if err != nil {
		t.Fatalf("Capture: %v", err)
	}
	if !bytes.HasPrefix(jpeg, []byte{0xFF, 0xD8, 0xFF}) {
		t.Errorf("not a JPEG: first bytes %x", jpeg[:4])
	}
	if w == 0 || h == 0 {
		t.Errorf("dims: %dx%d", w, h)
	}
}

func TestWaitForDevServer_Down(t *testing.T) {
	err := designqc.WaitForDevServer("http://127.0.0.1:1", 200*time.Millisecond)
	if err == nil {
		t.Error("expected error for unreachable server")
	}
}

func TestWaitForDevServer_Up(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("ok")) }))
	defer srv.Close()
	if err := designqc.WaitForDevServer(srv.URL, 1*time.Second); err != nil {
		t.Errorf("WaitForDevServer: %v", err)
	}
}
