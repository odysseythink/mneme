package designqc

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"time"

	"github.com/chromedp/chromedp"
)

type CaptureOptions struct {
	URL      string
	Quality  int
	MaxWidth int
	Timeout  time.Duration
}

// Browser owns a long-lived chromedp context shared across routes in one run.
type Browser struct {
	allocCancel context.CancelFunc
	ctxCancel   context.CancelFunc
	browserCtx  context.Context
}

func NewBrowser(parent context.Context) *Browser {
	allocCtx, allocCancel := chromedp.NewExecAllocator(parent,
		append(chromedp.DefaultExecAllocatorOptions[:],
			chromedp.Flag("headless", true),
			chromedp.Flag("disable-gpu", true),
			chromedp.Flag("no-sandbox", true),
		)...,
	)
	browserCtx, ctxCancel := chromedp.NewContext(allocCtx)
	return &Browser{
		allocCancel: allocCancel,
		ctxCancel:   ctxCancel,
		browserCtx:  browserCtx,
	}
}

func (b *Browser) Close() {
	if b.ctxCancel != nil {
		b.ctxCancel()
	}
	if b.allocCancel != nil {
		b.allocCancel()
	}
}

func (b *Browser) Capture(opts CaptureOptions) ([]byte, int, int, error) {
	if opts.Quality <= 0 || opts.Quality > 100 {
		opts.Quality = 80
	}
	if opts.MaxWidth <= 0 {
		opts.MaxWidth = 1440
	}
	if opts.Timeout <= 0 {
		opts.Timeout = 30 * time.Second
	}

	ctx, cancel := context.WithTimeout(b.browserCtx, opts.Timeout)
	defer cancel()

	var buf []byte
	var dims [2]int
	err := chromedp.Run(ctx,
		chromedp.EmulateViewport(int64(opts.MaxWidth), 900),
		chromedp.Navigate(opts.URL),
		chromedp.WaitReady("body", chromedp.ByQuery),
		chromedp.Sleep(500*time.Millisecond),
		chromedp.FullScreenshot(&buf, opts.Quality),
		chromedp.Evaluate(
			`[document.documentElement.scrollWidth, document.documentElement.scrollHeight]`,
			&dims,
		),
	)
	if err != nil {
		return nil, 0, 0, err
	}
	return buf, dims[0], dims[1], nil
}

// DetectChrome returns the path to a usable Chrome/Chromium binary, or
// an error with actionable install instructions.
func DetectChrome() (string, error) {
	if p := os.Getenv("CHROME_PATH"); p != "" {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	candidates := []string{}
	switch runtime.GOOS {
	case "darwin":
		candidates = append(candidates,
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			"/Applications/Chromium.app/Contents/MacOS/Chromium",
			"/Applications/Google Chrome Canary.app/Contents/MacOS/Google Chrome Canary",
		)
	case "linux":
		candidates = append(candidates, "google-chrome", "chromium", "chromium-browser", "google-chrome-stable")
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c, nil
		}
		if p, err := exec.LookPath(c); err == nil {
			return p, nil
		}
	}
	return "", errors.New(
		"Chrome/Chromium not found. Install via:\n" +
			"  macOS:  brew install --cask google-chrome\n" +
			"  Linux:  apt-get install chromium-browser\n" +
			"Or set CHROME_PATH=/path/to/chrome",
	)
}

// WaitForDevServer GETs baseURL with the given timeout. Any HTTP response
// (2xx/3xx/4xx/5xx) counts as ready. Connection refused / timeout returns error.
func WaitForDevServer(baseURL string, timeout time.Duration) error {
	client := &http.Client{Timeout: timeout}
	resp, err := client.Get(baseURL + "/")
	if err != nil {
		return fmt.Errorf("dev server not reachable at %s; run \"pnpm dev\" in another terminal first", baseURL)
	}
	resp.Body.Close()
	return nil
}
