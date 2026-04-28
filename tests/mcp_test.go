package tests

import (
	"encoding/json"
	"testing"

	ctxpkg "github.com/ranwei/mneme/pkg/context"
	"github.com/ranwei/mneme/pkg/mcp"
)

func TestMCPServerInit(t *testing.T) {
	mockStore := &MockStore{}

	indexer := ctxpkg.NewIndexer(mockStore, nil, nil, "test-model")
	searcher := ctxpkg.NewSearcher(mockStore, nil)

	server := mcp.NewMCPServer(indexer, searcher, nil)
	if server == nil {
		t.Fatal("Failed to create MCP server")
	}
}

func TestInitializeCapabilities(t *testing.T) {
	mockStore := &MockStore{}
	indexer := ctxpkg.NewIndexer(mockStore, nil, nil, "test-model")
	searcher := ctxpkg.NewSearcher(mockStore, nil)
	server := mcp.NewMCPServer(indexer, searcher, nil)

	req := mcp.MCPRequest{
		JsonRpc: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  "initialize",
		Params:  json.RawMessage(`{}`),
	}

	resp := server.HandleRequest(req)

	if resp.Error != nil {
		t.Fatalf("Expected no error, got: %v", resp.Error)
	}

	// Check result structure
	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatal("Result should be a map")
	}

	// Check protocolVersion
	if result["protocolVersion"] != "2024-11-05" {
		t.Errorf("protocolVersion mismatch: %v", result["protocolVersion"])
	}

	// Check capabilities exist
	caps, ok := result["capabilities"].(map[string]interface{})
	if !ok {
		t.Fatal("capabilities should be a map")
	}

	// Check tools capability
	if _, ok := caps["tools"].(map[string]interface{}); !ok {
		t.Fatal("tools capability should exist")
	}

	// Check resources capability
	resources, ok := caps["resources"].(map[string]interface{})
	if !ok {
		t.Fatal("resources capability should exist and be a map")
	}

	// Check subscribe and listChanged in resources
	if _, ok := resources["subscribe"]; !ok {
		t.Fatal("resources should have 'subscribe' key")
	}
	if _, ok := resources["listChanged"]; !ok {
		t.Fatal("resources should have 'listChanged' key")
	}

	// Verify values are false
	if resources["subscribe"] != false {
		t.Errorf("subscribe should be false, got: %v", resources["subscribe"])
	}
	if resources["listChanged"] != false {
		t.Errorf("listChanged should be false, got: %v", resources["listChanged"])
	}
}

func TestPingHandler(t *testing.T) {
	mockStore := &MockStore{}
	indexer := ctxpkg.NewIndexer(mockStore, nil, nil, "test-model")
	searcher := ctxpkg.NewSearcher(mockStore, nil)
	server := mcp.NewMCPServer(indexer, searcher, nil)

	req := mcp.MCPRequest{
		JsonRpc: "2.0",
		ID:      json.RawMessage(`2`),
		Method:  "ping",
		Params:  json.RawMessage(`{}`),
	}

	resp := server.HandleRequest(req)

	// Should have no error
	if resp.Error != nil {
		t.Fatalf("Expected no error, got: %v", resp.Error)
	}

	// Should have empty result object
	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatal("Result should be a map")
	}

	// Should be empty
	if len(result) != 0 {
		t.Errorf("Result should be empty, got: %v", result)
	}
}
