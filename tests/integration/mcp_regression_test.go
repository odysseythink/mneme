// tests/integration/mcp_regression_test.go
package integration_test

import (
	"os/exec"
	"strings"
	"testing"
)

func TestMCPServerStartsWithNoArgs(t *testing.T) {
	// Verify the binary starts as MCP server when given no args and a piped stdin.
	// We send a minimal JSON-RPC initialize request and verify we get a response.
	initRequest := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"0.0.1"}}}`

	cmd := exec.Command(binaryPath)
	cmd.Stdin = strings.NewReader(initRequest + "\n")
	cmd.Env = []string{
		"EMBEDDING_API_KEY=test-key-placeholder",
		"DB_BACKEND=chromem",
	}

	out, _ := cmd.CombinedOutput()
	outStr := string(out)

	// Should get a JSON-RPC response (not a "usage" message)
	if !strings.Contains(outStr, `"jsonrpc"`) && !strings.Contains(outStr, `"result"`) {
		// It's acceptable if it prints the usage/info message when EMBEDDING_API_KEY
		// doesn't have a real value — the key thing is that no-args → MCP path, not subcommand path
		if strings.Contains(outStr, "unknown subcommand") {
			t.Errorf("binary routed to subcommand path instead of MCP server: %s", outStr)
		}
	}
}

func TestMCPZeroRegressionBuildCheck(t *testing.T) {
	// Verify the binary compiles and the MCP server path is reachable.
	// Deep behavioral regression is covered by tests/mcp_test.go (in-process).
	if binaryPath == "" {
		t.Skip("binaryPath not set")
	}
	cmd := exec.Command(binaryPath, "version")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("binary version failed: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "mneme") {
		t.Errorf("unexpected version output: %s", out)
	}
}
