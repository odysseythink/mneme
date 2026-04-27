package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	"github.com/odysseythink/mlog"
	"github.com/ranwei/claude-context/pkg"
	"github.com/ranwei/claude-context/pkg/embedding"
)

type Server struct {
	indexer   pkg.Indexer
	searcher  pkg.Searcher
	embedding *embedding.CachedClient
	mu        sync.Mutex
	nextID    int
}

func NewMCPServer(indexer pkg.Indexer, searcher pkg.Searcher, emb *embedding.CachedClient) *Server {
	return &Server{
		indexer:   indexer,
		searcher:  searcher,
		embedding: emb,
	}
}

func (s *Server) Start() error {
	reader := bufio.NewReader(os.Stdin)

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return fmt.Errorf("read error: %w", err)
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var req MCPRequest
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			mlog.Warningf("parse error on input: %v", err)
			s.sendError(nil, -32700, "Parse error")
			continue
		}

		// Notifications have no id and expect no response
		if req.Method == "notifications/initialized" {
			mlog.V(2).Infof("received notification: %s", req.Method)
			continue
		}

		mlog.V(2).Infof("request id=%d method=%s", req.ID, req.Method)
		go s.handleRequest(req)
	}
}

func (s *Server) HandleRequest(req MCPRequest) MCPResponse {
	var resp MCPResponse
	resp.JsonRpc = "2.0"
	resp.ID = req.ID // echo the raw ID back unchanged

	ctx := context.Background()

	switch req.Method {
	case "initialize":
		resp.Result = s.handleInitialize()
	case "ping":
		resp.Result = map[string]interface{}{}
	case "tools/list":
		resp.Result = s.handleToolList()
	case "tools/call":
		resp.Result = s.handleToolCall(ctx, req.Params)
	case "resources/read":
		resp.Result = s.handleResourceRead(ctx, req.Params)
	case "resources/list":
		resp.Result = s.handleResourceList()
	default:
		mlog.Warningf("unknown method: %s", req.Method)
		resp.Error = &MCPError{Code: -32601, Message: "Method not found"}
	}

	return resp
}

func (s *Server) handleRequest(req MCPRequest) {
	resp := s.HandleRequest(req)
	s.sendResponse(resp)
}

func (s *Server) handleInitialize() interface{} {
	return map[string]interface{}{
		"protocolVersion": "2024-11-05",
		"capabilities": map[string]interface{}{
			"tools": map[string]interface{}{},
			"resources": map[string]interface{}{
				"subscribe":   false,
				"listChanged": false,
			},
		},
		"serverInfo": map[string]interface{}{
			"name":    "claude-context",
			"version": "1.0.0",
		},
	}
}

func (s *Server) handleToolList() interface{} {
	return map[string]interface{}{
		"tools": []map[string]interface{}{
			{
				"name":        "index_codebase",
				"description": "Index a codebase directory for semantic search. Use the primary working directory from your system context as codebase_path unless the user specifies a different path. Subsequent calls on an already-indexed codebase are incremental — only changed files are re-embedded.",
				"inputSchema": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"codebase_path": map[string]interface{}{
							"type":        "string",
							"description": "Absolute path to the codebase directory to index. Use the primary working directory from system context if not otherwise specified.",
						},
					},
					"required": []string{"codebase_path"},
				},
			},
			{
				"name":        "search_codebase",
				"description": "Semantically search an indexed codebase for code relevant to a query. Always set codebase_path to the primary working directory from your system context so the search is scoped to the current project. If the result is empty, the codebase is likely not indexed yet — call index_codebase with the same path first, then retry the search.",
				"inputSchema": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"query": map[string]interface{}{
							"type":        "string",
							"description": "Natural language description of what you are looking for",
						},
						"codebase_path": map[string]interface{}{
							"type":        "string",
							"description": "Absolute path to the codebase to search. Use the primary working directory from system context to scope results to the current project. Omit only when explicitly searching across all indexed codebases.",
						},
						"top_k": map[string]interface{}{
							"type":        "integer",
							"description": "Number of results to return (default: 5)",
						},
					},
					"required": []string{"query"},
				},
			},
			{
				"name":        "ping_embedding",
				"description": "Test the embedding API with a single call. Returns the embedding dimension on success, or the exact API error on failure. Use this to diagnose embedding configuration issues.",
				"inputSchema": map[string]interface{}{
					"type":       "object",
					"properties": map[string]interface{}{},
				},
			},
			{
				"name":        "describe_codebase",
				"description": "Show the anatomy (file structure with descriptions) and recent session history for the current project. Pass cwd from your system context. No embedding API required.",
				"inputSchema": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"cwd": map[string]interface{}{
							"type":        "string",
							"description": "Working directory of the current project",
						},
					},
					"required": []string{"cwd"},
				},
			},
			{
				"name":        "get_project_rules",
				"description": "Return the cerebrum rules for the current project — coding conventions Claude should follow. Pass cwd from your system context. No embedding API required.",
				"inputSchema": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"cwd": map[string]interface{}{
							"type":        "string",
							"description": "Working directory of the current project",
						},
					},
					"required": []string{"cwd"},
				},
			},
			{
				"name":        "find_similar_bugs",
				"description": "Search the project buglog for previously fixed bugs similar to the given query (code snippet or description). Pass cwd from your system context. No embedding API required.",
				"inputSchema": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"cwd": map[string]interface{}{
							"type":        "string",
							"description": "Working directory of the current project",
						},
						"query": map[string]interface{}{
							"type":        "string",
							"description": "Code snippet or description to match against known bugs",
						},
					},
					"required": []string{"cwd", "query"},
				},
			},
		},
	}
}

func (s *Server) handleToolCall(ctx context.Context, params json.RawMessage) interface{} {
	var req ToolCallRequest
	if err := json.Unmarshal(params, &req); err != nil {
		return toolError("invalid params: " + err.Error())
	}

	switch req.Name {
	case "index_codebase":
		if s.indexer == nil {
			return toolError("embedding tools unavailable: set EMBEDDING_API_KEY to use index_codebase")
		}
		return s.callIndexCodbase(ctx, req.Arguments)
	case "search_codebase":
		if s.searcher == nil {
			return toolError("embedding tools unavailable: set EMBEDDING_API_KEY to use search_codebase")
		}
		return s.callSearchCodebase(ctx, req.Arguments)
	case "ping_embedding":
		if s.embedding == nil {
			return toolError("embedding tools unavailable: set EMBEDDING_API_KEY to use ping_embedding")
		}
		return s.callPingEmbedding(ctx)
	case "describe_codebase":
		return s.callDescribeCodebase(ctx, req.Arguments)
	case "get_project_rules":
		return s.callGetProjectRules(ctx, req.Arguments)
	case "find_similar_bugs":
		return s.callFindSimilarBugs(ctx, req.Arguments)
	default:
		return toolError("unknown tool: " + req.Name)
	}
}

func (s *Server) callPingEmbedding(ctx context.Context) interface{} {
	if s.embedding == nil {
		return toolError("embedding client not initialized")
	}
	emb, err := s.embedding.GenerateEmbedding(ctx, "hello world")
	if err != nil {
		return toolError(fmt.Sprintf("embedding API error: %v", err))
	}
	return ToolCallResult{Content: []ToolContent{{Type: "text", Text: fmt.Sprintf("OK: embedding API works, dim=%d", len(emb))}}}
}

func (s *Server) callIndexCodbase(ctx context.Context, args json.RawMessage) interface{} {
	var req IndexRequest
	if err := json.Unmarshal(args, &req); err != nil {
		return toolError("invalid arguments: " + err.Error())
	}
	if req.CodebasePath == "" {
		return toolError("codebase_path is required")
	}

	result, err := s.indexer.Index(ctx, req.CodebasePath)
	if err != nil {
		return toolError(err.Error())
	}

	if result.VectorsStored == 0 && result.FilesProcessed > result.FilesSkipped {
		msg := fmt.Sprintf(
			"ERROR: Indexed %d files but stored 0 vectors. Model: %s. First error: %s",
			result.FilesProcessed-result.FilesSkipped, result.EmbeddingModel, result.LastError,
		)
		return toolError(msg)
	}

	text := fmt.Sprintf(
		"Indexed %d files (%d skipped, unchanged), stored %d vectors in %dms (model: %s, failed: %d)",
		result.FilesProcessed-result.FilesSkipped, result.FilesSkipped, result.VectorsStored,
		result.Duration.Milliseconds(), result.EmbeddingModel, len(result.FailedFiles),
	)
	return ToolCallResult{Content: []ToolContent{{Type: "text", Text: text}}}
}

func (s *Server) callSearchCodebase(ctx context.Context, args json.RawMessage) interface{} {
	var req SearchRequest
	if err := json.Unmarshal(args, &req); err != nil {
		return toolError("invalid arguments: " + err.Error())
	}
	if req.Query == "" {
		return toolError("query is required")
	}
	if req.TopK <= 0 {
		req.TopK = 5
	}

	results, err := s.searcher.Search(ctx, req.Query, req.TopK, req.CodebasePath)
	if err != nil {
		return toolError(err.Error())
	}

	data, _ := json.MarshalIndent(results, "", "  ")
	return ToolCallResult{Content: []ToolContent{{Type: "text", Text: string(data)}}}
}

func toolError(msg string) ToolCallResult {
	return ToolCallResult{
		Content: []ToolContent{{Type: "text", Text: msg}},
		IsError: true,
	}
}

func (s *Server) sendResponse(resp MCPResponse) {
	data, _ := json.Marshal(resp)
	s.mu.Lock()
	defer s.mu.Unlock()
	fmt.Println(string(data))
}

func (s *Server) sendError(id json.RawMessage, code int, message string) {
	resp := MCPResponse{
		JsonRpc: "2.0",
		ID:      id,
		Error:   &MCPError{Code: code, Message: message},
	}
	s.sendResponse(resp)
}
