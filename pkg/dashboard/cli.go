package dashboard

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"
)

type CLIDeps struct {
	UnixSocket    string
	TCPHost       string
	TCPPort       int
	TokenPath     string
	Opener        func(url string) error
	Stdout        io.Writer
	Stderr        io.Writer
	SkipUnixCheck bool
}

func RunCLI(deps CLIDeps, args []string) int {
	noOpen := false
	for _, a := range args {
		if a == "--no-open" {
			noOpen = true
		}
	}

	if !deps.SkipUnixCheck {
		if !pingUnix(deps.UnixSocket) {
			fmt.Fprintln(deps.Stderr, "✗ Daemon not running. Run: mneme daemon start")
			return 1
		}
	}

	if !pingTCP(deps.TCPHost, deps.TCPPort) {
		fmt.Fprintf(deps.Stderr,
			"✗ Dashboard TCP listener disabled. Add daemon_dashboard_port_enabled: true "+
				"to ~/.mneme/config.yaml, then: mneme daemon restart\n")
		return 1
	}

	tokenBytes, err := os.ReadFile(deps.TokenPath)
	if err != nil {
		fmt.Fprintf(deps.Stderr, "✗ Token file missing at %s — daemon may have just started.\n",
			deps.TokenPath)
		return 1
	}
	token := strings.TrimSpace(string(tokenBytes))
	if token == "" {
		fmt.Fprintln(deps.Stderr, "✗ Token file is empty")
		return 1
	}

	url := fmt.Sprintf("http://%s:%d/?token=%s", deps.TCPHost, deps.TCPPort, token)

	if noOpen {
		fmt.Fprintln(deps.Stdout, url)
		return 0
	}

	opener := deps.Opener
	if opener == nil {
		opener = systemOpener()
	}
	if err := opener(url); err != nil {
		fmt.Fprintln(deps.Stderr, "(could not open browser:", err, ")")
		fmt.Fprintln(deps.Stdout, url)
	}
	return 0
}

func pingUnix(socket string) bool {
	if socket == "" {
		return false
	}
	if _, err := os.Stat(socket); err != nil {
		return false
	}
	tr := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			d := net.Dialer{Timeout: 200 * time.Millisecond}
			return d.DialContext(ctx, "unix", socket)
		},
	}
	client := &http.Client{Transport: tr, Timeout: 200 * time.Millisecond}
	resp, err := client.Get("http://unix/health")
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func pingTCP(host string, port int) bool {
	d := net.Dialer{Timeout: 200 * time.Millisecond}
	conn, err := d.Dial("tcp", fmt.Sprintf("%s:%d", host, port))
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

func systemOpener() func(url string) error {
	return func(url string) error {
		var bin string
		var args []string
		switch runtime.GOOS {
		case "darwin":
			bin = "open"
			args = []string{url}
		case "linux":
			bin = "xdg-open"
			args = []string{url}
		case "windows":
			bin = "cmd"
			args = []string{"/c", "start", url}
		default:
			return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
		}
		return execCommand(bin, args...)
	}
}
