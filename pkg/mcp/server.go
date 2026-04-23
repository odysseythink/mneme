package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"github.com/ranwei/claude-context/pkg"
)

type Server struct {
	indexer  pkg.Indexer
	searcher pkg.Searcher
	mu       sync.Mutex
	nextID   int
}

func NewMCPServer(indexer pkg.Indexer, searcher pkg.Searcher) *Server {
	return &Server{
		indexer:  indexer,
		searcher: searcher,
	}
}

func (s *Server) Start() error {
	reader := bufio.NewReader(os.Stdin)

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("read error: %w", err)
		}

		var req MCPRequest
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			s.sendError(req.ID, -32700, "Parse error")
			continue
		}

		go s.handleRequest(req)
	}
}

func (s *Server) handleRequest(req MCPRequest) {
	var resp MCPResponse
	resp.JsonRpc = "2.0"
	resp.ID = req.ID

	ctx := context.Background()

	switch req.Method {
	case "resources/read":
		resp.Result = s.handleResourceRead(ctx, req.Params)
	case "resources/list":
		resp.Result = s.handleResourceList()
	default:
		resp.Error = &MCPError{Code: -32601, Message: "Method not found"}
	}

	s.sendResponse(resp)
}

func (s *Server) handleResourceRead(ctx context.Context, params json.RawMessage) interface{} {
	var req struct {
		URI string `json:"uri"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		return nil
	}

	if req.URI == "codebase://index" {
		return s.handleIndex(ctx, params)
	} else if len(req.URI) >= 16 && req.URI[:16] == "codebase://search" {
		return s.handleSearch(ctx, params)
	}

	return nil
}

func (s *Server) handleIndex(ctx context.Context, params json.RawMessage) interface{} {
	var idxReq IndexRequest
	if err := json.Unmarshal(params, &idxReq); err != nil {
		return nil
	}

	result, err := s.indexer.Index(ctx, idxReq.CodebasePath)
	if err != nil {
		return map[string]interface{}{"error": err.Error()}
	}

	return map[string]interface{}{
		"files_processed": result.FilesProcessed,
		"vectors_stored":  result.VectorsStored,
		"duration_ms":     result.Duration.Milliseconds(),
		"embedding_model": result.EmbeddingModel,
	}
}

func (s *Server) handleSearch(ctx context.Context, params json.RawMessage) interface{} {
	var searchReq SearchRequest
	if err := json.Unmarshal(params, &searchReq); err != nil {
		return nil
	}

	if searchReq.TopK <= 0 {
		searchReq.TopK = 5
	}

	results, err := s.searcher.Search(ctx, searchReq.Query, searchReq.TopK)
	if err != nil {
		return map[string]interface{}{"error": err.Error()}
	}

	return map[string]interface{}{
		"results": results,
		"count":   len(results),
	}
}

func (s *Server) handleResourceList() interface{} {
	return map[string]interface{}{
		"resources": []map[string]interface{}{
			{
				"uri":         "codebase://index",
				"name":        "Index Codebase",
				"description": "Index a codebase for semantic search",
				"mimeType":    "application/json",
			},
			{
				"uri":         "codebase://search",
				"name":        "Search Codebase",
				"description": "Search indexed codebase semantically",
				"mimeType":    "application/json",
			},
		},
	}
}

func (s *Server) sendResponse(resp MCPResponse) {
	data, _ := json.Marshal(resp)
	fmt.Println(string(data))
}

func (s *Server) sendError(id int, code int, message string) {
	resp := MCPResponse{
		JsonRpc: "2.0",
		ID:      id,
		Error:   &MCPError{Code: code, Message: message},
	}
	s.sendResponse(resp)
}
