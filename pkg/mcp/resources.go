package mcp

import (
	"context"
	"encoding/json"
)

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

	results, err := s.searcher.Search(ctx, searchReq.Query, searchReq.TopK, searchReq.CodebasePath)
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
