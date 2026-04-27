package tests

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/ranwei/claude-context/pkg/mcp"
)

func newTestServer() *mcp.Server {
	return mcp.NewMCPServer(nil, nil, nil)
}

func toolsCall(t *testing.T, s *mcp.Server, name string, argsMap map[string]interface{}) mcp.ToolCallResult {
	t.Helper()
	args, _ := json.Marshal(argsMap)
	params, _ := json.Marshal(map[string]interface{}{
		"name":      name,
		"arguments": json.RawMessage(args),
	})
	req := mcp.MCPRequest{
		Method: "tools/call",
		Params: params,
	}
	resp := s.HandleRequest(req)
	data, _ := json.Marshal(resp.Result)
	var result mcp.ToolCallResult
	json.Unmarshal(data, &result)
	return result
}

func TestServerEmbeddingToolsWithoutKey(t *testing.T) {
	s := newTestServer()

	for _, toolName := range []string{"index_codebase", "search_codebase", "ping_embedding"} {
		result := toolsCall(t, s, toolName, map[string]interface{}{"codebase_path": "/tmp"})
		if !result.IsError {
			t.Errorf("%s: expected IsError=true when embedding nil, got false", toolName)
		}
		if len(result.Content) == 0 || !strings.Contains(result.Content[0].Text, "EMBEDDING_API_KEY") {
			t.Errorf("%s: expected EMBEDDING_API_KEY mention in error, got: %v", toolName, result.Content)
		}
	}
}
