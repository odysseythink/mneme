package tests

import (
	"testing"

	"github.com/ranwei/claude-context/pkg/mcp"
	ctxpkg "github.com/ranwei/claude-context/pkg/context"
)

func TestMCPServerInit(t *testing.T) {
	mockStore := &MockStore{}

	indexer := ctxpkg.NewIndexer(mockStore, nil, nil)
	searcher := ctxpkg.NewSearcher(mockStore, nil)

	server := mcp.NewMCPServer(indexer, searcher)
	if server == nil {
		t.Fatal("Failed to create MCP server")
	}
}
